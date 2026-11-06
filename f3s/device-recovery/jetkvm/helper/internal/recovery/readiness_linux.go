//go:build linux

package recovery

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const readinessLimit = 65536
const readinessProcessLimit = 1024

// readinessReader is deliberately read-only. Its paths and commands are fixed by
// collectParentReadiness, rather than supplied by profiles or lease contents.
type readinessReader interface {
	read(context.Context, string, int, bool) ([]byte, ReadinessFile, error)
	names(context.Context, string, int) ([]string, error)
	resolve(string) (string, error)
	output(context.Context, ...string) ([]byte, error)
}

type nativeReadinessReader struct{}

// CollectParentReadiness takes one bounded read-only observation. Callers must
// collect and validate a second observation; this does not wait for readiness,
// claim a boot allowance, stop the vendor owner, or mutate network state.
// The three-second context bounds commands and checks between local reads. As
// with other proc/sysfs readers, it cannot cancel uninterruptible kernel I/O.
func CollectParentReadiness(ctx context.Context) (ParentReadinessSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return collectParentReadiness(ctx, nativeReadinessReader{})
}

func collectParentReadiness(ctx context.Context, r readinessReader) (ParentReadinessSnapshot, error) {
	var s ParentReadinessSnapshot
	read := func(path string, limit int) ([]byte, ReadinessFile, error) { return r.read(ctx, path, limit, false) }
	boot, _, err := read("/proc/sys/kernel/random/boot_id", 64)
	if err != nil {
		return s, err
	}
	s.Boot = strings.TrimSpace(string(boot))
	if s.Boot == "" {
		return s, errors.New("missing current boot identity")
	}
	_, s.Busybox, err = read("/bin/busybox", 4<<20)
	if err != nil {
		return s, err
	}
	_, s.Handler, err = read(readinessHandler, 16384)
	if err != nil {
		return s, err
	}
	hooks, err := r.names(ctx, readinessHandler+".d", 128)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return s, err
	}
	for _, name := range hooks {
		_, f, e := read(readinessHandler+".d/"+name, readinessLimit)
		if e != nil {
			return s, errors.New("unknown DHCP hook")
		}
		if f.Mode&0111 != 0 {
			s.ExecutableHooks++
		}
	}
	s.HooksKnown = true
	s.Begin, err = readinessProcesses(ctx, r)
	if err != nil {
		return s, err
	}
	mac, _, err := read("/sys/class/net/eth0/address", 64)
	if err != nil {
		return s, err
	}
	index, _, err := read("/sys/class/net/eth0/ifindex", 32)
	if err != nil {
		return s, err
	}
	flags, _, err := read("/sys/class/net/eth0/flags", 32)
	if err != nil {
		return s, err
	}
	s.Parent.Index, err = strconv.Atoi(strings.TrimSpace(string(index)))
	if err != nil {
		return s, errors.New("invalid parent index")
	}
	flag, err := strconv.ParseUint(strings.TrimSpace(string(flags)), 0, 32)
	if err != nil {
		return s, errors.New("invalid parent flags")
	}
	s.Parent.Exists, s.Parent.Up, s.Parent.MAC = true, flag&1 != 0, strings.TrimSpace(string(mac))
	addresses, err := r.output(ctx, "ip", "-4", "address", "show", "dev", "eth0")
	if err != nil {
		return s, err
	}
	for _, line := range strings.Split(string(addresses), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == "inet" {
			if len(fields) < 2 {
				return s, errors.New("invalid parent address output")
			}
			s.Parent.Addresses = append(s.Parent.Addresses, fields[1])
		}
	}
	s.CanonicalLease, err = r.resolve(readinessLeasePath)
	if err != nil {
		return s, err
	}
	lease, file, err := read(readinessLeasePath, 8192)
	if err != nil {
		return s, err
	}
	s.Lease, err = parseReadinessLease(lease, file)
	if err != nil {
		return s, err
	}
	mounts, _, err := read("/proc/self/mountinfo", readinessLimit)
	if err != nil {
		return s, err
	}
	s.Mount, err = readinessMount(mounts, s.CanonicalLease)
	if err != nil {
		return s, err
	}
	routes, err := r.output(ctx, "ip", "-4", "route", "show", "table", "254")
	if err != nil {
		return s, err
	}
	s.Defaults, err = readinessDefaults(routes)
	if err != nil {
		return s, err
	}
	resolverPath, err := r.resolve("/etc/resolv.conf")
	if err != nil {
		return s, err
	}
	resolver, _, err := read(resolverPath, 8192)
	if err != nil {
		return s, err
	}
	for _, line := range strings.Split(string(resolver), "\n") {
		line, _, _ = strings.Cut(line, "#")
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == "nameserver" {
			if len(fields) != 2 {
				return s, errors.New("invalid resolver entry")
			}
			addresses, e := parseReadyAddresses(fields[1])
			if e != nil {
				return s, e
			}
			s.DNS = append(s.DNS, addresses...)
		}
	}
	s.End, err = readinessProcesses(ctx, r)
	if err != nil {
		return s, err
	}
	after, _, err := read("/proc/sys/kernel/random/boot_id", 64)
	if err != nil {
		return s, err
	}
	if strings.TrimSpace(string(after)) != s.Boot {
		return s, errors.New("boot changed during observation")
	}
	// A callback may rewrite the file while collecting. Do not publish a stale
	// parsed lease with a newer owner's completion observation.
	_, leaseAfter, err := read(readinessLeasePath, 8192)
	if err != nil {
		return s, err
	}
	if leaseAfter != s.Lease.File {
		return s, errors.New("lease changed during observation")
	}
	return s, nil
}

func readinessProcesses(ctx context.Context, r readinessReader) (ReadinessProcesses, error) {
	var result ReadinessProcesses
	names, err := r.names(ctx, "/proc", readinessProcessLimit)
	if err != nil {
		return result, err
	}
	var processes []ReadinessOwner
	for _, name := range names {
		pid, e := strconv.Atoi(name)
		if e != nil || pid <= 0 {
			continue
		}
		data, _, e := r.read(ctx, "/proc/"+name+"/stat", 4096, false)

		if e != nil {
			return result, errors.New("unknown process read")
		}
		owner, comm, e := readinessStat(data, pid)
		if e != nil {
			return result, e
		}
		processes = append(processes, owner)
		if comm != "udhcpc" {
			continue
		}
		argv, _, e := r.read(ctx, "/proc/"+name+"/cmdline", 4096, false)
		if e != nil {
			return result, errors.New("unknown DHCP command line")
		}
		if len(argv) == 0 || argv[len(argv)-1] != 0 {
			return result, errors.New("truncated DHCP command line")
		}
		owner.Argv = strings.Split(string(argv[:len(argv)-1]), "\x00")
		// A clearly explicit other-interface owner is irrelevant. Missing, malformed,
		// conflicting or implicit-default selections remain fail-closed candidates.
		if readinessOtherInterface(owner.Argv) {
			continue
		}
		result.Candidates++
		_, owner.Executable, e = r.read(ctx, "/proc/"+name+"/exe", 4<<20, true)
		if e != nil {
			return result, errors.New("unknown DHCP executable")
		}
		result.Owner = owner
	}
	for _, process := range processes {
		if process.PPID == result.Owner.PID && result.Owner.PID != 0 {
			result.Children++
		}
	}
	return result, nil
}

func readinessOtherInterface(argv []string) bool {
	normalized := append([]string(nil), argv...)
	var selected []string
	for i := 1; i < len(argv); i++ {
		if argv[i] == "-i" {
			i++
			if i >= len(argv) {
				return false
			}
			selected = append(selected, argv[i])
			normalized[i] = "eth0"
		} else if strings.HasPrefix(argv[i], "-i") {
			selected = append(selected, argv[i][2:])
			normalized[i] = "-ieth0"
		}
	}
	// Ignore only the same reviewed handler/option grammar on an explicit other
	// interface. Unknown options might override interface selection or callbacks.
	return len(selected) == 1 && selected[0] != "" && selected[0] != "eth0" && readinessArgv(normalized) == nil
}

func readinessStat(data []byte, pid int) (ReadinessOwner, string, error) {
	o := ReadinessOwner{PID: pid}
	text := strings.TrimSpace(string(data))
	left, right := strings.Index(text, "("), strings.LastIndex(text, ") ")
	if left < 1 || right <= left {
		return o, "", errors.New("invalid process stat")
	}
	actual, err := strconv.Atoi(strings.TrimSpace(text[:left]))
	fields := strings.Fields(text[right+2:])
	if err != nil || actual != pid || len(fields) < 20 || len(fields[0]) != 1 {
		return o, "", errors.New("invalid process stat fields")
	}
	o.State = fields[0][0]
	for i, target := range []*int{&o.PPID, &o.Group, &o.Session} {
		*target, err = strconv.Atoi(fields[i+1])
		if err != nil {
			return o, "", errors.New("invalid process identity")
		}
	}
	o.Birth, err = strconv.ParseUint(fields[19], 10, 64)
	if err != nil || o.Birth == 0 {
		return o, "", errors.New("invalid process birth")
	}
	return o, text[left+1 : right], nil
}

func readinessMount(data []byte, lease string) (ReadinessMount, error) {
	var result ReadinessMount
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		halves := strings.Split(line, " - ")
		if len(halves) != 2 {
			return result, errors.New("invalid mountinfo")
		}
		fields, filesystem := strings.Fields(halves[0]), strings.Fields(halves[1])
		if len(fields) < 6 || len(filesystem) < 3 {
			return result, errors.New("incomplete mountinfo")
		}
		point := fields[4]
		if strings.Contains(point, "\\") {
			return result, errors.New("escaped mountpoint unsupported")
		}
		if !mountContains(point, lease) || len(point) <= len(result.Path) {
			continue
		}
		id, err := strconv.Atoi(fields[0])
		if err != nil || id <= 0 {
			return result, errors.New("invalid mount identity")
		}
		pair := strings.Split(fields[2], ":")
		if len(pair) != 2 {
			return result, errors.New("invalid mount device")
		}
		major, e1 := strconv.ParseUint(pair[0], 10, 32)
		minor, e2 := strconv.ParseUint(pair[1], 10, 32)
		if e1 != nil || e2 != nil {
			return result, errors.New("invalid mount device number")
		}
		device := (minor & 0xff) | ((major & 0xfff) << 8) | ((minor &^ 0xff) << 12) | ((major &^ 0xfff) << 32)
		result = ReadinessMount{ID: id, Path: point, Type: filesystem[0], Device: device}
	}
	if result.ID == 0 {
		return result, errors.New("lease covering mount unknown")
	}
	return result, nil
}

func readinessDefaults(data []byte) ([]ReadinessRoute, error) {
	var result []ReadinessRoute
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "default" {
			continue
		}
		if len(fields) < 5 || fields[1] != "via" || fields[3] != "dev" {
			return nil, errors.New("unsupported default route")
		}
		addresses, err := parseReadyAddresses(fields[2])
		if err != nil {
			return nil, err
		}
		result = append(result, ReadinessRoute{Gateway: addresses[0], Interface: fields[4]})
	}
	return result, nil
}

func (nativeReadinessReader) read(ctx context.Context, path string, limit int, follow bool) ([]byte, ReadinessFile, error) {
	var identity ReadinessFile
	if err := ctx.Err(); err != nil {
		return nil, identity, err
	}
	flags := syscall.O_RDONLY | syscall.O_CLOEXEC | syscall.O_NONBLOCK
	if !follow {
		flags |= syscall.O_NOFOLLOW
	}
	fd, err := syscall.Open(path, flags, 0)
	if err != nil {
		return nil, identity, err
	}
	file := os.NewFile(uintptr(fd), path)
	defer func() { _ = file.Close() }()
	stat, err := file.Stat()
	if err != nil {
		return nil, identity, err
	}
	raw, ok := stat.Sys().(*syscall.Stat_t)
	if !ok || !stat.Mode().IsRegular() {
		return nil, identity, errors.New("nonregular readiness file")
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil {
		return nil, identity, err
	}
	if len(data) > limit {
		return nil, identity, errors.New("readiness file exceeds bound")
	}
	if err := ctx.Err(); err != nil {
		return nil, identity, err
	}
	hash := sha256.Sum256(data)
	identity = ReadinessFile{Hash: hex.EncodeToString(hash[:]), Mode: uint32(raw.Mode & 07777), UID: raw.Uid, GID: raw.Gid, Links: uint64(raw.Nlink), Inode: raw.Ino, Device: uint64(raw.Dev), Regular: true}
	return data, identity, nil
}

func (nativeReadinessReader) names(ctx context.Context, path string, limit int) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Open without following a directory symlink; a substituted hook directory
	// must not be treated as an empty trusted directory.
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	defer func() { _ = file.Close() }()
	names, err := file.Readdirnames(limit + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(names) > limit {
		return nil, errors.New("readiness directory exceeds bound")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return names, nil
}

func (nativeReadinessReader) resolve(path string) (string, error) { return filepath.EvalSymlinks(path) }

// boundedReadinessOutput refuses oversized output rather than retaining it.
type boundedReadinessOutput struct{ bytes.Buffer }

func (b *boundedReadinessOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > readinessLimit {
		return 0, errors.New("readiness output exceeds bound")
	}
	return b.Buffer.Write(p)
}
func (nativeReadinessReader) output(ctx context.Context, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "/bin/busybox", args...)
	command.WaitDelay = 100 * time.Millisecond
	var output boundedReadinessOutput
	command.Stdout = &output
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("readiness query failed: %w", err)
	}
	return output.Bytes(), nil
}
