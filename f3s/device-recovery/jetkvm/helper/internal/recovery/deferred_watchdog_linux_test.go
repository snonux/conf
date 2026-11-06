//go:build linux

package recovery

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// Query tracing distinguishes the new read-only empty terminal path from typed
// Cleanup, whose physical-parent admission must remain in every fallback.
type deferredCleanupRunner struct {
	Runner
	parents       atomic.Int64
	unknownModule bool
	unknownPolicy bool
}

func (r *deferredCleanupRunner) Link(name string) (Link, error) {
	if name == "eth0" {
		r.parents.Add(1)
	}
	return r.Runner.Link(name)
}
func (r *deferredCleanupRunner) Module(kind string) (bool, error) {
	if r.unknownModule {
		return false, errors.New("unknown module read")
	}
	return r.Runner.Module(kind)
}
func (r *deferredCleanupRunner) Output(args ...string) (string, error) {
	if r.unknownPolicy {
		return "", errors.New("unknown policy read")
	}
	return r.Runner.Output(args...)
}

func TestDeferredFailedReadinessEmptyWatchdogExpiresAndExitsWithoutParent(t *testing.T) {
	f := setupDeferred(t)
	f.begin(t)
	r := &deferredCleanupRunner{Runner: f.kernel}
	f.adapter.Runner = r
	f.kernel.parent.MAC = "8e:00:00:00:00:01"
	f.kernel.parent.Up = false
	f.ops.collect = func(context.Context) (ParentReadinessSnapshot, error) {
		s := f.snapshot
		s.Parent.MAC = f.kernel.parent.MAC
		s.Parent.Up = false
		return s, nil
	}
	var clock atomic.Int64
	clock.Store(f.tick)
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = read.Close() }()
	done := make(chan error, 1)
	go func() {
		done <- workDeferredCanary(f.adapter, f.adapter.Boot, genA, func() (int64, error) { return clock.Load(), nil }, write, f.ops)
	}()
	var identity WorkerIdentity
	if err := json.NewDecoder(read).Decode(&identity); err != nil {
		t.Fatal(err)
	}
	if identity.Deadline != f.watchdog.Deadline {
		t.Fatal("watchdog moved original deadline")
	}
	if continueDeferredManagement(context.Background(), f.adapter, f.ops, f.waiter) == nil {
		t.Fatal("unready parent accepted")
	}
	var pending CanaryAuthorization
	if err := readPrivate(filepath.Join(f.adapter.Store.Dir, "canary-active.json"), &pending); err != nil {
		t.Fatal(err)
	}
	if pending.Expired || pending.Deadline != int64(time.Second)+WindowNS {
		t.Fatal("30s readiness failure expired or renewed 120s claim")
	}
	select {
	case <-done:
		t.Fatal("watchdog exited before original expiry")
	default:
	}
	clock.Store(identity.Deadline)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("empty deferred watchdog failed to exit")
	}
	var expired CanaryAuthorization
	if err := readPrivate(filepath.Join(f.adapter.Store.Dir, "canary-active.json"), &expired); err != nil {
		t.Fatal(err)
	}
	if !expired.Expired || expired.BootAllowance || expired.Deadline != identity.Deadline {
		t.Fatal("original expiry not saved")
	}
	if r.parents.Load() != 0 || len(f.kernel.calls) != 0 {
		t.Fatal("empty expiry used parent admission or network mutation")
	}
	if f.kernel.parent.MAC != "8e:00:00:00:00:01" || f.kernel.parent.Up {
		t.Fatal("expiry changed vendor parent")
	}
	if _, err := os.Lstat(filepath.Join(f.adapter.Store.Dir, "network-owned.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("empty terminal path relabelled ownership")
	}
}

func TestDeferredWatchdogEmptyShortcutRefusesNonzeroForeignAndUnknown(t *testing.T) {
	cases := []struct {
		name   string
		change func(*deferredFixture, *deferredCleanupRunner)
	}{
		{"nonzero old ownership", func(f *deferredFixture, r *deferredCleanupRunner) {
			privateJSON(t, filepath.Join(f.adapter.Store.Dir, "network-owned.json"), Owned{Hash: f.adapter.Profile.Hash(), Boot: "old-boot", Link: true, Module: true, Index: 17})
		}},
		{"global IPvlan", func(f *deferredFixture, r *deferredCleanupRunner) { f.kernel.ipvlanModule = true }},
		{"global macvlan", func(f *deferredFixture, r *deferredCleanupRunner) { f.kernel.module = true }},
		{"foreign child", func(f *deferredFixture, r *deferredCleanupRunner) {
			f.kernel.link = Link{Exists: true, Index: 99, MAC: "02:00:00:00:00:99"}
		}},
		{"foreign table", func(f *deferredFixture, r *deferredCleanupRunner) { f.kernel.routes = []string{"foreign"} }},
		{"foreign rule", func(f *deferredFixture, r *deferredCleanupRunner) { f.kernel.rules = []string{"1201: foreign"} }},
		{"foreign upper", func(f *deferredFixture, r *deferredCleanupRunner) {
			f.ops.upperFree = func() error { return errors.New("foreign upper") }
		}},
		{"unknown module", func(f *deferredFixture, r *deferredCleanupRunner) { r.unknownModule = true }},
		{"unknown policy", func(f *deferredFixture, r *deferredCleanupRunner) { r.unknownPolicy = true }},
		{"unknown uppers", func(f *deferredFixture, r *deferredCleanupRunner) {
			f.ops.upperFree = func() error { return os.ErrPermission }
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := setupDeferred(t)
			f.begin(t)
			r := &deferredCleanupRunner{Runner: f.kernel}
			f.adapter.Runner = r
			f.kernel.parent.MAC = "8e:00:00:00:00:01"
			f.kernel.parent.Up = false
			var auth CanaryAuthorization
			if err := readPrivate(filepath.Join(f.adapter.Store.Dir, "canary-active.json"), &auth); err != nil {
				t.Fatal(err)
			}
			auth.Expired = true
			privateJSON(t, filepath.Join(f.adapter.Store.Dir, "canary-active.json"), auth)
			tc.change(f, r)
			expected := State{Generation: genA, Boot: f.adapter.Boot, Deadline: auth.Deadline}
			if deferredWatchdogCleanup(f.adapter, expected, f.ops) == nil {
				t.Fatal("unsafe empty terminal success")
			}
			if r.parents.Load() != 1 {
				t.Fatal("nonempty/unknown evidence bypassed typed parent admission")
			}
			if len(f.kernel.calls) != 0 {
				t.Fatal("foreign/unknown state mutated")
			}
		})
	}
}

func TestDeferredWatchdogRejectsChangedBindingBeforeAnyCleanup(t *testing.T) {
	for _, which := range []string{"profile", "boot", "generation", "deadline", "claim", "primary", "not expired"} {
		t.Run(which, func(t *testing.T) {
			f := setupDeferred(t)
			f.begin(t)
			r := &deferredCleanupRunner{Runner: f.kernel}
			f.adapter.Runner = r
			var auth CanaryAuthorization
			if err := readPrivate(filepath.Join(f.adapter.Store.Dir, "canary-active.json"), &auth); err != nil {
				t.Fatal(err)
			}
			auth.Expired = true
			expected := State{Generation: genA, Boot: f.adapter.Boot, Deadline: auth.Deadline}
			switch which {
			case "profile":
				auth.Hash = "foreign"
			case "boot":
				auth.Boot = "different-boot"
			case "generation":
				auth.Generation = genB
			case "deadline":
				auth.Deadline++
			case "claim":
				var claim DeferredClaim
				if err := readPrivate(filepath.Join(f.adapter.Store.Dir, deferredClaimFile), &claim); err != nil {
					t.Fatal(err)
				}
				claim.Hash = "foreign"
				privateJSON(t, filepath.Join(f.adapter.Store.Dir, deferredClaimFile), claim)
			case "primary":
				base := setup(t)
				arm(t, base.engine)
				if err := f.adapter.Store.Save(mustState(t, base.engine)); err != nil {
					t.Fatal(err)
				}
			case "not expired":
				auth.Expired = false
			}
			privateJSON(t, filepath.Join(f.adapter.Store.Dir, "canary-active.json"), auth)
			if deferredWatchdogCleanup(f.adapter, expected, f.ops) == nil {
				t.Fatal("changed binding accepted")
			}
			if r.parents.Load() != 0 || len(f.kernel.calls) != 0 {
				t.Fatal("foreign claim reached cleanup")
			}
		})
	}
}

func TestDeferredWatchdogFallsBackToTypedCleanupForOwnedResources(t *testing.T) {
	f := setupDeferred(t)
	f.begin(t)
	r := &deferredCleanupRunner{Runner: f.kernel}
	f.adapter.Runner = r
	if err := f.adapter.Bootstrap(f.adapter.Boot, f.tick); err != nil {
		t.Fatal(err)
	}
	var auth CanaryAuthorization
	if err := readPrivate(filepath.Join(f.adapter.Store.Dir, "canary-active.json"), &auth); err != nil {
		t.Fatal(err)
	}
	auth.Expired = true
	privateJSON(t, filepath.Join(f.adapter.Store.Dir, "canary-active.json"), auth)
	mutations := len(f.kernel.calls)
	parents := r.parents.Load()
	if err := deferredWatchdogCleanup(f.adapter, State{Generation: genA, Boot: f.adapter.Boot, Deadline: auth.Deadline}, f.ops); err != nil {
		t.Fatal(err)
	}
	if len(f.kernel.calls) <= mutations || r.parents.Load() <= parents {
		t.Fatal("nonzero ownership used empty shortcut")
	}
	if f.kernel.link.Exists || f.kernel.ipvlanModule || len(f.kernel.routes) != 0 || len(f.kernel.rules) != 0 {
		t.Fatal("typed fallback did not clean owned resources")
	}
}

func TestDeferredWatchdogEmptyShortcutPreservesHistoricalZeroOwnership(t *testing.T) {
	f := setupDeferred(t)
	f.begin(t)
	historical := Owned{Hash: f.adapter.Profile.Hash(), Boot: "old-boot-zero-intent"}
	privateJSON(t, filepath.Join(f.adapter.Store.Dir, "network-owned.json"), historical)
	before, err := os.ReadFile(filepath.Join(f.adapter.Store.Dir, "network-owned.json"))
	if err != nil {
		t.Fatal(err)
	}
	r := &deferredCleanupRunner{Runner: f.kernel}
	f.adapter.Runner = r
	f.kernel.parent.MAC = "8e:00:00:00:00:01"
	f.kernel.parent.Up = false
	var auth CanaryAuthorization
	if err := readPrivate(filepath.Join(f.adapter.Store.Dir, "canary-active.json"), &auth); err != nil {
		t.Fatal(err)
	}
	auth.Expired = true
	privateJSON(t, filepath.Join(f.adapter.Store.Dir, "canary-active.json"), auth)
	if err := deferredWatchdogCleanup(f.adapter, State{Generation: genA, Boot: f.adapter.Boot, Deadline: auth.Deadline}, f.ops); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(f.adapter.Store.Dir, "network-owned.json"))
	if err != nil || string(before) != string(after) {
		t.Fatal("empty terminal path relabelled old zero intent")
	}
	if r.parents.Load() != 0 || len(f.kernel.calls) != 0 {
		t.Fatal("zero intent shortcut consulted/changed parent")
	}
}
