package recovery

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
)

// Engine applies the journal protocol. Start must return only after worker readiness.
// Management must verify the separately installed fixed recovery endpoint.
type Engine struct {
	Store      Store
	Config     string
	Device     string
	Boot       string
	Now        func() (int64, error)
	Start      func(State) error
	Management func() error
	Reboot     func() error
}

// Arm snapshots the last-valid full document; it never writes the vendor config.
func (e Engine) Arm(generation string, target json.RawMessage) error {
	return e.Store.Locked(func() error {
		old, err := e.Store.Load()
		if err == nil {
			if old.Device != e.Device {
				return errors.New("device identity mismatch")
			}
			if old.Generation == generation {
				return errors.New("generation already used")
			}
			if old.Phase != "confirmed" && old.Phase != "restored" {
				return errors.New("transaction already pending")
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if !regexp.MustCompile(`^[a-f0-9]{32,64}$`).MatchString(generation) {
			return errors.New("generation must be a random 128-bit or stronger hex nonce")
		}
		// Exclusive durable tombstones prevent any old worker/confirm identity reuse.
		if err := os.MkdirAll(filepath.Join(e.Store.Dir, "generations"), 0700); err != nil {
			return errors.New("create generation ledger")
		}
		generationPath := filepath.Join(e.Store.Dir, "generations", generation)
		if _, err := os.Lstat(generationPath); err == nil {
			return errors.New("generation already used")
		} else if !errors.Is(err, os.ErrNotExist) {
			return errors.New("inspect generation ledger")
		}
		if err := e.Management(); err != nil {
			return errors.New("fixed management not ready")
		}
		baseline, err := os.ReadFile(e.Config)
		if err != nil {
			return errors.New("read vendor configuration")
		}
		saved, err := Network(baseline)
		if err != nil {
			return err
		}
		digest, err := Digest(target)
		if err != nil {
			return err
		}
		now, err := e.Now()
		if err != nil {
			return err
		}
		state := State{Version: 1, Device: e.Device, Generation: generation, Phase: "armed", Boot: e.Boot, Deadline: now + WindowNS, Allowance: true, Baseline: baseline, Saved: saved, Target: target, TargetDigest: digest}
		if err := Atomic(generationPath, []byte("used\n")); err != nil {
			return err
		}
		if err := e.Store.Save(state); err != nil {
			return err
		}
		if err := e.startWithinDeadline(state); err != nil {
			state.Phase = "restored"
			state.Allowance = false
			// No RPC was permitted and no vendor JSON was modified on this boot.
			if saveErr := e.Store.Save(state); saveErr != nil {
				return saveErr
			}
			return errors.New("rollback worker failed to become ready; arm canceled")
		}
		return nil
	})
}

// PreApp is called synchronously before the vendor app on every boot.
// A returned error is NOT safe permission to start the app; the wrapper must block.
func (e Engine) PreApp() error {
	return e.Store.Locked(func() error {
		state, err := e.Store.Load()
		if errors.Is(err, os.ErrNotExist) {
			return e.Management()
		}
		if err != nil {
			return err
		}
		if state.Device != e.Device {
			return errors.New("device identity mismatch")
		}
		if state.Phase == "confirmed" || state.Phase == "restored" {
			return e.Management()
		}
		if state.Phase == "restoring" {
			return e.restore(&state)
		}
		now, err := e.Now()
		if err != nil {
			return e.restore(&state)
		}
		if state.Boot == e.Boot {
			if now >= state.Deadline {
				return e.restore(&state)
			}
			// Same-boot reapply cannot consume a boot or renew the deadline.
		} else {
			latest, readErr := os.ReadFile(e.Config)
			network, parseErr := Network(latest)
			digest, digestErr := Digest(network)
			if !state.Allowance || readErr != nil || parseErr != nil || digestErr != nil || digest != state.TargetDigest {
				return e.restore(&state)
			}
			state.Allowance = false
			state.Phase = "trial"
			state.Boot = e.Boot
			state.Deadline = now + WindowNS
			if err := e.Store.Save(state); err != nil {
				return e.restore(&state)
			}
		}
		if err := e.Management(); err != nil {
			return e.restore(&state)
		}
		if err := e.startWithinDeadline(state); err != nil {
			return e.restore(&state)
		}
		return nil
	})
}

// Expire marks restoration before requesting a reboot. It never writes app JSON.
func (e Engine) Expire(generation string) error {
	return e.Store.Locked(func() error {
		state, err := e.checked(generation)
		if err != nil {
			return err
		}
		if state.Phase == "confirmed" || state.Phase == "restored" {
			return nil
		}
		if state.Boot != e.Boot {
			return errors.New("stale worker boot")
		}
		now, err := e.Now()
		if err == nil && state.Phase != "restoring" && now < state.Deadline {
			return errors.New("deadline has not elapsed")
		}
		state.Phase = "restoring"
		state.Allowance = false
		if err := e.Store.Save(state); err != nil {
			return err
		}
		if err := e.Reboot(); err != nil {
			return errors.New("reboot failed; restoration remains pending")
		}
		return nil
	})
}

// Confirm accepts only controller attestations for both fresh authenticated paths.
func (e Engine) Confirm(generation string, targetProof, recoveryProof bool) error {
	if !targetProof || !recoveryProof {
		return errors.New("both fresh authenticated path proofs required")
	}
	return e.Store.Locked(func() error {
		state, err := e.checked(generation)
		if err != nil {
			return err
		}
		if state.Phase == "confirmed" {
			return nil
		}
		if state.Phase != "trial" || state.Boot != e.Boot {
			return errors.New("transaction cannot be confirmed")
		}
		now, err := e.Now()
		if err != nil {
			return err
		}
		if now >= state.Deadline {
			return errors.New("confirmation deadline elapsed")
		}
		if err := e.Management(); err != nil {
			return errors.New("fixed management not ready")
		}
		now, err = e.Now()
		if err != nil {
			return err
		}
		if now >= state.Deadline {
			return errors.New("confirmation verification exceeded deadline")
		}
		state.Phase = "confirmed"
		state.Allowance = false
		return e.Store.Save(state)
	})
}

func (e Engine) checked(generation string) (State, error) {
	state, err := e.Store.Load()
	if err != nil {
		return state, err
	}
	if state.Device != e.Device || state.Generation != generation {
		return state, errors.New("stale transaction identity")
	}
	return state, nil
}

func (e Engine) restore(state *State) error {
	state.Phase = "restoring"
	state.Allowance = false
	if err := e.Store.Save(*state); err != nil {
		return err
	}
	latest, err := os.ReadFile(e.Config)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.New("read latest vendor configuration")
	}
	merged, fallback, err := Merge(latest, state.Baseline, state.Saved)
	if err != nil {
		return err
	}
	if err := Atomic(e.Config, merged); err != nil {
		return err
	}
	state.Phase = "restored"
	state.Fallback = fallback
	if err := e.Store.Save(*state); err != nil {
		return err
	}
	return e.Management()
}

func (e Engine) startWithinDeadline(state State) error {
	if err := e.Start(state); err != nil {
		return err
	}
	now, err := e.Now()
	if err != nil {
		return err
	}
	if now >= state.Deadline {
		return errors.New("worker startup exceeded deadline")
	}
	return nil
}
