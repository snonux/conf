//go:build linux

package recovery

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Native executes fixed operations against the installed BusyBox and kernel.
type Native struct{}

// Run never invokes a shell or includes command output in diagnostics.
func (Native) Run(args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, "/bin/busybox", args...).Run(); err != nil {
		return errors.New("BusyBox operation failed")
	}
	return nil
}

// Output collects kernel policy descriptions without logging addresses.
func (Native) Output(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, "/bin/busybox", args...).Output()
	if err != nil {
		return "", errors.New("BusyBox inspection failed")
	}
	return string(data), nil
}

// Module reports whether the selected management module is already loaded; it never unloads pre-existing modules.
func (Native) Module(kind string) (bool, error) {
	if kind != "macvlan" && kind != "ipvlan" {
		return false, errors.New("unsupported module inspection")
	}
	_, err := os.Stat("/sys/module/" + kind)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// CreateIPvlan encodes L2 explicitly: shipped BusyBox cannot encode IPvlan mode.
// It never sets a MAC or parent flags, and requests exclusive creation.
func (Native) CreateIPvlan(parent, child string) error {
	if parent != "eth0" || child != "jkrecover" {
		return errors.New("refuse unrelated IPvlan creation")
	}
	iface, err := net.InterfaceByName(parent)
	if err != nil {
		return errors.New("inspect IPvlan parent index")
	}
	entries, err := os.ReadDir("/sys/class/net/eth0")
	if err != nil {
		return errors.New("inspect IPvlan parent upper links")
	}
	if !ipvlanParentFree(entries) {
		return errors.New("IPvlan parent already has an upper link")
	}
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW|syscall.SOCK_CLOEXEC, syscall.NETLINK_ROUTE)
	if err != nil {
		return errors.New("open IPvlan netlink socket")
	}
	defer func() { _ = syscall.Close(fd) }()
	for _, option := range []int{syscall.SO_RCVTIMEO, syscall.SO_SNDTIMEO} {
		if err := syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, option, &syscall.Timeval{Sec: 3}); err != nil {
			return errors.New("bound IPvlan netlink operation")
		}
	}
	if err := syscall.Sendto(fd, ipvlanRequest(iface.Index), 0, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK}); err != nil {
		return errors.New("send IPvlan create")
	}
	data := make([]byte, 4096)
	n, from, err := syscall.Recvfrom(fd, data, 0)
	if err != nil {
		return errors.New("receive IPvlan acknowledgment")
	}
	peer, ok := from.(*syscall.SockaddrNetlink)
	if !ok || peer.Pid != 0 {
		return errors.New("foreign IPvlan acknowledgment")
	}
	return ipvlanAck(data[:n])
}

// IPvlan mode is per port; never change a foreign sibling's mode.
func ipvlanParentFree(entries []os.DirEntry) bool {
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "upper_") {
			return false
		}
	}
	return true
}

func ipvlanRequest(parent int) []byte {
	// Linux UAPI: LINK=5, IFNAME=3, LINKINFO=18; INFO_KIND=1,
	// INFO_DATA=2, IPVLAN_MODE=1, L2=0. No IFLA_ADDRESS is sent.
	index := make([]byte, 4)
	binary.NativeEndian.PutUint32(index, uint32(parent))
	info := append(linkAttr(1, []byte("ipvlan\x00")), linkAttr(2|0x8000, linkAttr(1, []byte{0, 0}))...)
	attrs := append(linkAttr(5, index), linkAttr(3, []byte("jkrecover\x00"))...)
	attrs = append(attrs, linkAttr(18|0x8000, info)...)
	data := make([]byte, 32) // nlmsghdr + ifinfomsg, family AF_UNSPEC
	binary.NativeEndian.PutUint32(data[:4], uint32(len(data)+len(attrs)))
	binary.NativeEndian.PutUint16(data[4:6], syscall.RTM_NEWLINK)
	binary.NativeEndian.PutUint16(data[6:8], syscall.NLM_F_REQUEST|syscall.NLM_F_ACK|syscall.NLM_F_CREATE|syscall.NLM_F_EXCL)
	binary.NativeEndian.PutUint32(data[8:12], 1)
	return append(data, attrs...)
}

func linkAttr(kind uint16, value []byte) []byte {
	n := len(value) + 4
	data := make([]byte, (n+3)&^3)
	binary.NativeEndian.PutUint16(data[:2], uint16(n))
	binary.NativeEndian.PutUint16(data[2:4], kind)
	copy(data[4:], value)
	return data
}

func ipvlanAck(data []byte) error {
	messages, err := syscall.ParseNetlinkMessage(data)
	if err != nil || len(messages) != 1 {
		return errors.New("invalid IPvlan acknowledgment")
	}
	message := messages[0]
	if message.Header.Seq != 1 || message.Header.Type != syscall.NLMSG_ERROR || len(message.Data) < 20 || binary.NativeEndian.Uint32(message.Data[:4]) != 0 || binary.NativeEndian.Uint16(message.Data[8:10]) != syscall.RTM_NEWLINK || binary.NativeEndian.Uint32(message.Data[12:16]) != 1 {
		return errors.New("IPvlan exclusive create failed")
	}
	return nil
}

// DisableIPv6 changes only the validated child before it is brought up.
func (Native) DisableIPv6(link string) error {
	if link != "jkrecover" {
		return errors.New("refuse unrelated IPv6 setting")
	}
	return os.WriteFile("/proc/sys/net/ipv6/conf/"+link+"/disable_ipv6", []byte("1\n"), 0600)
}

// Link obtains kind and parent from RTM_GETLINK rather than assuming an Ethernet link is macvlan.
func (Native) Link(name string) (Link, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		if _, statErr := os.Lstat(filepath.Join("/sys/class/net", name)); errors.Is(statErr, os.ErrNotExist) {
			return Link{}, nil
		}
		return Link{}, errors.New("inspect management interface")
	}
	result := Link{Exists: true, Index: iface.Index, MAC: iface.HardwareAddr.String(), Up: iface.Flags&net.FlagUp != 0}
	addresses, err := iface.Addrs()
	if err != nil {
		return result, errors.New("inspect interface addresses")
	}
	for _, address := range addresses {
		result.Addresses = append(result.Addresses, address.String())
	}
	data, err := syscall.NetlinkRIB(syscall.RTM_GETLINK, syscall.AF_UNSPEC)
	if err != nil {
		return result, errors.New("inspect kernel link kind")
	}
	messages, err := syscall.ParseNetlinkMessage(data)
	if err != nil {
		return result, errors.New("parse kernel links")
	}
	for _, message := range messages {
		if message.Header.Type != syscall.RTM_NEWLINK || len(message.Data) < 16 || int(binary.NativeEndian.Uint32(message.Data[4:8])) != iface.Index {
			continue
		}
		attrs, err := syscall.ParseNetlinkRouteAttr(&message)
		if err != nil {
			return result, errors.New("parse kernel link attributes")
		}
		for _, attr := range attrs {
			switch attr.Attr.Type & 0x3fff {
			case 5:
				if len(attr.Value) >= 4 {
					parent, err := net.InterfaceByIndex(int(binary.NativeEndian.Uint32(attr.Value)))
					if err != nil {
						return result, errors.New("inspect parent link")
					}
					result.Parent, result.ParentIndex = parent.Name, parent.Index
				}
			case 18:
				kind, mode, observed, err := linkInfo(attr.Value)
				if err != nil {
					return result, err
				}
				result.Kind, result.Mode, result.ModeObserved = kind, mode, observed
			}
		}
	}
	disabled, err := os.ReadFile("/proc/sys/net/ipv6/conf/" + name + "/disable_ipv6")
	if err != nil {
		return result, errors.New("inspect child IPv6 setting")
	}
	result.IPv6Disabled = strings.TrimSpace(string(disabled)) == "1"
	return result, nil
}

// Identity matches the vendor GetDeviceID implementation's CPU serial source.
func Identity() (string, error) {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return "", errors.New("read device hardware identity")
	}
	return serial(data)
}

func serial(data []byte) (string, error) {
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok && strings.TrimSpace(key) == "Serial" {
			id := strings.TrimSpace(value)
			if _, err := strconv.ParseUint(id, 16, 64); err == nil && len(id) > 0 && len(id) <= 16 {
				return id, nil
			}
		}
	}
	return "", errors.New("hardware device identity missing")
}

// linkInfo parses IFLA_INFO_KIND and nested IFLA_INFO_DATA/IFLA_IPVLAN_MODE.
// A missing mode is not equivalent to observed L2 (numeric zero).
func linkInfo(data []byte) (string, uint16, bool, error) {
	attrs, err := nestedAttrs(data)
	if err != nil {
		return "", 0, false, err
	}
	kind := strings.TrimRight(string(attrs[1]), "\x00")
	if kind == "" {
		return "", 0, false, errors.New("missing kernel link kind")
	}
	if kind != "ipvlan" {
		return kind, 0, false, nil
	}
	info, err := nestedAttrs(attrs[2])
	if err != nil {
		return "", 0, false, err
	}
	mode, ok := info[1]
	if !ok || len(mode) != 2 {
		return "", 0, false, errors.New("missing or invalid IPvlan mode")
	}
	return kind, binary.NativeEndian.Uint16(mode), true, nil
}

func nestedAttrs(data []byte) (map[uint16][]byte, error) {
	attrs := make(map[uint16][]byte)
	for len(data) != 0 {
		if len(data) < 4 {
			return nil, errors.New("truncated nested link attribute")
		}
		length := int(binary.NativeEndian.Uint16(data[:2]))
		kind := binary.NativeEndian.Uint16(data[2:4]) & 0x3fff
		if length < 4 || length > len(data) {
			return nil, errors.New("invalid nested link attribute")
		}
		if _, exists := attrs[kind]; exists {
			return nil, errors.New("duplicate nested link attribute")
		}
		attrs[kind] = data[4:length]
		aligned := (length + 3) &^ 3
		if aligned > len(data) {
			return nil, errors.New("invalid link attribute alignment")
		}
		data = data[aligned:]
	}
	return attrs, nil
}

func linkKind(data []byte) (string, error) {
	kind, _, _, err := linkInfo(data)
	return kind, err
}

// Prerequisites rejects unreviewed OS/ABI/storage before any state or network mutation.
func Prerequisites() error {
	var uts syscall.Utsname
	if err := syscall.Uname(&uts); err != nil {
		return errors.New("inspect Linux ABI")
	}
	machine := utsText(uts.Machine[:])
	release := utsText(uts.Release[:])
	if machine != "armv7l" || release != "5.10.160" {
		return errors.New("unreviewed kernel or ARM ABI")
	}
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return errors.New("inspect userdata mount")
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 4 && fields[1] == "/userdata" && fields[2] == "ext4" && has(strings.Split(fields[3], ","), "rw") {
			return nil
		}
	}
	return errors.New("userdata must be mounted ext4 read-write")
}

// NoVendorApp prevents accidentally using the synchronous merge command at runtime.
func NoVendorApp() error {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return errors.New("inspect pre-app process state")
	}
	for _, entry := range entries {
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		data, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "comm"))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return errors.New("cannot establish vendor app absence")
		}
		if strings.TrimSpace(string(data)) == "jetkvm_app" {
			return errors.New("vendor app running; pre-app merge forbidden")
		}
	}
	return nil
}
func utsText[T ~int8 | ~uint8](value []T) string {
	var result []byte
	for _, c := range value {
		if c == 0 {
			break
		}
		result = append(result, byte(c))
	}
	return string(result)
}
