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

// Module reports whether macvlan is already loaded; it never unloads pre-existing modules.
func (Native) Module() (bool, error) {
	_, err := os.Stat("/sys/module/macvlan")
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
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
					result.Parent = parent.Name
				}
			case 18:
				kind, err := linkKind(attr.Value)
				if err != nil {
					return result, err
				}
				result.Kind = kind
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

func linkKind(data []byte) (string, error) {
	for len(data) >= 4 {
		length := int(binary.NativeEndian.Uint16(data[:2]))
		kind := binary.NativeEndian.Uint16(data[2:4]) & 0x3fff
		if length < 4 || length > len(data) {
			return "", errors.New("invalid nested link attribute")
		}
		if kind == 1 {
			return strings.TrimRight(string(data[4:length]), "\x00"), nil
		}
		aligned := (length + 3) &^ 3
		if aligned > len(data) {
			return "", errors.New("invalid link attribute alignment")
		}
		data = data[aligned:]
	}
	return "", errors.New("missing kernel link kind")
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
