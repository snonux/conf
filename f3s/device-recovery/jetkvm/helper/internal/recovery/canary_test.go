package recovery

import (
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type delayedProbe struct {
	Runner
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (p *delayedProbe) Link(name string) (Link, error) {
	p.once.Do(func() { close(p.entered); <-p.release })
	return p.Runner.Link(name)
}

func TestKeepCanaryDeadlineAfterLockWait(t *testing.T) {
	a, k := setupAdapter(t)
	var now atomic.Int64
	now.Store(1_000_000_000)
	deadline := now.Load() + WindowNS
	reads := make(chan struct{}, 2)
	a.Now = func() (int64, error) {
		select {
		case reads <- struct{}{}:
		default:
		}
		return now.Load(), nil
	}
	prepareKeepCanary(t, a, deadline)
	held, release := make(chan struct{}), make(chan struct{})
	holder := make(chan error, 1)
	go func() { holder <- a.Store.Locked(func() error { close(held); <-release; return nil }) }()
	<-held
	result := make(chan error, 1)
	go func() { result <- KeepCanary(a, a.Boot, true) }()
	// This is a real flock wait: no clock admission read can happen while the
	// separate open lock inode is held by the other goroutine.
	select {
	case <-reads:
		close(release)
		t.Fatal("confirmation clock sampled before acquiring lock")
	case <-time.After(25 * time.Millisecond):
	}
	now.Store(deadline)
	close(release)
	if err := <-holder; err != nil {
		t.Fatal(err)
	}
	if err := <-result; err == nil {
		t.Fatal("lock-delayed confirmation canceled elapsed cleanup")
	}
	assertUnconfirmedAndCleanable(t, a, k)
}

func TestKeepCanaryDeadlineAfterSlowProbe(t *testing.T) {
	a, k := setupAdapter(t)
	var now atomic.Int64
	now.Store(1_000_000_000)
	deadline := now.Load() + WindowNS
	a.Now = func() (int64, error) { return now.Load(), nil }
	prepareKeepCanary(t, a, deadline)
	probe := &delayedProbe{Runner: a.Runner, entered: make(chan struct{}), release: make(chan struct{})}
	a.Runner = probe
	result := make(chan error, 1)
	go func() { result <- KeepCanary(a, a.Boot, true) }()
	<-probe.entered
	now.Store(deadline)
	close(probe.release)
	if err := <-result; err == nil {
		t.Fatal("probe-delayed confirmation canceled elapsed cleanup")
	}
	assertUnconfirmedAndCleanable(t, a, k)
}

func TestKeepCanaryClockFailureLeavesCleanupAvailable(t *testing.T) {
	for _, failAt := range []int{1, 2} {
		t.Run(string(rune('0'+failAt)), func(t *testing.T) {
			a, k := setupAdapter(t)
			prepareKeepCanary(t, a, 1_000_000_000+WindowNS)
			reads := 0
			a.Now = func() (int64, error) {
				reads++
				if reads == failAt {
					return 0, errors.New("clock unavailable")
				}
				return 1_000_000_000, nil
			}
			if err := KeepCanary(a, a.Boot, true); err == nil {
				t.Fatal("failed confirmation clock accepted")
			}
			assertUnconfirmedAndCleanable(t, a, k)
		})
	}
	a, k := setupAdapter(t)
	prepareKeepCanary(t, a, 1_000_000_000+WindowNS)
	a.Now = nil
	if err := KeepCanary(a, a.Boot, true); err == nil {
		t.Fatal("missing confirmation clock accepted")
	}
	assertUnconfirmedAndCleanable(t, a, k)
}

func prepareKeepCanary(t *testing.T, a Adapter, deadline int64) {
	t.Helper()
	auth := CanaryAuthorization{Hash: a.Profile.Hash(), Generation: genA, Boot: a.Boot, Deadline: deadline, BootAllowance: true}
	privateJSON(t, filepath.Join(a.Store.Dir, "canary-active.json"), auth)
	if err := a.Bootstrap(a.Boot, deadline-WindowNS); err != nil {
		t.Fatal(err)
	}
}

func assertUnconfirmedAndCleanable(t *testing.T, a Adapter, k *kernel) {
	t.Helper()
	var auth CanaryAuthorization
	if err := readPrivate(filepath.Join(a.Store.Dir, "canary-active.json"), &auth); err != nil {
		t.Fatal(err)
	}
	if auth.Confirmed || !auth.BootAllowance {
		t.Fatal("rejected confirmation changed durable canary authorization")
	}
	if err := a.Store.Locked(a.Cleanup); err != nil {
		t.Fatal(err)
	}
	if k.link.Exists || k.module || len(k.routes) > 0 || len(k.rules) > 0 {
		t.Fatal("rejected confirmation stranded cleanup")
	}
}
