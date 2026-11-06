//go:build linux

package recovery

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"syscall"
	"testing"
	"time"
)

type deferredFixture struct {
	adapter          Adapter
	kernel           *kernel
	ops              deferredOps
	tick             int64
	live             map[int]bool
	watchdog, waiter WorkerIdentity
	starts           []string
	snapshot         ParentReadinessSnapshot
}

func setupDeferred(t *testing.T) *deferredFixture {
	t.Helper()
	a, k := ipvlanAdapter(t)
	a.Boot = "boot-current-0001"
	f := &deferredFixture{adapter: a, kernel: k, tick: int64(time.Second), live: map[int]bool{500: true, 501: true}}
	f.adapter.Now = func() (int64, error) { return f.tick, nil }
	f.snapshot = readinessFixtureSnapshot(t)
	f.snapshot.Boot = a.Boot
	f.snapshot.Parent.MAC = a.Profile.MAC
	f.ops = deferredOps{now: f.adapter.Now, collect: func(context.Context) (ParentReadinessSnapshot, error) { return f.snapshot, nil }, alive: func(w WorkerIdentity) error {
		if !f.live[w.PID] {
			return errors.New("injected dead process")
		}
		return nil
	}, upperFree: func() error { return nil }, pause: func(context.Context) error { f.tick += int64(time.Second); return nil }}
	auth := CanaryAuthorization{Hash: a.Profile.Hash(), Generation: genA, Boot: "boot-prior-0001", Deadline: WindowNS, BootAllowance: true}
	privateJSON(t, filepath.Join(a.Store.Dir, "canary-active.json"), auth)
	privateJSON(t, filepath.Join(a.Store.Dir, "vendor-config.json"), map[string]string{"mode": "untouched"})
	return f
}
func (f *deferredFixture) watch(t *testing.T) func(State) error {
	return func(state State) error {
		t.Helper()
		f.starts = append(f.starts, "watchdog")
		var auth CanaryAuthorization
		var claim DeferredClaim
		if err := readPrivate(filepath.Join(f.adapter.Store.Dir, "canary-active.json"), &auth); err != nil {
			t.Fatal(err)
		}
		if err := readPrivate(filepath.Join(f.adapter.Store.Dir, deferredClaimFile), &claim); err != nil {
			t.Fatal(err)
		}
		if auth.BootAllowance || auth.Boot != f.adapter.Boot || auth.Deadline != f.tick+WindowNS || claim.Attempt != "watchdog-attempted" || state.Deadline != auth.Deadline || len(f.kernel.calls) != 0 {
			t.Fatal("watchdog did not precede mutation with durable claim")
		}
		f.watchdog = WorkerIdentity{Generation: state.Generation, Boot: state.Boot, Deadline: state.Deadline, PID: 500, Birth: "100"}
		return deferredSave(f.adapter.Store, "worker.json", f.watchdog)
	}
}
func (f *deferredFixture) wait(t *testing.T) func(State) error {
	return func(state State) error {
		t.Helper()
		f.starts = append(f.starts, "waiter")
		if err := deferredIdentity(f.adapter.Store, "worker.json", state, f.ops.alive); err != nil {
			t.Fatal(err)
		}
		if len(f.kernel.calls) != 0 {
			t.Fatal("mutation before detached waiter")
		}
		f.waiter = WorkerIdentity{Generation: state.Generation, Boot: state.Boot, Deadline: state.Deadline, PID: 501, Birth: "101"}
		return deferredSave(f.adapter.Store, deferredWaiterFile, f.waiter)
	}
}
func (f *deferredFixture) begin(t *testing.T) {
	t.Helper()
	if err := beginDeferredManagement(context.Background(), f.adapter, f.ops, f.watch(t), f.wait(t)); err != nil {
		t.Fatal(err)
	}
}

func TestDeferredClaimWatchdogUnlockedWaitAndImmutableReapply(t *testing.T) {
	f := setupDeferred(t)
	f.begin(t)
	if !reflect.DeepEqual(f.starts, []string{"watchdog", "waiter"}) {
		t.Fatal("wrong launch order")
	}
	activeBefore, err := os.ReadFile(filepath.Join(f.adapter.Store.Dir, "canary-active.json"))
	if err != nil {
		t.Fatal(err)
	}
	claimBefore, err := os.ReadFile(filepath.Join(f.adapter.Store.Dir, deferredClaimFile))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	f.ops.collect = func(context.Context) (ParentReadinessSnapshot, error) {
		count++
		if count <= 2 {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			if err := deferredLocked(ctx, f.adapter.Store, func() error { return nil }); err != nil {
				t.Fatal("readiness waiter held Store lock")
			}
		}
		if len(f.kernel.calls) != 0 {
			t.Fatal("mutation before final readiness admission")
		}
		return f.snapshot, nil
	}
	if err := continueDeferredManagement(context.Background(), f.adapter, f.ops, f.waiter); err != nil {
		t.Fatal(err)
	}
	if count != 3 || !f.kernel.link.Exists || !f.kernel.ipvlanModule {
		t.Fatal("stable pair/final admission did not ensure")
	}
	var result deferredResult
	if err := readPrivate(filepath.Join(f.adapter.Store.Dir, deferredResultFile), &result); err != nil || result.Outcome != "ensured" {
		t.Fatal("success receipt missing")
	}
	mutations := len(f.kernel.calls)
	f.tick += int64(time.Second)
	forbid := func(State) error { t.Fatal("reapply launched another process"); return nil }
	if err := beginDeferredManagement(context.Background(), f.adapter, f.ops, forbid, forbid); err != nil {
		t.Fatal(err)
	}
	activeAfter, _ := os.ReadFile(filepath.Join(f.adapter.Store.Dir, "canary-active.json"))
	claimAfter, _ := os.ReadFile(filepath.Join(f.adapter.Store.Dir, deferredClaimFile))
	if string(activeBefore) != string(activeAfter) || string(claimBefore) != string(claimAfter) || len(f.kernel.calls) != mutations {
		t.Fatal("reapply reset deadline/claim or mutated")
	}
	// A second actual boot cannot reuse the consumed allowance, even if the
	// kernel has discarded all volatile resources.
	f.adapter.Boot = "boot-second-0001"
	f.kernel.link = Link{}
	f.kernel.ipvlanModule = false
	f.kernel.routes = nil
	f.kernel.rules = nil
	if beginDeferredManagement(context.Background(), f.adapter, f.ops, forbid, forbid) == nil {
		t.Fatal("consumed allowance reused")
	}
	var afterSecond CanaryAuthorization
	if err := readPrivate(filepath.Join(f.adapter.Store.Dir, "canary-active.json"), &afterSecond); err != nil {
		t.Fatal(err)
	}
	if !afterSecond.Expired || afterSecond.BootAllowance || afterSecond.Boot != "boot-current-0001" || afterSecond.Deadline != f.watchdog.Deadline || len(f.kernel.calls) != mutations {
		t.Fatal("second boot failed terminal marker or minted resources/deadline")
	}
}

func TestDeferredReadinessAllowsEarlyWrongMACDownThenVendorCompletion(t *testing.T) {
	f := setupDeferred(t)
	f.begin(t)
	calls := 0
	f.ops.collect = func(context.Context) (ParentReadinessSnapshot, error) {
		calls++
		s := f.snapshot
		if calls == 1 {
			s.Parent.MAC = "8e:00:00:00:00:01"
			s.Parent.Up = false
		}
		return s, nil
	}
	if err := continueDeferredManagement(context.Background(), f.adapter, f.ops, f.waiter); err != nil {
		t.Fatal(err)
	}
	if calls != 4 || !f.kernel.link.Exists {
		t.Fatal("did not wait for actual vendor completion")
	}
}

func TestDeferredReadinessTimeoutLeavesIndependentExpiryAndVendorUntouched(t *testing.T) {
	f := setupDeferred(t)
	f.begin(t)
	vendor, err := os.ReadFile(filepath.Join(f.adapter.Store.Dir, "vendor-config.json"))
	if err != nil {
		t.Fatal(err)
	}
	f.ops.collect = func(context.Context) (ParentReadinessSnapshot, error) {
		s := f.snapshot
		s.Parent.Up = false
		return s, nil
	}
	if continueDeferredManagement(context.Background(), f.adapter, f.ops, f.waiter) == nil {
		t.Fatal("DOWN parent accepted")
	}
	if f.tick != int64(time.Second)+deferredWaitNS || len(f.kernel.calls) != 0 {
		t.Fatal("readiness timeout extended or mutated")
	}
	var auth CanaryAuthorization
	if err := readPrivate(filepath.Join(f.adapter.Store.Dir, "canary-active.json"), &auth); err != nil {
		t.Fatal(err)
	}
	if auth.Deadline != int64(time.Second)+WindowNS || auth.Expired || auth.Confirmed {
		t.Fatal("waiter cancelled/renewed pending watchdog")
	}
	if err := deferredIdentity(f.adapter.Store, "worker.json", State{Generation: genA, Boot: f.adapter.Boot, Deadline: auth.Deadline}, f.ops.alive); err != nil {
		t.Fatal("watchdog lost after waiter timeout")
	}
	var result deferredResult
	if err := readPrivate(filepath.Join(f.adapter.Store.Dir, deferredResultFile), &result); err != nil || result.Outcome != "failed" {
		t.Fatal("failure evidence missing")
	}
	after, err := os.ReadFile(filepath.Join(f.adapter.Store.Dir, "vendor-config.json"))
	if err != nil || string(vendor) != string(after) {
		t.Fatal("vendor file changed")
	}
}

func TestDeferredPartialLaunchCannotMintDeadlineOrRetryUnprovenWorker(t *testing.T) {
	for _, failure := range []string{"watchdog error", "watchdog missing receipt", "watchdog dead", "waiter error"} {
		t.Run(failure, func(t *testing.T) {
			f := setupDeferred(t)
			watch, wait := f.watch(t), f.wait(t)
			switch failure {
			case "watchdog error":
				watch = func(State) error { return errors.New("launch failure") }
			case "watchdog missing receipt":
				watch = func(State) error { return nil }
			case "watchdog dead":
				f.live[500] = false
			case "waiter error":
				wait = func(State) error { return errors.New("launch failure") }
			}
			if beginDeferredManagement(context.Background(), f.adapter, f.ops, watch, wait) == nil {
				t.Fatal("partial setup accepted")
			}
			before, err := os.ReadFile(filepath.Join(f.adapter.Store.Dir, "canary-active.json"))
			if err != nil {
				t.Fatal(err)
			}
			if len(f.kernel.calls) != 0 {
				t.Fatal("partial launch mutated kernel")
			}
			forbidden := func(State) error { t.Fatal("unproven launch retried"); return nil }
			if beginDeferredManagement(context.Background(), f.adapter, f.ops, forbidden, forbidden) == nil {
				t.Fatal("partial setup reapply accepted")
			}
			after, _ := os.ReadFile(filepath.Join(f.adapter.Store.Dir, "canary-active.json"))
			if string(before) != string(after) {
				t.Fatal("failure reset deadline")
			}
		})
	}
}

func TestDeferredAdmissionRefusesForeignResourcesAndMetadata(t *testing.T) {
	cases := []struct {
		name   string
		change func(*deferredFixture)
	}{
		{"IPvlan loaded globally", func(f *deferredFixture) { f.kernel.ipvlanModule = true }},
		{"macvlan loaded globally", func(f *deferredFixture) { f.kernel.module = true }},
		{"foreign child", func(f *deferredFixture) { f.kernel.link = Link{Exists: true} }},
		{"foreign table", func(f *deferredFixture) { f.kernel.routes = []string{"foreign"} }},
		{"foreign rule", func(f *deferredFixture) { f.kernel.rules = []string{"1201: foreign"} }},
		{"parent upper", func(f *deferredFixture) { f.ops.upperFree = func() error { return errors.New("foreign upper") } }},
		{"stale OLDboot ownership", func(f *deferredFixture) {
			privateJSON(t, filepath.Join(f.adapter.Store.Dir, "network-owned.json"), Owned{Hash: f.adapter.Profile.Hash(), Boot: "old-boot", Link: true, Module: true, Index: 17})
		}},
		{"foreign ownership hash", func(f *deferredFixture) {
			privateJSON(t, filepath.Join(f.adapter.Store.Dir, "network-owned.json"), Owned{Hash: "foreign"})
		}},
		{"foreign continuation metadata", func(f *deferredFixture) {
			privateJSON(t, filepath.Join(f.adapter.Store.Dir, deferredWaiterFile), WorkerIdentity{Generation: genB, PID: 700, Birth: "123"})
		}},
		{"foreign prior watchdog", func(f *deferredFixture) {
			privateJSON(t, filepath.Join(f.adapter.Store.Dir, "worker.json"), WorkerIdentity{Generation: genB, PID: 700, Birth: "123"})
		}},
		{"legacy MAC profile", func(f *deferredFixture) { f.adapter.Profile.Kind = "macvlan" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := setupDeferred(t)
			tc.change(f)
			before, _ := os.ReadFile(filepath.Join(f.adapter.Store.Dir, "canary-active.json"))
			forbid := func(State) error { t.Fatal("foreign admission launched worker"); return nil }
			if beginDeferredManagement(context.Background(), f.adapter, f.ops, forbid, forbid) == nil {
				t.Fatal("foreign state admitted")
			}
			after, _ := os.ReadFile(filepath.Join(f.adapter.Store.Dir, "canary-active.json"))
			if string(before) != string(after) || len(f.kernel.calls) != 0 {
				t.Fatal("foreign admission changed authorization or kernel")
			}
		})
	}
}

func TestDeferredFinalAdmissionRechecksWatchdogPrimaryOwnerAndPolicy(t *testing.T) {
	cases := []struct {
		name   string
		change func(*deferredFixture)
	}{
		{"watchdog PID reused", func(f *deferredFixture) { f.live[500] = false }},
		{"foreign waiter", func(f *deferredFixture) {
			w := f.waiter
			w.PID = 700
			privateJSON(t, filepath.Join(f.adapter.Store.Dir, deferredWaiterFile), w)
		}},
		{"deadline reached", func(f *deferredFixture) { f.tick += WindowNS }},
		{"primary became pending", func(f *deferredFixture) {
			base := setup(t)
			arm(t, base.engine)
			if err := f.adapter.Store.Save(mustState(t, base.engine)); err != nil {
				t.Fatal(err)
			}
		}},
		{"foreign policy appeared", func(f *deferredFixture) { f.kernel.rules = []string{"1201: foreign"} }},
		{"owner changed at final snapshot", func(f *deferredFixture) { f.snapshot.End.Owner.Birth++ }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := setupDeferred(t)
			f.begin(t)
			count := 0
			f.ops.collect = func(context.Context) (ParentReadinessSnapshot, error) {
				count++
				s := f.snapshot
				if count == 2 {
					tc.change(f)
				}
				return s, nil
			}
			if continueDeferredManagement(context.Background(), f.adapter, f.ops, f.waiter) == nil {
				t.Fatal("admission race accepted")
			}
			if len(f.kernel.calls) != 0 {
				t.Fatal("admission race reached Ensure")
			}
		})
	}
}

func TestDeferredPrimaryGateAndBoundedStableLock(t *testing.T) {
	f := setupDeferred(t)
	base := setup(t)
	arm(t, base.engine)
	if err := f.adapter.Store.Save(mustState(t, base.engine)); err != nil {
		t.Fatal(err)
	}
	forbid := func(State) error { t.Fatal("primary pending launched additive worker"); return nil }
	if beginDeferredManagement(context.Background(), f.adapter, f.ops, forbid, forbid) == nil {
		t.Fatal("pending primary admitted")
	}
	if err := os.Remove(filepath.Join(f.adapter.Store.Dir, "state.json")); err != nil {
		t.Fatal(err)
	}
	if err := f.adapter.Store.Locked(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		returnErr := beginDeferredManagement(ctx, f.adapter, f.ops, forbid, forbid)
		if !errors.Is(returnErr, context.DeadlineExceeded) {
			t.Fatal("early entry waited indefinitely for Store lock")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestDeferredWaiterHandshakePreservesWatchdogAndReapsFailures(t *testing.T) {
	for _, mode := range []string{"ready", "wrong", "closed", "silent"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("JKRECOVER_SUBPROCESS", mode)
			f := setup(t)
			arm(t, f.engine)
			state := mustState(t, f.engine)
			watchdog := WorkerIdentity{Generation: genA, Boot: state.Boot, Deadline: state.Deadline, PID: 900, Birth: "123"}
			privateJSON(t, filepath.Join(f.engine.Store.Dir, "worker.json"), watchdog)
			before, err := os.ReadFile(filepath.Join(f.engine.Store.Dir, "worker.json"))
			if err != nil {
				t.Fatal(err)
			}
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			err = LaunchManagementWaiter(executable, f.engine.Store.Dir, f.engine.Config, f.engine.Device, "profile", state)
			if (err == nil) != (mode == "ready") {
				t.Fatal("incorrect waiter handshake admission")
			}
			after, err := os.ReadFile(filepath.Join(f.engine.Store.Dir, "worker.json"))
			if err != nil || string(before) != string(after) {
				t.Fatal("waiter overwrote watchdog identity")
			}
			pidText, err := os.ReadFile(filepath.Join(f.engine.Store.Dir, "waiter-test-pid"))
			if err != nil {
				t.Fatal(err)
			}
			pid, err := strconv.Atoi(string(pidText))
			if err != nil {
				t.Fatal(err)
			}
			if mode == "ready" {
				var waiter WorkerIdentity
				if err := readPrivate(filepath.Join(f.engine.Store.Dir, deferredWaiterFile), &waiter); err != nil {
					t.Fatal(err)
				}
				if waiter.PID != pid || waiter.Generation != genA || waiter.Boot != state.Boot || waiter.Deadline != state.Deadline || waiter.PID == os.Getpid() {
					t.Fatal("waiter identity not bound")
				}
				session, _, errno := syscall.Syscall(syscall.SYS_GETSID, uintptr(pid), 0, 0)
				if errno != 0 || int(session) != pid {
					t.Fatal("waiter not detached")
				}
				if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
					t.Fatal(err)
				}
			} else if _, err := os.Lstat(filepath.Join(f.engine.Store.Dir, deferredWaiterFile)); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("failed handshake published waiter identity")
			}
			until := time.Now().Add(time.Second)
			for {
				_, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
				if errors.Is(err, os.ErrNotExist) {
					break
				}
				if time.Now().After(until) {
					t.Fatal("owned waiter child not reaped")
				}
				time.Sleep(time.Millisecond)
			}
		})
	}
}

func TestDeferredCancelledWaiterRetainsWatchdogAndNoResources(t *testing.T) {
	f := setupDeferred(t)
	f.begin(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(continueDeferredManagement(ctx, f.adapter, f.ops, f.waiter), context.Canceled) {
		t.Fatal("cancelled waiter proceeded")
	}
	if len(f.kernel.calls) != 0 || !f.live[500] {
		t.Fatal("cancelled waiter mutated or lost watchdog")
	}
}

func TestDeferredImmediateAdmissionRejectsModuleOrUpperAfterReadiness(t *testing.T) {
	for _, which := range []string{"module", "upper"} {
		t.Run(which, func(t *testing.T) {
			f := setupDeferred(t)
			f.begin(t)
			count := 0
			upperAppeared := false
			f.ops.upperFree = func() error {
				if upperAppeared {
					return errors.New("new foreign upper")
				}
				return nil
			}
			f.ops.collect = func(context.Context) (ParentReadinessSnapshot, error) {
				count++
				if count == 3 {
					if which == "module" {
						f.kernel.ipvlanModule = true
					} else {
						upperAppeared = true
					}
				}
				return f.snapshot, nil
			}
			if continueDeferredManagement(context.Background(), f.adapter, f.ops, f.waiter) == nil {
				t.Fatal("post-readiness foreign resource admitted")
			}
			if len(f.kernel.calls) != 0 {
				t.Fatal("late foreign resource reached mutation")
			}
		})
	}
}

func TestDeferredNativeProcessBirthRefusesReuse(t *testing.T) {
	birth, err := processBirth(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	ops := nativeDeferredOps(Adapter{})
	if err := ops.alive(WorkerIdentity{PID: os.Getpid(), Birth: birth}); err != nil {
		t.Fatal(err)
	}
	if ops.alive(WorkerIdentity{PID: os.Getpid(), Birth: birth + "0"}) == nil {
		t.Fatal("reused/wrong process birth accepted")
	}
	if ops.alive(WorkerIdentity{PID: -1, Birth: "1"}) == nil {
		t.Fatal("missing process accepted")
	}
}

func TestDeferredSecondBootTerminalMarkerDoesNotClaimCleanup(t *testing.T) {
	f := setupDeferred(t)
	f.begin(t)
	f.adapter.Boot = "boot-second-0001"
	historical := Owned{Hash: f.adapter.Profile.Hash(), Boot: "boot-current-0001", Module: true, Link: true, Index: 17, Routes: 2, Rule: true}
	privateJSON(t, filepath.Join(f.adapter.Store.Dir, "network-owned.json"), historical)
	before, err := os.ReadFile(filepath.Join(f.adapter.Store.Dir, "network-owned.json"))
	if err != nil {
		t.Fatal(err)
	}
	f.kernel.link = Link{Exists: true, Index: 99, MAC: "02:00:00:00:00:99"}
	f.kernel.ipvlanModule = true
	f.kernel.rules = []string{"1201: foreign"}
	f.kernel.routes = []string{"foreign"}
	forbid := func(State) error { t.Fatal("second boot launched a process"); return nil }
	if beginDeferredManagement(context.Background(), f.adapter, f.ops, forbid, forbid) == nil {
		t.Fatal("second boot claimed ready")
	}
	var auth CanaryAuthorization
	if err := readPrivate(filepath.Join(f.adapter.Store.Dir, "canary-active.json"), &auth); err != nil {
		t.Fatal(err)
	}
	if !auth.Expired || auth.BootAllowance || auth.Boot != "boot-current-0001" || auth.Deadline != f.watchdog.Deadline {
		t.Fatal("terminal marker changed historical identity/deadline")
	}
	after, err := os.ReadFile(filepath.Join(f.adapter.Store.Dir, "network-owned.json"))
	if err != nil || string(before) != string(after) {
		t.Fatal("marker rewrote old ownership")
	}
	if len(f.kernel.calls) != 0 || !f.kernel.link.Exists || !f.kernel.ipvlanModule || len(f.kernel.rules) != 1 || len(f.kernel.routes) != 1 {
		t.Fatal("terminal marker performed or claimed cleanup")
	}
}
