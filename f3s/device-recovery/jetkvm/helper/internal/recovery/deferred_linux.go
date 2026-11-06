//go:build linux

package recovery

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

const deferredWaitNS = int64(30 * time.Second)
const deferredClaimFile = "management-deferred.json"
const deferredWaiterFile = "management-waiter.json"
const deferredResultFile = "management-deferred-result.json"

// DeferredClaim is a separate management-only claim, never a primary receipt.
// Attempt markers are durable before spawning, so a crash cannot authorize a
// second untracked process or a fresh deadline on the same boot.
type DeferredClaim struct {
	Hash       string `json:"profile_hash"`
	Generation string `json:"generation"`
	Boot       string `json:"boot"`
	Deadline   int64  `json:"deadline_boottime_ns"`
	WaitUntil  int64  `json:"wait_until_boottime_ns"`
	Attempt    string `json:"attempt"`
}

type deferredResult struct {
	Claim   DeferredClaim `json:"claim"`
	Outcome string        `json:"outcome"`
}

// deferredOps is only the read-only observation boundary of this boot role.
// Process launches remain explicit parameters and all network mutation remains
// in the existing Adapter behind the stable Store lock.
type deferredOps struct {
	now       func() (int64, error)
	collect   func(context.Context) (ParentReadinessSnapshot, error)
	alive     func(WorkerIdentity) error
	upperFree func() error
	pause     func(context.Context) error
}

// BeginDeferredManagement consumes one explicit new-boot allowance and starts
// the independent canary watchdog before a detached readiness continuation.
// A failure is returned to the operator; the proposed wrapper is fail-open to
// vendor startup. This function never calls Ensure or changes the parent.
func BeginDeferredManagement(ctx context.Context, a Adapter, startWatchdog, startWaiter func(State) error) error {
	return beginDeferredManagement(ctx, a, nativeDeferredOps(a), startWatchdog, startWaiter)
}

func beginDeferredManagement(ctx context.Context, a Adapter, ops deferredOps, startWatchdog, startWaiter func(State) error) error {
	if ops.now == nil || startWatchdog == nil || startWaiter == nil {
		return errors.New("deferred clock and launchers required")
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	return deferredLocked(ctx, a.Store, func() error {
		if err := deferredPrimaryAbsent(a.Store); err != nil {
			return err
		}
		if a.Profile.kind() != "ipvlan" || a.Boot == "" {
			return errors.New("deferred role requires pinned IPvlan factory-MAC profile")
		}
		if err := a.Profile.Validate(); err != nil {
			return err
		}
		var auth CanaryAuthorization
		if err := readPrivate(filepath.Join(a.Store.Dir, "canary-active.json"), &auth); err != nil {
			return err
		}
		tick, err := ops.now()
		if err != nil {
			return err
		}
		if auth.Hash != a.Profile.Hash() || !regexp.MustCompile(`^[a-f0-9]{32,64}$`).MatchString(auth.Generation) || auth.Boot == "" || auth.Deadline <= 0 || auth.Confirmed || auth.Expired {
			return errors.New("deferred canary authorization invalid or terminal")
		}
		if auth.Boot == a.Boot {
			claim, err := deferredChecked(a, ops, auth.Generation, false)
			if err != nil {
				return err
			}
			// Reapply is only an observation. It never launches another process or
			// recreates resources, and cannot reset either immutable deadline.
			var result deferredResult
			err = readPrivate(filepath.Join(a.Store.Dir, deferredResultFile), &result)
			if err == nil {
				if result.Claim != claim || result.Outcome != "ensured" {
					return errors.New("deferred continuation already failed")
				}
				return a.Probe()
			}
			if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			current, err := ops.now()
			if err != nil {
				return err
			}
			if current >= claim.WaitUntil {
				return errors.New("deferred waiter exceeded readiness deadline")
			}
			return deferredIdentity(a.Store, deferredWaiterFile, claim.state(), ops.alive)
		}
		if !auth.BootAllowance {
			// A reboot has destroyed the prior watchdog. Make the prior pending
			// canary explicitly terminal without touching an unready vendor parent.
			// Historical boot/deadline and ownership remain evidence, not a claim
			// of cleanup. Root must reconcile them after vendor readiness.
			auth.Expired = true
			if err := saveCanary(a.Store, auth); err != nil {
				return err
			}
			return errors.New("second boot expired canary; owned reconciliation required")
		}
		if tick < 0 || tick > math.MaxInt64-WindowNS {
			return errors.New("deferred boot clock invalid")
		}
		if err := deferredFreshMetadata(a.Store); err != nil {
			return err
		}
		if err := deferredPriorWatchdog(a.Store, auth); err != nil {
			return err
		}
		if err := deferredEmpty(a, ops); err != nil {
			return err
		}
		// Persist consumption first. If a crash interrupts claim/watchdog setup,
		// this boot cannot borrow another allowance or reach Ensure without proof.
		auth.Boot, auth.BootAllowance, auth.Deadline = a.Boot, false, tick+WindowNS
		if err := saveCanary(a.Store, auth); err != nil {
			return err
		}
		claim := DeferredClaim{Hash: auth.Hash, Generation: auth.Generation, Boot: auth.Boot, Deadline: auth.Deadline, WaitUntil: tick + deferredWaitNS, Attempt: "watchdog-attempted"}
		if err := deferredSave(a.Store, deferredClaimFile, claim); err != nil {
			return err
		}
		if err := startWatchdog(claim.state()); err != nil {
			return err
		}
		if err := deferredIdentity(a.Store, "worker.json", claim.state(), ops.alive); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		tick, err = ops.now()
		if err != nil {
			return err
		}
		if tick >= claim.WaitUntil {
			return errors.New("deferred launch exceeded readiness window")
		}
		claim.Attempt = "waiter-attempted"
		if err := deferredSave(a.Store, deferredClaimFile, claim); err != nil {
			return err
		}
		if err := startWaiter(claim.state()); err != nil {
			return err
		}
		return deferredIdentity(a.Store, deferredWaiterFile, claim.state(), ops.alive)
	})
}

func continueDeferredManagement(ctx context.Context, a Adapter, ops deferredOps, self WorkerIdentity) (resultErr error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var initial DeferredClaim
	if err := readPrivate(filepath.Join(a.Store.Dir, deferredClaimFile), &initial); err != nil {
		return err
	}
	if initial.Generation != self.Generation || initial.Boot != self.Boot || initial.Boot != a.Boot || initial.Deadline != self.Deadline || initial.Hash != a.Profile.Hash() || a.Profile.kind() != "ipvlan" || initial.Attempt != "waiter-attempted" || initial.Deadline < WindowNS || initial.WaitUntil != initial.Deadline-WindowNS+deferredWaitNS || self.PID <= 1 || self.Birth == "" {
		return errors.New("deferred continuation identity mismatch")
	}
	tick, err := ops.now()
	if err != nil {
		return err
	}
	if tick < 0 || tick >= initial.WaitUntil {
		return errors.New("deferred readiness deadline reached")
	}
	// Clamp every collector and lock acquisition to the remaining original wait
	// interval, rather than giving a delayed child a new thirty-second window.
	remainingCtx, remainingCancel := context.WithTimeout(ctx, time.Duration(initial.WaitUntil-tick))
	defer remainingCancel()
	ctx = remainingCtx
	defer func() {
		if resultErr != nil {
			// Do not replace another claim or hide the original failure if evidence
			// persistence itself fails. The independent watchdog remains authoritative.
			evidenceCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err := deferredLocked(evidenceCtx, a.Store, func() error {
				var current DeferredClaim
				if err := readPrivate(filepath.Join(a.Store.Dir, deferredClaimFile), &current); err != nil {
					return err
				}
				if current != initial {
					return errors.New("deferred claim changed before failure record")
				}
				return deferredSave(a.Store, deferredResultFile, deferredResult{initial, "failed"})
			})
			if err != nil {
				resultErr = errors.Join(resultErr, errors.New("deferred failure evidence not persisted"))
			}
		}
	}()
	var previous *ParentReadinessSnapshot
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		tick, err := ops.now()
		if err != nil {
			return err
		}
		if tick >= initial.WaitUntil || tick >= initial.Deadline {
			return errors.New("deferred readiness deadline reached")
		}
		// No Store lock is held while collecting or waiting for vendor startup.
		snapshot, collectErr := ops.collect(ctx)
		if collectErr == nil && previous != nil && ValidateParentReadiness(a.Boot, a.Profile.MAC, *previous, snapshot) == nil {
			return deferredLocked(ctx, a.Store, func() error {
				claim, err := deferredChecked(a, ops, self.Generation, true)
				if err != nil {
					return err
				}
				if claim != initial {
					return errors.New("deferred claim changed while waiting")
				}
				var waiter WorkerIdentity
				if err := readPrivate(filepath.Join(a.Store.Dir, deferredWaiterFile), &waiter); err != nil {
					return err
				}
				if waiter != self {
					return errors.New("foreign deferred continuation")
				}
				if err := ops.alive(waiter); err != nil {
					return err
				}
				if err := deferredEmpty(a, ops); err != nil {
					return err
				}
				// One immediate observation under the lock binds the unlocked stable pair
				// to admission. There is no readiness sleep/poll while holding this lock.
				current, err := ops.collect(ctx)
				if err != nil {
					return err
				}
				if err := ValidateParentReadiness(a.Boot, a.Profile.MAC, snapshot, current); err != nil {
					return err
				}
				if err := deferredEmpty(a, ops); err != nil {
					return err
				}
				if _, err := deferredChecked(a, ops, self.Generation, true); err != nil {
					return err
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				tick, err := ops.now()
				if err != nil {
					return err
				}
				if err := a.Bootstrap(a.Boot, tick); err != nil {
					cleanupErr := a.Cleanup()
					if cleanupErr != nil {
						return errors.Join(err, cleanupErr)
					}
					return err
				}
				if _, err := deferredChecked(a, ops, self.Generation, false); err != nil {
					return errors.Join(err, a.Cleanup())
				}
				tick, err = ops.now()
				if err != nil {
					return errors.Join(err, a.Cleanup())
				}
				if tick >= claim.Deadline {
					return expireCanary(a, deferredAuth(claim))
				}
				return deferredSave(a.Store, deferredResultFile, deferredResult{claim, "ensured"})
			})
		}
		if collectErr == nil {
			previous = &snapshot
		} else {
			previous = nil
		}
		if err := ops.pause(ctx); err != nil {
			return err
		}
	}
}

func deferredChecked(a Adapter, ops deferredOps, generation string, waiting bool) (DeferredClaim, error) {
	var claim DeferredClaim
	if err := deferredPrimaryAbsent(a.Store); err != nil {
		return claim, err
	}
	if err := readPrivate(filepath.Join(a.Store.Dir, deferredClaimFile), &claim); err != nil {
		return claim, err
	}
	var auth CanaryAuthorization
	if err := readPrivate(filepath.Join(a.Store.Dir, "canary-active.json"), &auth); err != nil {
		return claim, err
	}
	tick, err := ops.now()
	if err != nil {
		return claim, err
	}
	if claim.Hash != a.Profile.Hash() || claim.Generation != generation || claim.Boot != a.Boot || claim.Deadline != auth.Deadline || claim.Attempt != "waiter-attempted" || auth.Generation != generation || auth.Hash != claim.Hash || auth.Boot != a.Boot || auth.BootAllowance || auth.Confirmed || auth.Expired || tick < 0 || tick >= auth.Deadline || auth.Deadline-tick > WindowNS || claim.WaitUntil <= 0 || claim.WaitUntil > claim.Deadline || claim.WaitUntil != claim.Deadline-WindowNS+deferredWaitNS || (waiting && tick >= claim.WaitUntil) {
		return claim, errors.New("deferred claim expired, changed or not armed")
	}
	if err := deferredIdentity(a.Store, "worker.json", claim.state(), ops.alive); err != nil {
		return claim, err
	}
	return claim, nil
}

func deferredIdentity(store Store, name string, state State, alive func(WorkerIdentity) error) error {
	var identity WorkerIdentity
	if err := readPrivate(filepath.Join(store.Dir, name), &identity); err != nil {
		return err
	}
	if identity.Generation != state.Generation || identity.Boot != state.Boot || identity.Deadline != state.Deadline || identity.PID <= 1 || identity.Birth == "" {
		return errors.New("deferred process claim mismatch")
	}
	return alive(identity)
}

func deferredPrimaryAbsent(store Store) error {
	state, err := store.Load()
	if err == nil && state.Phase != "confirmed" && state.Phase != "restored" {
		return errors.New("primary transaction pending; deferred management forbidden")
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func deferredEmpty(a Adapter, ops deferredOps) error {
	owned, err := a.load()
	if err != nil {
		return err
	}
	if owned.Module || owned.Link || owned.Index != 0 || owned.Routes != 0 || owned.Rule {
		return errors.New("deferred role refuses existing or stale ownership intent")
	}
	for _, kind := range []string{"ipvlan", "macvlan"} {
		loaded, err := a.Runner.Module(kind)
		if err != nil {
			return err
		}
		if loaded {
			return errors.New("deferred role refuses pre-existing management module")
		}
	}
	link, err := a.Runner.Link(a.Profile.Link)
	if err != nil {
		return err
	}
	if link.Exists {
		return errors.New("deferred role refuses pre-existing child")
	}
	routes, rules, err := a.policy()
	if err != nil {
		return err
	}
	if len(routes) != 0 || len(rules) != 0 {
		return errors.New("deferred role refuses occupied policy")
	}
	return ops.upperFree()
}

func deferredSave(store Store, name string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return errors.New("encode deferred metadata")
	}
	return Atomic(filepath.Join(store.Dir, name), data)
}

func deferredAuth(claim DeferredClaim) CanaryAuthorization {
	return CanaryAuthorization{Hash: claim.Hash, Generation: claim.Generation, Boot: claim.Boot, Deadline: claim.Deadline}
}

func nativeDeferredOps(a Adapter) deferredOps {
	return deferredOps{now: a.Now, collect: CollectParentReadiness, alive: func(identity WorkerIdentity) error {
		birth, err := processBirth(identity.PID)
		if err != nil || birth != identity.Birth {
			return errors.New("deferred process no longer live")
		}
		return nil
	}, upperFree: func() error {
		names, err := (nativeReadinessReader{}).names(context.Background(), "/sys/class/net/eth0", 128)
		if err != nil {
			return err
		}
		for _, name := range names {
			if len(name) >= 6 && name[:6] == "upper_" {
				return errors.New("deferred parent has upper link")
			}
		}
		return nil
	}, pause: func(ctx context.Context) error {
		timer := time.NewTimer(100 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		}
	}}
}

func (claim DeferredClaim) state() State {
	return State{Generation: claim.Generation, Boot: claim.Boot, Deadline: claim.Deadline}
}

func deferredFreshMetadata(store Store) error {
	for _, name := range []string{deferredClaimFile, deferredWaiterFile, deferredResultFile} {
		if _, err := os.Lstat(filepath.Join(store.Dir, name)); err == nil {
			return errors.New("prior deferred metadata requires explicit reconciliation")
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func deferredPriorWatchdog(store Store, auth CanaryAuthorization) error {
	var prior WorkerIdentity
	err := readPrivate(filepath.Join(store.Dir, "worker.json"), &prior)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if prior.Generation != auth.Generation || prior.Boot != auth.Boot || prior.Deadline != auth.Deadline || prior.PID <= 1 || prior.Birth == "" {
		return errors.New("refuse replacing foreign prior watchdog identity")
	}
	return nil
}
