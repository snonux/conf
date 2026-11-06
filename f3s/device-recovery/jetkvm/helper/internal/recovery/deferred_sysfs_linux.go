//go:build linux

package recovery

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// nativeParentUpperFree handles the kernel's class-interface symlink only.
// Generic directory reads retain O_NOFOLLOW for untrusted DHCP hook paths.
func nativeParentUpperFree(ctx context.Context, sysRoot string) error {
	class := filepath.Join(sysRoot, "class/net/eth0")
	before, err := os.Lstat(class)
	if err != nil || before.Mode()&os.ModeSymlink == 0 {
		return errors.New("parent class entry must be a kernel symlink")
	}
	target, err := filepath.EvalSymlinks(class)
	if err != nil {
		return errors.New("resolve parent sysfs device")
	}
	devices, err := filepath.Abs(filepath.Join(sysRoot, "devices"))
	if err != nil {
		return errors.New("resolve sysfs devices root")
	}
	relative, err := filepath.Rel(devices, target)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.Base(target) != "eth0" || filepath.Base(filepath.Dir(target)) != "net" {
		return errors.New("parent class target outside its sysfs device")
	}
	device, err := os.Lstat(target)
	if err != nil || !device.IsDir() {
		return errors.New("parent sysfs device unavailable")
	}
	names, err := (nativeReadinessReader{}).names(ctx, target, 128)
	if err != nil {
		return err
	}
	for _, name := range names {
		if strings.HasPrefix(name, "upper_") {
			return errors.New("deferred parent has upper link")
		}
	}
	return stableParentSysfs(ctx, class, target, before, device)
}

func stableParentSysfs(ctx context.Context, class, target string, before, device os.FileInfo) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	after, err := os.Lstat(class)
	if err != nil || !os.SameFile(before, after) {
		return errors.New("parent class entry changed")
	}
	resolved, err := filepath.EvalSymlinks(class)
	if err != nil || resolved != target {
		return errors.New("parent class target changed")
	}
	current, err := os.Lstat(target)
	if err != nil || !current.IsDir() || !os.SameFile(device, current) {
		return errors.New("parent sysfs device changed")
	}
	return nil
}
