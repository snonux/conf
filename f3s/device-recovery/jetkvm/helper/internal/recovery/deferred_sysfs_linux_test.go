//go:build linux

package recovery

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestDeferredNativeSysfsClassComposition(t *testing.T) {
	cases := []struct {
		name   string
		change func(*testing.T, string, string, string)
	}{
		{"free", nil},
		{"owned upper", func(t *testing.T, _, device, _ string) {
			writeSysfsFixture(t, filepath.Join(device, "upper_jkrecover"))
		}},
		{"foreign upper", func(t *testing.T, _, device, _ string) {
			writeSysfsFixture(t, filepath.Join(device, "upper_foreign"))
		}},
		{"unknown device", func(t *testing.T, _, device, _ string) {
			if err := os.Remove(device); err != nil {
				t.Fatal(err)
			}
		}},
		{"foreign target", func(t *testing.T, root, _, class string) {
			outside := t.TempDir()
			if err := os.Remove(class); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, class); err != nil {
				t.Fatal(err)
			}
		}},
		{"too many entries", func(t *testing.T, _, device, _ string) {
			for i := 0; i < 129; i++ {
				writeSysfsFixture(t, filepath.Join(device, fmt.Sprintf("attribute%d", i)))
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, device, class := sysfsClassFixture(t)
			if tc.change != nil {
				tc.change(t, root, device, class)
			}
			f := setupDeferred(t)
			// Exercise the production native composition, not a stand-in upper check.
			f.ops.upperFree = nativeDeferredOpsAt(f.adapter, root).upperFree
			before, err := os.ReadFile(filepath.Join(f.adapter.Store.Dir, "canary-active.json"))
			if err != nil {
				t.Fatal(err)
			}
			err = beginDeferredManagement(context.Background(), f.adapter, f.ops, f.watch(t), f.wait(t))
			if tc.change == nil {
				if err != nil {
					t.Fatal(err)
				}
				if len(f.starts) != 2 {
					t.Fatal("native free parent did not reach watchdog and waiter")
				}
				return
			}
			if err == nil {
				t.Fatal("uncertain or occupied native parent admitted")
			}
			after, err := os.ReadFile(filepath.Join(f.adapter.Store.Dir, "canary-active.json"))
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) || len(f.starts) != 0 || len(f.kernel.calls) != 0 {
				t.Fatal("refusal changed authorization, launched or mutated")
			}
		})
	}
}

func TestDeferredNativeSysfsPreservesGenericSymlinkRefusal(t *testing.T) {
	root, device, class := sysfsClassFixture(t)
	if _, err := (nativeReadinessReader{}).names(context.Background(), class, 128); err == nil {
		t.Fatal("generic reader followed class or hook symlink")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if nativeParentUpperFree(ctx, root) == nil {
		t.Fatal("cancelled observation admitted")
	}
	classInfo, err := os.Lstat(class)
	if err != nil {
		t.Fatal(err)
	}
	deviceInfo, err := os.Lstat(device)
	if err != nil {
		t.Fatal(err)
	}
	// Keep the old inode alive until the replacement exists: no inode-reuse
	// dependence in the identity-change regression.
	replacement := class + ".replacement"
	if err := os.Symlink(device, replacement); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, class); err != nil {
		t.Fatal(err)
	}
	if stableParentSysfs(context.Background(), class, device, classInfo, deviceInfo) == nil {
		t.Fatal("replaced class identity admitted")
	}
}

func sysfsClassFixture(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	device := filepath.Join(root, "devices/platform/example/net/eth0")
	class := filepath.Join(root, "class/net/eth0")
	for _, dir := range []string{device, filepath.Dir(class)} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("../../devices/platform/example/net/eth0", class); err != nil {
		t.Fatal(err)
	}
	return root, device, class
}

func writeSysfsFixture(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
}
