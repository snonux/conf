//go:build linux

package recovery

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type readinessTestReader struct {
	files       map[string][]byte
	metadata    map[string]ReadinessFile
	directories map[string][]string
	outputs     map[string][]byte
	failures    map[string]error
	reads       map[string]int
	change      func(*readinessTestReader, string)
}

func (r *readinessTestReader) read(ctx context.Context, path string, limit int, follow bool) ([]byte, ReadinessFile, error) {
	if err := ctx.Err(); err != nil {
		return nil, ReadinessFile{}, err
	}
	r.reads[path]++
	if r.change != nil {
		r.change(r, path)
	}
	if err := r.failures[path]; err != nil {
		return nil, ReadinessFile{}, err
	}
	data, ok := r.files[path]
	if !ok {
		return nil, ReadinessFile{}, os.ErrNotExist
	}
	if len(data) > limit {
		return nil, ReadinessFile{}, errors.New("fixture read exceeds limit")
	}
	if follow && path != "/proc/42/exe" {
		return nil, ReadinessFile{}, errors.New("unexpected follow")
	}
	return data, r.metadata[path], nil
}
func (r *readinessTestReader) names(ctx context.Context, path string, limit int) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := r.failures[path]; err != nil {
		return nil, err
	}
	names, ok := r.directories[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	if len(names) > limit {
		return nil, errors.New("fixture directory exceeds limit")
	}
	return names, nil
}
func (r *readinessTestReader) resolve(path string) (string, error) { return path, nil }
func (r *readinessTestReader) output(ctx context.Context, args ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key := strings.Join(args, " ")
	if err := r.failures[key]; err != nil {
		return nil, err
	}
	data, ok := r.outputs[key]
	if !ok {
		return nil, errors.New("unexpected command")
	}
	return data, nil
}
func readinessTestStat(pid, ppid int, comm string) []byte {
	// state=field3, starttime=field22; no assumption that comm lacks spaces or ')'.
	fields := []string{"S", fmt.Sprint(ppid), fmt.Sprint(pid), fmt.Sprint(pid)}
	for len(fields) < 19 {
		fields = append(fields, "0")
	}
	fields = append(fields, "123")
	return []byte(fmt.Sprintf("%d (%s) %s\n", pid, comm, strings.Join(fields, " ")))
}
func readinessTestSource() *readinessTestReader {
	r := &readinessTestReader{files: map[string][]byte{
		"/proc/sys/kernel/random/boot_id": []byte("current-boot\n"), "/bin/busybox": []byte("fixture ELF"), readinessHandler: []byte("fixture pinned script"),
		"/proc/42/stat": readinessTestStat(42, 1, "udhcpc"), "/proc/42/cmdline": []byte("udhcpc\x00-i\x00eth0\x00-x\x00hostname:a-example\x00"), "/proc/42/exe": []byte("fixture ELF"),
		"/proc/1/stat":                readinessTestStat(1, 0, "init (worker)"),
		"/sys/class/net/eth0/address": []byte("30:00:00:00:00:01\n"), "/sys/class/net/eth0/flags": []byte("0x1003\n"), "/sys/class/net/eth0/ifindex": []byte("2\n"),
		readinessLeasePath: readinessFixtureLease(), "/proc/self/mountinfo": []byte("1 0 8:1 / / rw - ext4 /dev/root rw\n5 1 0:7 / /run rw - tmpfs tmpfs rw\n"),
		"/etc/resolv.conf": []byte("search a.example # eth0\nnameserver 192.0.2.53 # eth0\nnameserver 192.0.2.54 # eth0\n"),
	}, metadata: map[string]ReadinessFile{}, directories: map[string][]string{"/proc": {"1", "42", "self"}}, outputs: map[string][]byte{
		"ip -4 address show dev eth0": []byte("2: eth0: <UP>\n    inet 192.0.2.10/24 brd 192.0.2.255 scope global eth0\n"),
		"ip -4 route show table 254":  []byte("default via 192.0.2.1 dev eth0\n192.0.2.0/24 dev eth0 scope link src 192.0.2.10\n"),
	}, failures: map[string]error{}, reads: map[string]int{}}
	for path := range r.files {
		r.metadata[path] = readinessFixtureFile(false)
	}
	r.metadata["/bin/busybox"] = readinessFixtureFile(true)
	r.metadata["/proc/42/exe"] = readinessFixtureFile(true)
	handler := readinessFixtureFile(true)
	handler.Hash = readinessHandlerSHA
	r.metadata[readinessHandler] = handler
	return r
}

func TestCollectParentReadinessNativeFormatsAndNoOptionalChildrenAPI(t *testing.T) {
	r := readinessTestSource()
	first, err := collectParentReadiness(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	second, err := collectParentReadiness(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateParentReadiness("current-boot", "30:00:00:00:00:01", first, second); err != nil {
		t.Fatal(err)
	}
	for path := range r.reads {
		if strings.Contains(path, "/children") || strings.Contains(path, "/disable_ipv6") {
			t.Fatal("unexpected optional/child-only dependency")
		}
	}
}

func TestCollectParentReadinessRejectsUnknownAndChangingEvidence(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*readinessTestReader)
	}{
		{"disappeared enumerated process", func(r *readinessTestReader) { r.directories["/proc"] = append(r.directories["/proc"], "99") }},
		{"unreadable process", func(r *readinessTestReader) { r.failures["/proc/1/stat"] = os.ErrPermission }},
		{"malformed process", func(r *readinessTestReader) { r.files["/proc/1/stat"] = []byte("malformed") }},
		{"unknown owner argv", func(r *readinessTestReader) { delete(r.files, "/proc/42/cmdline") }},
		{"zombie callback", func(r *readinessTestReader) {
			r.directories["/proc"] = append(r.directories["/proc"], "99")
			r.files["/proc/99/stat"] = []byte(strings.Replace(string(readinessTestStat(99, 42, "callback")), ") S ", ") Z ", 1))
		}},
		{"custom handler", func(r *readinessTestReader) {
			r.files["/proc/42/cmdline"] = []byte("udhcpc\x00-ieth0\x00-s/custom\x00")
		}},
		{"unknown hook directory", func(r *readinessTestReader) { r.failures[readinessHandler+".d"] = os.ErrPermission }},
		{"executable hook", func(r *readinessTestReader) {
			r.directories[readinessHandler+".d"] = []string{"hook"}
			r.files[readinessHandler+".d/hook"] = []byte("hook")
			r.metadata[readinessHandler+".d/hook"] = readinessFixtureFile(true)
		}},
		{"persistent backing", func(r *readinessTestReader) {
			r.files["/proc/self/mountinfo"] = []byte("1 0 8:1 / / rw - ext4 /dev/root rw\n")
		}},
		{"unknown route status", func(r *readinessTestReader) { r.failures["ip -4 route show table 254"] = errors.New("query failed") }},
		{"malformed parent flags", func(r *readinessTestReader) { r.files["/sys/class/net/eth0/flags"] = []byte("0xxx") }},
		{"lease rewritten", func(r *readinessTestReader) {
			r.change = func(r *readinessTestReader, path string) {
				if path == readinessLeasePath && r.reads[path] == 2 {
					f := r.metadata[path]
					f.Hash = strings.Repeat("b", 64)
					r.metadata[path] = f
				}
			}
		}},
		{"owner birth race", func(r *readinessTestReader) {
			r.change = func(r *readinessTestReader, path string) {
				if path == "/proc/42/stat" && r.reads[path] == 2 {
					r.files[path] = []byte(strings.Replace(string(r.files[path]), "123", "124", 1))
				}
			}
		}},
		{"new boot", func(r *readinessTestReader) {
			r.change = func(r *readinessTestReader, path string) {
				if path == "/proc/sys/kernel/random/boot_id" && r.reads[path] == 2 {
					r.files[path] = []byte("another-boot\n")
				}
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := readinessTestSource()
			tc.mutate(r)
			s, err := collectParentReadiness(context.Background(), r)
			if err == nil {
				err = ValidateParentReadiness("current-boot", "30:00:00:00:00:01", s, s)
			}
			if err == nil {
				t.Fatal("accepted unknown/changing evidence")
			}
		})
	}
}

func TestReadinessNativeReaderBoundsSymlinksAndCancellation(t *testing.T) {
	r := nativeReadinessReader{}
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "file")
	if err := os.WriteFile(path, []byte("12345"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.read(ctx, path, 4, false); err == nil {
		t.Fatal("unbounded file read")
	}
	if err := os.Symlink(path, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.read(ctx, filepath.Join(dir, "link"), 10, false); err == nil {
		t.Fatal("followed substituted regular file")
	}
	if err := os.Symlink(dir, filepath.Join(dir, "dirlink")); err != nil {
		t.Fatal(err)
	}
	if _, err := r.names(ctx, filepath.Join(dir, "dirlink"), 10); err == nil {
		t.Fatal("followed hook directory symlink")
	}
	if _, err := r.names(ctx, dir, 1); err == nil {
		t.Fatal("unbounded directory enumeration")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := r.read(cancelled, path, 10, false); !errors.Is(err, context.Canceled) {
		t.Fatal("ignored cancellation")
	}
	var output boundedReadinessOutput
	if _, err := output.Write(make([]byte, readinessLimit+1)); err == nil || output.Len() != 0 {
		t.Fatal("retained oversized command output")
	}
}

func TestReadinessOtherInterfaceDoesNotHideUnknownOptions(t *testing.T) {
	if !readinessOtherInterface([]string{"udhcpc", "-i", "eth1"}) {
		t.Fatal("reviewed other interface not recognized")
	}
	for _, args := range [][]string{{"udhcpc", "-ieth1", "--interface=eth0"}, {"udhcpc", "-ieth1", "-ieth0"}, {"udhcpc", "-ieth1", "-s/custom"}, {"udhcpc", "-ieth1", "-f"}} {
		if readinessOtherInterface(args) {
			t.Fatal("unknown selection hidden as other interface")
		}
	}
}

func TestReadinessNativeReaderPreservesSpecialModeBits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mode-fixture")
	if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		mode os.FileMode
		raw  uint32
	}{
		{0755 | os.ModeSetuid, 04755},
		{0755 | os.ModeSetgid, 02755},
		{0755 | os.ModeSticky, 01755},
	} {
		if err := os.Chmod(path, tc.mode); err != nil {
			t.Fatal(err)
		}
		_, identity, err := (nativeReadinessReader{}).read(context.Background(), path, 64, false)
		if err != nil {
			t.Fatal(err)
		}
		if identity.Mode != tc.raw {
			t.Fatalf("mode got %o want %o", identity.Mode, tc.raw)
		}
		if readyFile(identity, true) {
			t.Fatal("accepted executable with special bits")
		}
	}
}
