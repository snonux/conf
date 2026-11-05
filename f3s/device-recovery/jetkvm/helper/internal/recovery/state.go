package recovery

import (
	"encoding/json"
	"errors"
	"regexp"
)

// WindowNS bounds each live pending boot without wall-clock input.
const WindowNS int64 = 120_000_000_000

// State is the root-private durable journal. It must never be logged wholesale.
type State struct {
	Version      int             `json:"version"`
	Device       string          `json:"device"`
	Generation   string          `json:"generation"`
	Phase        string          `json:"phase"`
	Boot         string          `json:"boot"`
	Deadline     int64           `json:"deadline_boottime_ns"`
	Allowance    bool            `json:"allowance"`
	Baseline     json.RawMessage `json:"baseline"`
	Saved        json.RawMessage `json:"saved_network"`
	Target       json.RawMessage `json:"target_network"`
	TargetDigest string          `json:"target_digest"`
	Fallback     bool            `json:"fallback_used"`
}

// Validate rejects corrupt journals without displaying their private fields.
func (s State) Validate() error {
	id := regexp.MustCompile(`^[a-zA-Z0-9-]{8,80}$`)
	if s.Version != 1 || !id.MatchString(s.Device) || !id.MatchString(s.Generation) || !id.MatchString(s.Boot) || s.Deadline <= 0 {
		return errors.New("invalid transaction identity or deadline")
	}
	switch s.Phase {
	case "armed", "trial", "restoring", "restored", "confirmed":
	default:
		return errors.New("invalid transaction phase")
	}
	if s.Phase != "armed" && s.Allowance {
		return errors.New("invalid boot allowance")
	}
	if _, err := Network(s.Baseline); err != nil {
		return errors.New("invalid configuration safety copy")
	}
	if _, err := document(s.Saved); err != nil {
		return errors.New("invalid saved network field")
	}
	digest, err := Digest(s.Target)
	if err != nil || digest != s.TargetDigest {
		return errors.New("invalid target network pin")
	}
	saved, _ := Network(s.Baseline)
	a, _ := Digest(saved)
	b, _ := Digest(s.Saved)
	if a != b {
		return errors.New("saved network and safety copy disagree")
	}
	return nil
}
