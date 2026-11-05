package recovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagementBootTerminalCleansWithoutBlockingVendorStartup(t *testing.T) {
	for _, terminal := range []string{"confirmed", "expired"} {
		for _, boot := range []string{"same-boot", "new-boot"} {
			for _, module := range []string{"owned-module", "preexisting-module"} {
				t.Run(terminal+"/"+boot+"/"+module, func(t *testing.T) {
					a, k, config := terminalManagement(t, terminal)
					if module == "preexisting-module" {
						if err := a.Cleanup(); err != nil {
							t.Fatal(err)
						}
						k.module = true
						if err := a.Ensure(); err != nil {
							t.Fatal(err)
						}
					}
					if boot == "new-boot" {
						a.Boot = "boot-after-terminal-001"
						k.link, k.routes, k.rules = Link{}, nil, nil
						k.module = module == "preexisting-module"
					}
					authPath := filepath.Join(a.Store.Dir, "canary-active.json")
					before, err := os.ReadFile(authPath)
					if err != nil {
						t.Fatal(err)
					}
					start := func(State) error { t.Fatal("terminal canary started worker"); return nil }
					for range 2 {
						if err := ManagementBoot(a, a.Boot, 1, start); err != nil {
							t.Fatal(err)
						}
					}
					if k.link.Exists || len(k.routes) != 0 || len(k.rules) != 0 || k.module != (module == "preexisting-module") {
						t.Fatal("terminal cleanup did not preserve ownership baseline")
					}
					after, err := os.ReadFile(authPath)
					if err != nil || string(before) != string(after) {
						t.Fatal("terminal cleanup changed authorization or deadline")
					}
					assertVendorUnchanged(t, config)
				})
			}
		}
	}
}

func TestManagementBootTerminalRefusesForeignOwnershipAndCleanupFailure(t *testing.T) {
	for _, terminal := range []string{"confirmed", "expired"} {
		for _, failure := range []string{"foreign-link", "foreign-policy", "foreign-new-boot", "busy-module"} {
			t.Run(terminal+"/"+failure, func(t *testing.T) {
				a, k, config := terminalManagement(t, terminal)
				switch failure {
				case "foreign-link":
					k.link.MAC = "02:00:5e:00:53:ff"
				case "foreign-policy":
					k.routes = append(k.routes, "203.0.113.0/24 dev foreign")
				case "foreign-new-boot":
					a.Boot = "boot-after-terminal-001"
				case "busy-module":
					k.fail = "rmmod"
				}
				mutations := len(k.calls)
				if err := ManagementBoot(a, a.Boot, 1, func(State) error {
					t.Fatal("failed terminal cleanup started worker")
					return nil
				}); err == nil {
					t.Fatal("unsafe terminal cleanup returned success")
				}
				if failure != "busy-module" && len(k.calls) != mutations {
					t.Fatal("foreign resource was mutated")
				}
				if failure == "busy-module" && !k.module {
					t.Fatal("busy module lost")
				}
				assertVendorUnchanged(t, config)
			})
		}
	}
}

func TestManagementBootTerminalStillRefusesPendingPrimary(t *testing.T) {
	for _, terminal := range []string{"confirmed", "expired"} {
		for _, phase := range []string{"armed", "trial", "restoring"} {
			t.Run(terminal+"/"+phase, func(t *testing.T) {
				a, k, config := terminalManagement(t, terminal)
				f := setup(t)
				arm(t, f.engine)
				state := mustState(t, f.engine)
				state.Phase = phase
				state.Allowance = phase == "armed"
				if err := a.Store.Save(state); err != nil {
					t.Fatal(err)
				}
				mutations := len(k.calls)
				if err := ManagementBoot(a, a.Boot, 1, func(State) error {
					t.Fatal("pending primary started management worker")
					return nil
				}); err == nil {
					t.Fatal("pending primary admitted terminal cleanup")
				}
				if len(k.calls) != mutations || !k.link.Exists {
					t.Fatal("pending primary's recovery resources changed")
				}
				assertVendorUnchanged(t, config)
			})
		}
	}
}

func terminalManagement(t *testing.T, terminal string) (Adapter, *kernel, string) {
	t.Helper()
	a, k := setupAdapter(t)
	if err := a.Ensure(); err != nil {
		t.Fatal(err)
	}
	auth := CanaryAuthorization{Hash: a.Profile.Hash(), Generation: genA, Boot: a.Boot, Deadline: WindowNS, Confirmed: terminal == "confirmed", Expired: terminal == "expired"}
	privateJSON(t, filepath.Join(a.Store.Dir, "canary-active.json"), auth)
	config := filepath.Join(a.Store.Dir, "vendor-config.json")
	if err := os.WriteFile(config, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	return a, k, config
}

func assertVendorUnchanged(t *testing.T, config string) {
	t.Helper()
	data, err := os.ReadFile(config)
	if err != nil || string(data) != original {
		t.Fatal("terminal management changed vendor configuration")
	}
}
