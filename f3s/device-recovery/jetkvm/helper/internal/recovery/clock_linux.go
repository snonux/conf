//go:build linux

package recovery

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

// BootTime reads CLOCK_BOOTTIME, including suspend, without wall-clock input.
func BootTime() (int64, error) {
	var value syscall.Timespec
	_, _, errno := syscall.Syscall(syscall.SYS_CLOCK_GETTIME, 7, uintptr(unsafe.Pointer(&value)), 0)
	if errno != 0 {
		return 0, errors.New("CLOCK_BOOTTIME unavailable")
	}
	return int64(value.Sec)*1_000_000_000 + int64(value.Nsec), nil
}

// BootID reads the Linux kernel boot identity.
func BootID() (string, error) {
	data, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", errors.New("read kernel boot identity")
	}
	id := strings.TrimSpace(string(data))
	if len(id) != 36 {
		return "", errors.New("invalid kernel boot identity")
	}
	return id, nil
}
