package recovery

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

const genA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const genB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
const original = `{"network_config":{"mode":"dhcp"},"auth":{"password":"PRIVATE_AUTH_SENTINEL"},"usb":{"enabled":true},"video":{"quality":77},"future":{"arbitrary":[1,2]}}`
const target = `{"mode":"static","ipv4":"198.51.100.10"}`

type fixture struct {
	engine  Engine
	now     *atomic.Int64
	starts  *atomic.Int64
	reboots *atomic.Int64
}

func setup(t *testing.T) fixture {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(config, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	now := new(atomic.Int64)
	now.Store(1_000_000_000)
	starts := new(atomic.Int64)
	reboots := new(atomic.Int64)
	engine := Engine{Store: Store{Dir: dir}, Config: config, Device: "exampledevice0001", Boot: "boot-original-0001", Now: func() (int64, error) { return now.Load(), nil }, Start: func(State) error { starts.Add(1); return nil }, Management: func() error { return nil }, Reboot: func() error { reboots.Add(1); return nil }}
	return fixture{engine, now, starts, reboots}
}
func mustState(t *testing.T, e Engine) State {
	t.Helper()
	state, err := e.Store.Load()
	if err != nil {
		t.Fatal(err)
	}
	return state
}
func setTarget(t *testing.T, e Engine) {
	t.Helper()
	latest, _, err := Merge([]byte(original), nil, []byte(target))
	if err != nil {
		t.Fatal(err)
	}
	if err := Atomic(e.Config, latest); err != nil {
		t.Fatal(err)
	}
}
func arm(t *testing.T, e Engine) {
	t.Helper()
	if err := e.Arm(genA, json.RawMessage(target)); err != nil {
		t.Fatal(err)
	}
}
func trial(t *testing.T, f fixture) Engine {
	t.Helper()
	arm(t, f.engine)
	setTarget(t, f.engine)
	e := f.engine
	e.Boot = "boot-target-00002"
	f.now.Store(2_000_000_000)
	if err := e.PreApp(); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestFirstTargetBootAndSecondBootRestore(t *testing.T) {
	f := setup(t)
	e := trial(t, f)
	state := mustState(t, e)
	if state.Phase != "trial" || state.Allowance {
		t.Fatalf("allowance not durably consumed: %+v", publicState(state))
	}
	deadline := state.Deadline
	f.now.Add(30_000_000_000)
	if err := e.PreApp(); err != nil {
		t.Fatal(err)
	}
	if got := mustState(t, e); got.Deadline != deadline || got.Allowance {
		t.Fatal("same-boot reapply renewed allowance/deadline")
	}
	// Unexpected first target-bearing boot above is allowed; a second never is.
	e.Boot = "boot-subsequent-3"
	f.now.Store(1_000_000_000)
	if err := e.PreApp(); err != nil {
		t.Fatal(err)
	}
	state = mustState(t, e)
	if state.Phase != "restored" {
		t.Fatal("second pending boot not restored")
	}
	data, err := os.ReadFile(e.Config)
	if err != nil {
		t.Fatal(err)
	}
	network, err := Network(data)
	if err != nil {
		t.Fatal(err)
	}
	if string(network) != `{"mode":"dhcp"}` {
		t.Fatal("saved primary profile was not restored")
	}
	if !strings.Contains(string(data), "PRIVATE_AUTH_SENTINEL") {
		t.Fatal("authentication not preserved")
	}
}
func publicState(s State) string { return s.Phase }

func TestPreAppFailureRestoresBeforeReturning(t *testing.T) {
	for _, test := range []string{"worker", "mismatched-target", "deadline", "clock", "missing-config", "invalid-config"} {
		t.Run(test, func(t *testing.T) {
			f := setup(t)
			arm(t, f.engine)
			setTarget(t, f.engine)
			e := f.engine
			e.Boot = "boot-new-00000002"
			switch test {
			case "worker":
				e.Start = func(State) error { return errors.New("not ready") }
			case "mismatched-target":
				if err := Atomic(e.Config, []byte(original)); err != nil {
					t.Fatal(err)
				}
			case "deadline":
				e.Boot = f.engine.Boot
				f.now.Add(WindowNS)
			case "clock":
				e.Now = func() (int64, error) { return 0, errors.New("clock failed") }
			case "missing-config":
				if err := os.Remove(e.Config); err != nil {
					t.Fatal(err)
				}
			case "invalid-config":
				if err := Atomic(e.Config, []byte(`{"auth":"PRIVATE_TRUNCATED`)); err != nil {
					t.Fatal(err)
				}
			}
			if err := e.PreApp(); err != nil {
				t.Fatal(err)
			}
			state := mustState(t, e)
			if state.Phase != "restored" || state.Allowance {
				t.Fatal("unprotected target boot permitted")
			}
			if (test == "missing-config" || test == "invalid-config") && !state.Fallback {
				t.Fatal("whole-config fallback not reported")
			}
		})
	}
}

func TestExpiryWritesJournalButNotRuntimeConfig(t *testing.T) {
	f := setup(t)
	e := trial(t, f)
	before, err := os.ReadFile(e.Config)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Expire(genA); err == nil {
		t.Fatal("early expiry accepted")
	}
	f.now.Store(mustState(t, e).Deadline)
	e.Reboot = func() error { return errors.New("kernel failure") }
	if err := e.Expire(genA); err == nil {
		t.Fatal("failed reboot reported success")
	}
	state := mustState(t, e)
	if state.Phase != "restoring" || state.Allowance {
		t.Fatal("failed reboot lost restoring marker")
	}
	after, err := os.ReadFile(e.Config)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("runtime expiry wrote live app configuration")
	}
	if err := e.Confirm(genA, true, true); err == nil {
		t.Fatal("confirmation canceled restoring")
	}
	e.Boot = "boot-restore-0003"
	if err := e.PreApp(); err != nil {
		t.Fatal(err)
	}
	if mustState(t, e).Phase != "restored" {
		t.Fatal("pre-app restoration failed")
	}
}

func TestConfirmAndExpiryRace(t *testing.T) {
	for range 25 {
		f := setup(t)
		e := trial(t, f)
		deadline := mustState(t, e).Deadline
		// Both commands are real flock/rename/fsync operations. Whichever clock
		// observation wins, a confirmed generation must never subsequently restore.
		f.now.Store(deadline - 1)
		var group sync.WaitGroup
		group.Add(2)
		go func() { defer group.Done(); _ = e.Confirm(genA, true, true) }()
		go func() { defer group.Done(); f.now.Store(deadline); _ = e.Expire(genA) }()
		group.Wait()
		state := mustState(t, e)
		if state.Phase != "confirmed" && state.Phase != "restoring" {
			t.Fatalf("invalid race outcome %s", state.Phase)
		}
		if state.Phase == "confirmed" && f.reboots.Load() != 0 {
			t.Fatal("confirmed state was rebooted")
		}
	}
}

func TestConfirmProofIdentityAndGenerationReuse(t *testing.T) {
	f := setup(t)
	e := trial(t, f)
	for _, attempt := range []struct {
		generation string
		a, b       bool
	}{{genB, true, true}, {genA, false, true}, {genA, true, false}} {
		if err := e.Confirm(attempt.generation, attempt.a, attempt.b); err == nil {
			t.Fatal("invalid proof accepted")
		}
	}
	stale := e
	stale.Device = "wrong-device-001"
	if err := stale.Confirm(genA, true, true); err == nil {
		t.Fatal("wrong device accepted")
	}
	stale = e
	stale.Boot = "boot-stale-000000"
	if err := stale.Expire(genA); err == nil {
		t.Fatal("stale boot accepted")
	}
	if err := e.Confirm(genA, true, true); err != nil {
		t.Fatal(err)
	}
	if err := e.Confirm(genA, true, true); err != nil {
		t.Fatal("confirmation not idempotent")
	}
	if err := e.Arm(genB, json.RawMessage(target)); err != nil {
		t.Fatal(err)
	}
	if err := e.Expire(genA); err == nil {
		t.Fatal("stale worker accepted later generation")
	}
	// Complete B without a reboot, then demonstrate persistent nonce tombstones.
	state := mustState(t, e)
	state.Phase = "restored"
	state.Allowance = false
	if err := e.Store.Save(state); err != nil {
		t.Fatal(err)
	}
	if err := e.Arm(genA, json.RawMessage(target)); err == nil {
		t.Fatal("old nonce reused")
	}
}

func TestArmFailuresDoNotMutateVendorJSON(t *testing.T) {
	for _, test := range []string{"management", "worker", "generation", "malformed-target", "slow-worker"} {
		t.Run(test, func(t *testing.T) {
			f := setup(t)
			e := f.engine
			generation := genA
			network := json.RawMessage(target)
			switch test {
			case "management":
				e.Management = func() error { return errors.New("not accepted") }
			case "worker":
				e.Start = func(State) error { return errors.New("failed") }
			case "generation":
				generation = "not-random"
			case "malformed-target":
				network = []byte(`{"PRIVATE_SENTINEL`)
			case "slow-worker":
				e.Start = func(State) error { f.now.Add(WindowNS); return nil }
			}
			err := e.Arm(generation, network)
			if err == nil {
				t.Fatal("invalid arm succeeded")
			}
			if strings.Contains(err.Error(), "PRIVATE_SENTINEL") {
				t.Fatal("private input in error")
			}
			data, err := os.ReadFile(e.Config)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != original {
				t.Fatal("arm wrote vendor JSON")
			}
		})
	}
}

func TestConsumedAllowanceSurvivesWorkerFailure(t *testing.T) {
	f := setup(t)
	arm(t, f.engine)
	setTarget(t, f.engine)
	e := f.engine
	e.Boot = "boot-new-0000002"
	e.Start = func(observed State) error {
		durable := mustState(t, e)
		if durable.Allowance || durable.Phase != "trial" || durable.Deadline != observed.Deadline {
			t.Fatal("worker started before durable allowance consumption")
		}
		return errors.New("startup failure")
	}
	if err := e.PreApp(); err != nil {
		t.Fatal(err)
	}
	if mustState(t, e).Phase != "restored" {
		t.Fatal("worker failure failed to restore synchronously")
	}
}

func TestSlowConfirmationCannotCancelElapsedDeadline(t *testing.T) {
	f := setup(t)
	e := trial(t, f)
	deadline := mustState(t, e).Deadline
	f.now.Store(deadline - 1)
	e.Management = func() error { f.now.Store(deadline); return nil }
	if err := e.Confirm(genA, true, true); err == nil {
		t.Fatal("verification that crossed deadline canceled rollback")
	}
	if mustState(t, e).Phase != "trial" {
		t.Fatal("late proof changed transaction")
	}
}
func TestClockFailureAfterReadinessRequestsFailSafeReboot(t *testing.T) {
	f := setup(t)
	e := trial(t, f)
	e.Now = func() (int64, error) { return 0, errors.New("clock failed") }
	if err := e.Expire(genA); err != nil {
		t.Fatal(err)
	}
	if f.reboots.Load() != 1 || mustState(t, e).Phase != "restoring" {
		t.Fatal("clock failure did not enter fail-safe restoration")
	}
}

func TestPublicStatusNeverSerializesPrivateConfig(t *testing.T) {
	f := setup(t)
	arm(t, f.engine)
	status, err := f.engine.Status(genA)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"PRIVATE_AUTH_SENTINEL", "198.51.100.10", "network_config", "saved_network", "baseline"} {
		if strings.Contains(string(data), private) {
			t.Fatal("private configuration appeared in status")
		}
	}
	if status.Phase != "armed" {
		t.Fatal("status missing public phase")
	}
	if _, err := f.engine.Status(genB); err == nil {
		t.Fatal("stale status generation accepted")
	}
}
