package triggerfish

import (
	"encoding/json"
	"errors"
	"io"
	"net/netip"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// TrialFromEnvironment reads explicit, nonsecret operational trial input.
// Empty input preserves the inventory's fail-closed zero default. Errors never
// include the payload: credentials do not belong in this environment variable.
func TrialFromEnvironment() (RouterTrial, error) {
	raw := os.Getenv("GONF_ROUTER_TRIAL")
	if strings.TrimSpace(raw) == "" {
		return RouterTrial{}, nil
	}
	var trial RouterTrial
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&trial); err != nil {
		return RouterTrial{}, errors.New("invalid GONF_ROUTER_TRIAL JSON")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return RouterTrial{}, errors.New("trailing GONF_ROUTER_TRIAL input")
	}
	if err := validateTrialInput(trial); err != nil {
		return RouterTrial{}, err
	}
	return trial, nil
}

func validateTrialInput(t RouterTrial) error {
	if !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{7,63}$`).MatchString(t.Token) {
		return errors.New("GONF_ROUTER_TRIAL requires a valid trial token")
	}
	if !trialProfile(t.Profile) {
		return errors.New("GONF_ROUTER_TRIAL requires a supported profile")
	}
	if t.WindowSeconds < 180 || t.WindowSeconds > 1800 {
		return errors.New("GONF_ROUTER_TRIAL window must be 180–1800 seconds")
	}
	if t.BootCanary && !bootCanaryStage(t.Profile) {
		return errors.New("GONF_ROUTER_TRIAL boot canary requires a canary-capable profile")
	}
	if t.ReadinessReviewed && !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(t.CandidateSHA256) {
		return errors.New("GONF_ROUTER_TRIAL readiness requires the frozen candidate digest")
	}
	if t.ProofRole == "" && t.SSHConnection == "" {
		return nil
	}
	return validateTrialConnection(t)
}

func validateTrialConnection(t RouterTrial) error {
	expected := map[string]string{
		"old": "192.168.1.2", "new": "10.20.0.1",
		"recovery": "10.98.0.1", "vpn": "10.99.0.1",
	}
	fields := strings.Fields(t.SSHConnection)
	if len(fields) != 4 || expected[t.ProofRole] == "" {
		return errors.New("GONF_ROUTER_TRIAL requires a valid proof role and SSH tuple")
	}
	client, err := netip.ParseAddr(fields[0])
	if err != nil || !client.Is4() || fields[2] != expected[t.ProofRole] || fields[3] != "22" {
		return errors.New("GONF_ROUTER_TRIAL SSH tuple does not match its proof role")
	}
	port, err := strconv.ParseUint(fields[1], 10, 16)
	if err != nil || port == 0 || (t.ProofRole == "vpn" && fields[0] != "10.99.0.2") {
		return errors.New("GONF_ROUTER_TRIAL SSH tuple has an invalid client")
	}
	// lab-igc1's new receipt must cross the moved lab port (lab.go).
	return validateLabProof(t, client)
}
