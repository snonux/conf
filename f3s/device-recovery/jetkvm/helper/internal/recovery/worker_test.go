package recovery

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("JKRECOVER_SUBPROCESS") != "" && len(os.Args) > 1 && os.Args[1] == "-state" {
		flags := flag.NewFlagSet("worker-test", flag.ContinueOnError)
		dir := flags.String("state", "", "")
		_ = flags.String("config", "", "")
		_ = flags.String("device", "", "")
		_ = flags.String("profile", "", "")
		if err := flags.Parse(os.Args[1:]); err != nil {
			os.Exit(2)
		}
		if flags.Arg(0) == "management-waiter" {
			if err := os.WriteFile(filepath.Join(*dir, "waiter-test-pid"), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
				os.Exit(6)
			}
		}
		state, err := (Store{Dir: *dir}).Load()
		if err != nil {
			os.Exit(3)
		}
		ready := os.NewFile(3, "readiness")
		mode := os.Getenv("JKRECOVER_SUBPROCESS")
		if mode == "silent" {
			time.Sleep(10 * time.Second)
			os.Exit(0)
		}
		if mode == "closed" {
			_ = ready.Close()
			os.Exit(0)
		}
		birth, err := processBirth(os.Getpid())
		if err != nil {
			os.Exit(4)
		}
		identity := WorkerIdentity{state.Generation, state.Boot, state.Deadline, os.Getpid(), birth}
		if mode == "wrong" {
			identity.Generation = genB
		}
		if err := json.NewEncoder(ready).Encode(identity); err != nil {
			os.Exit(5)
		}
		_ = ready.Close()
		time.Sleep(10 * time.Second)
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func TestDetachedWorkerHandshake(t *testing.T) {
	for _, mode := range []string{"ready", "wrong", "closed", "silent"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("JKRECOVER_SUBPROCESS", mode)
			f := setup(t)
			arm(t, f.engine)
			state := mustState(t, f.engine)
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			err = Launch(executable, f.engine.Store.Dir, f.engine.Config, f.engine.Device, "profile", state)
			if mode != "ready" {
				if err == nil {
					t.Fatal("lost or stale worker accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var worker WorkerIdentity
			if err := readPrivate(filepath.Join(f.engine.Store.Dir, "worker.json"), &worker); err != nil {
				t.Fatal(err)
			}
			if worker.Deadline != state.Deadline || worker.Boot != state.Boot || worker.Generation != state.Generation || worker.PID == os.Getpid() {
				t.Fatal("worker identity not bound to durable state")
			}
			session, _, errno := syscall.Syscall(syscall.SYS_GETSID, uintptr(worker.PID), 0, 0)
			if errno != 0 || int(session) != worker.PID {
				t.Fatal("worker not in detached session")
			}
			if err := syscall.Kill(worker.PID, syscall.SIGKILL); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestActualWorkerReadinessExpiryAndTerminalExit(t *testing.T) {
	f := setup(t)
	e := trial(t, f)
	state := mustState(t, e)
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = read.Close() }()
	finished := make(chan error, 1)
	go func() { finished <- Work(e, genA, write) }()
	var worker WorkerIdentity
	if err := json.NewDecoder(read).Decode(&worker); err != nil {
		t.Fatal(err)
	}
	if worker.Generation != genA || worker.Boot != e.Boot || worker.Deadline != state.Deadline {
		t.Fatal("worker readiness mismatch")
	}
	f.now.Store(state.Deadline)
	until := time.Now().Add(5 * time.Second)
	for f.reboots.Load() == 0 && time.Now().Before(until) {
		time.Sleep(10 * time.Millisecond)
	}
	if f.reboots.Load() == 0 {
		t.Fatal("actual worker did not expire")
	}
	if mustState(t, e).Phase != "restoring" {
		t.Fatal("worker failed to mark restoration")
	}
	if err := e.Store.Locked(func() error { s := mustState(t, e); s.Phase = "restored"; return e.Store.Save(s) }); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("terminal generation worker stayed active")
	}
}
func TestClockAndWorkerRejectStaleBootDeadline(t *testing.T) {
	first, err := BootTime()
	if err != nil {
		t.Fatal(err)
	}
	second, err := BootTime()
	if err != nil || second < first {
		t.Fatal("BOOTTIME not monotonic")
	}
	boot, err := BootID()
	if err != nil || len(boot) != 36 {
		t.Fatal("kernel boot identity unavailable")
	}
	for _, test := range []string{"boot", "deadline", "generation", "clock"} {
		t.Run(test, func(t *testing.T) {
			f := setup(t)
			arm(t, f.engine)
			e := f.engine
			generation := genA
			switch test {
			case "boot":
				e.Boot = "wrong-boot-000001"
			case "deadline":
				f.now.Add(WindowNS)
			case "generation":
				generation = genB
			case "clock":
				e.Now = func() (int64, error) { return 0, errors.New("clock unavailable") }
			}
			read, write, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = read.Close(); _ = write.Close() }()
			if err := Work(e, generation, write); err == nil {
				t.Fatal("unguarded worker published readiness")
			}
		})
	}
	if _, err := processBirth(-1); err == nil {
		t.Fatal("nonexistent PID accepted")
	}
	if _, err := linkKind([]byte{1, 0, 1, 0}); err == nil {
		t.Fatal("invalid netlink length accepted")
	}
	data := []byte{12, 0, 1, 0, 'm', 'a', 'c', 'v', 'l', 'a', 'n', 0}
	if kind, err := linkKind(data); err != nil || kind != "macvlan" {
		t.Fatal("valid netlink kind rejected")
	}
}

func TestSerialVerbatimIncludingFifteenHexDigits(t *testing.T) {
	for _, test := range []struct{ input, want string }{{"Serial : 46793b0d4c8d45d\n", "46793b0d4c8d45d"}, {"Serial : e10a0883debf7d4a\n", "e10a0883debf7d4a"}, {"Serial : 0\n", "0"}, {"Serial : xyz\n", ""}, {"Serial : 10000000000000000\n", ""}, {"Serial : \n", ""}, {"Model : example\n", ""}} {
		got, err := serial([]byte(test.input))
		if test.want == "" {
			if err == nil {
				t.Fatal("invalid hardware serial accepted")
			}
		} else if err != nil || got != test.want {
			t.Fatalf("hardware serial changed: %q", got)
		}
	}
}

func TestActualCanaryWorkerExpiryCleansOwnedState(t *testing.T) {
	a, k := setupAdapter(t)
	var now atomic.Int64
	now.Store(1_000_000_000)
	clock := func() (int64, error) { return now.Load(), nil }
	auth := CanaryAuthorization{Hash: a.Profile.Hash(), Generation: genA, Boot: a.Boot, Deadline: now.Load() + WindowNS}
	privateJSON(t, filepath.Join(a.Store.Dir, "canary-active.json"), auth)
	if err := a.Bootstrap(a.Boot, now.Load()); err != nil {
		t.Fatal(err)
	}
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = read.Close() }()
	finished := make(chan error, 1)
	go func() { finished <- WorkCanary(a, a.Boot, genA, clock, write) }()
	var ready WorkerIdentity
	if err := json.NewDecoder(read).Decode(&ready); err != nil {
		t.Fatal(err)
	}
	if ready.Deadline != auth.Deadline {
		t.Fatal("canary worker changed persisted deadline")
	}
	now.Store(auth.Deadline)
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("canary cleanup did not execute")
	}
	if k.link.Exists || k.module || len(k.routes) > 0 || len(k.rules) > 0 {
		t.Fatal("independent canary worker left owned resources")
	}
	var durable CanaryAuthorization
	if err := readPrivate(filepath.Join(a.Store.Dir, "canary-active.json"), &durable); err != nil {
		t.Fatal(err)
	}
	if !durable.Expired || durable.BootAllowance {
		t.Fatal("canary expiry not persisted")
	}
}
