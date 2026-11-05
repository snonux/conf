package recovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCanaryCleanupTerminalIdempotentAndVendorUnchanged(t *testing.T) {
	for _, terminal := range []string{"confirmed", "expired"} {
		t.Run(terminal, func(t *testing.T) {
			a, k, config := terminalManagement(t, terminal)
			active := filepath.Join(a.Store.Dir, "canary-active.json")
			before, err := os.ReadFile(active)
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if err := CleanupCanary(a, a.Boot); err != nil {
					t.Fatal(err)
				}
			}
			if k.link.Exists || k.module || len(k.routes) > 0 || len(k.rules) > 0 {
				t.Fatal("terminal resources remain")
			}
			after, err := os.ReadFile(active)
			if err != nil || string(after) != string(before) {
				t.Fatal("terminal identity/deadline changed")
			}
			assertVendorUnchanged(t, config)
		})
	}
}

func TestCanaryCleanupPreservesPendingWatchdogEvenPastDeadline(t *testing.T) {
	for _, deadline := range []int64{1, WindowNS} {
		a, k, config := terminalManagement(t, "pending")
		auth := CanaryAuthorization{Hash: a.Profile.Hash(), Generation: genA, Boot: a.Boot, Deadline: deadline}
		privateJSON(t, filepath.Join(a.Store.Dir, "canary-active.json"), auth)
		before := len(k.calls)
		if err := CleanupCanary(a, a.Boot); err == nil {
			t.Fatal("pending watchdog canceled")
		}
		if len(k.calls) != before || !k.link.Exists || !k.module {
			t.Fatal("pending resources changed")
		}
		assertVendorUnchanged(t, config)
	}
}

func TestCanaryCleanupRefusesPendingPrimaryForeignIdentityAndMetadata(t *testing.T) {
	for _, change := range []string{"armed", "trial", "restoring", "profile", "boot", "adapter-boot", "link", "route", "missing-active", "malformed-primary"} {
		t.Run(change, func(t *testing.T) {
			a, k, config := terminalManagement(t, "confirmed")
			callBoot := a.Boot
			switch change {
			case "armed", "trial", "restoring":
				f := setup(t)
				arm(t, f.engine)
				state := mustState(t, f.engine)
				state.Phase = change
				state.Allowance = change == "armed"
				if err := a.Store.Save(state); err != nil {
					t.Fatal(err)
				}
			case "profile":
				a.Profile.Address = "192.168.11.202/24"
			case "boot":
				callBoot = "different-boot-001"
			case "adapter-boot":
				a.Boot = "different-boot-001"
			case "link":
				k.link.MAC = "02:00:5e:00:53:ff"
			case "route":
				k.routes = append(k.routes, "default via 203.0.113.1")
			case "missing-active":
				if err := os.Remove(filepath.Join(a.Store.Dir, "canary-active.json")); err != nil {
					t.Fatal(err)
				}
			case "malformed-primary":
				if err := os.WriteFile(filepath.Join(a.Store.Dir, "state.json"), []byte("bad"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before := len(k.calls)
			if err := CleanupCanary(a, callBoot); err == nil {
				t.Fatal("unsafe cleanup admitted")
			}
			if len(k.calls) != before || !k.link.Exists {
				t.Fatal("refusal changed resources")
			}
			assertVendorUnchanged(t, config)
		})
	}
}

func TestCanaryCleanupFailureRetainsOwnershipAndCanRetry(t *testing.T) {
	a, k, config := terminalManagement(t, "confirmed")
	k.fail = "rmmod"
	if err := CleanupCanary(a, a.Boot); err == nil {
		t.Fatal("busy module falsely clean")
	}
	own, err := a.load()
	if err != nil || !own.Module || !k.module {
		t.Fatal("cleanup ownership forgotten")
	}
	k.fail = ""
	if err := CleanupCanary(a, a.Boot); err != nil {
		t.Fatal(err)
	}
	if k.module || k.link.Exists {
		t.Fatal("retry leaked resources")
	}
	assertVendorUnchanged(t, config)
}

func TestCanaryCleanupIPvlanPreservesPreexistingAndForeignModules(t *testing.T) {
	for _, preexisting := range []bool{false, true} {
		a, k := ipvlanAdapter(t)
		k.ipvlanModule = preexisting
		k.module = true
		if err := a.Ensure(); err != nil {
			t.Fatal(err)
		}
		auth := CanaryAuthorization{Hash: a.Profile.Hash(), Generation: genA, Boot: a.Boot, Confirmed: true, Deadline: WindowNS}
		privateJSON(t, filepath.Join(a.Store.Dir, "canary-active.json"), auth)
		for range 2 {
			if err := CleanupCanary(a, a.Boot); err != nil {
				t.Fatal(err)
			}
		}
		if k.link.Exists || k.ipvlanModule != preexisting || !k.module {
			t.Fatal("selected/preexisting/foreign module ownership lost")
		}
	}
}
