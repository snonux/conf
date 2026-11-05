//go:build linux

package recovery

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// StartCanary pins root authorization once and arms cleanup before mutation.
func StartCanary(executable, config, profilePath, boot string, adapter Adapter, now func() (int64, error)) error {
	return adapter.Store.Locked(func() error {
		state, stateErr := adapter.Store.Load()
		if stateErr == nil && state.Phase != "confirmed" && state.Phase != "restored" {
			return errors.New("primary transaction pending; additive bootstrap forbidden")
		}
		if stateErr != nil && !errors.Is(stateErr, os.ErrNotExist) {
			return stateErr
		}
		var auth CanaryAuthorization
		if err := readPrivate(filepath.Join(adapter.Store.Dir, "canary-authorization.json"), &auth); err != nil {
			return err
		}
		tick, err := now()
		if err != nil {
			return err
		}
		if !regexp.MustCompile(`^[a-f0-9]{32,64}$`).MatchString(auth.Generation) || auth.Confirmed || auth.Expired || auth.Hash != adapter.Profile.Hash() || auth.Boot != boot || auth.Deadline <= tick || auth.Deadline-tick > WindowNS {
			return errors.New("canary authorization expired or invalid")
		}
		var active CanaryAuthorization
		err = readPrivate(filepath.Join(adapter.Store.Dir, "canary-active.json"), &active)
		if err == nil {
			if active.Generation == auth.Generation {
				auth = active
			} else if !active.Confirmed && !active.Expired {
				return errors.New("different canary already pending")
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if auth.Confirmed || auth.Expired || auth.Boot != boot || auth.Deadline <= tick {
			return errors.New("canary cannot be renewed")
		}
		if err := saveCanary(adapter.Store, auth); err != nil {
			return err
		}
		start := func(state State) error {
			return launch(executable, adapter.Store.Dir, config, adapter.Profile.Device, profilePath, "canary-worker", state)
		}
		return guardedCanary(adapter, auth, tick, start)
	})
}

// ManagementBoot permits one explicitly authorized bounded additive boot trial.
// It never requires or creates a completed native acceptance receipt.
func ManagementBoot(adapter Adapter, boot string, now int64, start func(State) error) error {
	return adapter.Store.Locked(func() error {
		state, err := adapter.Store.Load()
		if err == nil && state.Phase != "confirmed" && state.Phase != "restored" {
			return errors.New("primary transaction pending; full pre-app hook required")
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		var auth CanaryAuthorization
		if err := readPrivate(filepath.Join(adapter.Store.Dir, "canary-active.json"), &auth); err != nil {
			return err
		}
		if auth.Hash != adapter.Profile.Hash() {
			return errors.New("canary profile mismatch")
		}
		if auth.Confirmed {
			return errors.New("new bounded boot authorization required")
		}
		if auth.Boot != boot {
			if !auth.BootAllowance || auth.Expired {
				auth.Expired = true
				if err := saveCanary(adapter.Store, auth); err != nil {
					return err
				}
				return adapter.Cleanup()
			}
			auth.Boot = boot
			auth.BootAllowance = false
			auth.Deadline = now + WindowNS
			if err := saveCanary(adapter.Store, auth); err != nil {
				return err
			}
		}
		if auth.Expired || now >= auth.Deadline {
			auth.Expired = true
			if err := saveCanary(adapter.Store, auth); err != nil {
				return err
			}
			return adapter.Cleanup()
		}
		return guardedCanary(adapter, auth, now, start)
	})
}

// WorkCanary cleans only additive resources, with an immutable boot/nonce deadline.
func WorkCanary(adapter Adapter, boot, generation string, now func() (int64, error), ready *os.File) error {
	var auth CanaryAuthorization
	if err := readPrivate(filepath.Join(adapter.Store.Dir, "canary-active.json"), &auth); err != nil {
		return err
	}
	tick, err := now()
	if err != nil {
		return err
	}
	if auth.Confirmed || auth.Expired || auth.Generation != generation || auth.Hash != adapter.Profile.Hash() || auth.Boot != boot || tick >= auth.Deadline {
		return errors.New("stale canary worker")
	}
	birth, err := processBirth(os.Getpid())
	if err != nil {
		return err
	}
	if err := json.NewEncoder(ready).Encode(WorkerIdentity{generation, boot, auth.Deadline, os.Getpid(), birth}); err != nil {
		return errors.New("canary readiness failed")
	}
	if err := ready.Close(); err != nil {
		return err
	}
	for {
		tick, clockErr := now()
		done := false
		err := adapter.Store.Locked(func() error {
			var current CanaryAuthorization
			if err := readPrivate(filepath.Join(adapter.Store.Dir, "canary-active.json"), &current); err != nil {
				return err
			}
			if current.Generation != auth.Generation || current.Boot != boot || current.Deadline != auth.Deadline || current.Confirmed {
				done = true
				return nil
			}
			if clockErr != nil || tick >= auth.Deadline || current.Expired {
				current.Expired = true
				current.BootAllowance = false
				if err := saveCanary(adapter.Store, current); err != nil {
					return err
				}
				if err := adapter.Cleanup(); err != nil {
					return err
				}
				done = true
			}
			return nil
		})
		if err == nil && done {
			return nil
		}
		time.Sleep(time.Second)
	}
}

// KeepCanary cancels additive timeout after a fresh authenticated fixed-path proof.
// It does not grant native boot/renew acceptance or permit primary-network changes.
func KeepCanary(adapter Adapter, boot string, proof bool) error {
	if !proof {
		return errors.New("fresh authenticated recovery proof required")
	}
	if adapter.Now == nil {
		return errors.New("monotonic canary confirmation clock required")
	}
	return adapter.Store.Locked(func() error {
		now, err := adapter.Now()
		if err != nil {
			return err
		}
		var auth CanaryAuthorization
		if err := readPrivate(filepath.Join(adapter.Store.Dir, "canary-active.json"), &auth); err != nil {
			return err
		}
		if auth.Expired || auth.Hash != adapter.Profile.Hash() || auth.Boot != boot || now >= auth.Deadline {
			return errors.New("stale canary confirmation")
		}
		if err := adapter.Probe(); err != nil {
			return err
		}
		now, err = adapter.Now()
		if err != nil {
			return err
		}
		if now >= auth.Deadline {
			return errors.New("canary confirmation verification exceeded deadline")
		}
		auth.Confirmed = true
		auth.BootAllowance = false
		return saveCanary(adapter.Store, auth)
	})
}

func guardedCanary(adapter Adapter, auth CanaryAuthorization, now int64, start func(State) error) error {
	state := State{Generation: auth.Generation, Boot: auth.Boot, Deadline: auth.Deadline}
	if err := start(state); err != nil {
		auth.Expired = true
		auth.BootAllowance = false
		if saveErr := saveCanary(adapter.Store, auth); saveErr != nil {
			return saveErr
		}
		if cleanupErr := adapter.Cleanup(); cleanupErr != nil {
			return errors.New("worker failed; owned cleanup incomplete")
		}
		return errors.New("canary worker failed; owned resources cleaned")
	}
	if adapter.Now != nil {
		tick, err := adapter.Now()
		if err != nil || tick >= auth.Deadline {
			return expireCanary(adapter, auth)
		}
		now = tick
	}
	if err := adapter.Bootstrap(auth.Boot, now); err != nil {
		if cleanupErr := adapter.Cleanup(); cleanupErr != nil {
			return errors.New("canary failed; cleanup pending under independent worker")
		}
		return err
	}
	if adapter.Now != nil {
		tick, err := adapter.Now()
		if err != nil || tick >= auth.Deadline {
			return expireCanary(adapter, auth)
		}
	}

	return nil
}
func saveCanary(store Store, auth CanaryAuthorization) error {
	data, err := json.Marshal(auth)
	if err != nil {
		return err
	}
	return Atomic(filepath.Join(store.Dir, "canary-active.json"), data)
}

func expireCanary(adapter Adapter, auth CanaryAuthorization) error {
	auth.Expired = true
	auth.BootAllowance = false
	if err := saveCanary(adapter.Store, auth); err != nil {
		return err
	}
	if err := adapter.Cleanup(); err != nil {
		return err
	}
	return errors.New("canary setup exceeded deadline; owned resources cleaned")
}
