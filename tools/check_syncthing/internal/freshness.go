package internal

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// freshnessStateFile is the freshness mode's own file in -state-dir, kept
// apart from the folder mode's so the two checks never overwrite each other.
const freshnessStateFile = "check_syncthing_freshness.json"

// maxRunGap is the longest pause between two runs that still continues
// the clocks. A longer one (the check is paused while f3s is taken down,
// or a frontend failover moved it to a node with old state) starts a new
// baseline, so stale counters and last-seen times cannot raise a false
// alert. Gogios runs the check every 30 minutes.
const maxRunGap = 2 * time.Hour

// DeviceRule asks the freshness mode to warn when Device (an ID or name)
// has been neither connected nor seen for longer than MaxAge.
type DeviceRule struct {
	Device string
	MaxAge time.Duration
}

// ParseDeviceRule parses "<device ID or name>=<duration>", e.g.
// "Pixel 7 Pro=72h". The name may contain "=", so the last one splits.
func ParseDeviceRule(s string) (DeviceRule, error) {
	i := strings.LastIndex(s, "=")
	if i <= 0 {
		return DeviceRule{}, fmt.Errorf("device rule %q is not <device>=<duration>", s)
	}
	maxAge, err := time.ParseDuration(strings.TrimSpace(s[i+1:]))
	if err != nil || maxAge <= 0 {
		return DeviceRule{}, fmt.Errorf("device rule %q needs a positive duration", s)
	}
	return DeviceRule{Device: strings.TrimSpace(s[:i]), MaxAge: maxAge}, nil
}

// FreshState is the freshness mode's memory between runs: the received-byte
// counter per device at the last run, when that run was, when any counter
// last advanced, and since when no data has arrived for longer than the
// period (zero while data is fresh), which drives the escalation.
type FreshState struct {
	InBytes     map[string]int64 `json:"inBytes"`
	LastRun     time.Time        `json:"lastRun"`
	LastAdvance time.Time        `json:"lastAdvance"`
	StaleSince  time.Time        `json:"staleSince"`
}

// Observe records the current counters and reports whether this run is a
// baseline. Data arrived if a device's counter grew since the last run. A
// counter that shrank (the device reconnected, or Syncthing restarted) and
// a device seen for the first time have nothing to compare with; both only
// set a new baseline for that device. The first run ever, and the first
// after a pause longer than maxRunGap, start all clocks afresh.
func (s FreshState) Observe(counters map[string]int64, now time.Time) (FreshState, bool) {
	next := FreshState{InBytes: counters, LastRun: now, LastAdvance: s.LastAdvance, StaleSince: s.StaleSince}
	if s.LastRun.IsZero() || now.Sub(s.LastRun) > maxRunGap {
		next.LastAdvance, next.StaleSince = now, time.Time{}
		return next, true
	}
	for id, n := range counters {
		if prev, ok := s.InBytes[id]; ok && n > prev {
			next.LastAdvance = now
			break
		}
	}
	return next, false
}

// freshFinding is one stale condition of the freshness mode.
type freshFinding struct {
	Level Level
	Text  string
}

// runFreshness proves that Syncthing actually receives data: some peer's
// received-byte counter must advance within cfg.Period (WARNING, then
// CRITICAL once that lasted cfg.EscalateAfter), and every cfg.Devices peer
// must be connected or seen within its MaxAge (WARNING). Ignored devices
// count for neither.
func runFreshness(ctx context.Context, client Client, rules []IgnoreRule, cfg Config, now time.Time) Result {
	if cfg.StateDir == "" {
		return unknown(errors.New("freshness mode needs -state-dir"))
	}
	if cfg.Period <= 0 {
		return unknown(fmt.Errorf("period must be positive, got %s", cfg.Period))
	}
	devices, err := client.Devices(ctx)
	if err != nil {
		return apiFailure(err)
	}
	conns, err := client.Connections(ctx)
	if err != nil {
		return apiFailure(err)
	}
	stats, err := client.DeviceStats(ctx)
	if err != nil {
		return apiFailure(err)
	}
	watched, err := resolveDevices(cfg.Devices, devices)
	if err != nil {
		return unknown(err)
	}
	ignored := ignoredDevices(devices, rules)

	st, err := readStateFile[FreshState](cfg.StateDir, freshnessStateFile)
	if err != nil {
		return unknown(fmt.Errorf("state error: %w", err))
	}
	counters := make(map[string]int64, len(conns))
	for id, conn := range conns {
		if !ignored[id] {
			counters[id] = conn.InBytesTotal
		}
	}
	st, baseline := st.Observe(counters, now)
	result := Result{Level: OK, Line: "SYNCTHING FRESHNESS OK: baseline recorded (first run or after a pause)"}
	if !baseline {
		// Peers' last-seen times are as stale as the counters right after
		// a pause, so a baseline run judges neither.
		var findings []freshFinding
		if finding, stale := transferFinding(&st, cfg, now); stale {
			findings = append(findings, finding)
		}
		findings = append(findings, deviceFindings(watched, ignored, conns, stats, now)...)
		result = freshSummary(findings, now.Sub(st.LastAdvance))
	}
	stateErr := writeStateFile(cfg.StateDir, freshnessStateFile, st)
	if stateErr != nil {
		stateErr = fmt.Errorf("state error: %w", stateErr)
	}
	return withStateError(result, stateErr)
}

// transferFinding judges how long no data arrived and keeps StaleSince:
// set when the condition starts, cleared once data is fresh again.
func transferFinding(st *FreshState, cfg Config, now time.Time) (freshFinding, bool) {
	quiet := now.Sub(st.LastAdvance)
	if quiet < cfg.Period {
		st.StaleSince = time.Time{}
		return freshFinding{}, false
	}
	if st.StaleSince.IsZero() {
		st.StaleSince = now
	}
	finding := freshFinding{Level: Warning, Text: fmt.Sprintf("no data from any peer for %s (limit %s)",
		humanDuration(quiet), humanDuration(cfg.Period))}
	if cfg.EscalateAfter > 0 && now.Sub(st.StaleSince) >= cfg.EscalateAfter {
		finding.Level = Critical
		finding.Text += " (escalated)"
	}
	return finding, true
}

// watchedDevice is a DeviceRule resolved to a configured device.
type watchedDevice struct {
	Device
	MaxAge time.Duration
}

func resolveDevices(rules []DeviceRule, devices []Device) ([]watchedDevice, error) {
	watched := make([]watchedDevice, 0, len(rules))
	for _, rule := range rules {
		found := false
		for _, d := range devices {
			if rule.Device == d.ID || rule.Device == d.Name {
				watched = append(watched, watchedDevice{Device: d, MaxAge: rule.MaxAge})
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("device %q is not configured in Syncthing", rule.Device)
		}
	}
	return watched, nil
}

func ignoredDevices(devices []Device, rules []IgnoreRule) map[string]bool {
	ignored := map[string]bool{}
	for _, d := range devices {
		for _, rule := range rules {
			if rule.MatchesDevice(d) {
				ignored[d.ID] = true
			}
		}
	}
	return ignored
}

func deviceFindings(watched []watchedDevice, ignored map[string]bool, conns map[string]Connection,
	stats map[string]DeviceStats, now time.Time) []freshFinding {
	var findings []freshFinding
	for _, w := range watched {
		if ignored[w.ID] || conns[w.ID].Connected {
			continue
		}
		lastSeen := stats[w.ID].LastSeen
		age := now.Sub(lastSeen)
		switch {
		case lastSeen.Unix() <= 0: // Syncthing reports "never" as the Unix epoch.
			findings = append(findings, freshFinding{Level: Warning, Text: fmt.Sprintf("%s never seen (limit %s)",
				deviceName(w.Device), humanDuration(w.MaxAge))})
		case age > w.MaxAge:
			findings = append(findings, freshFinding{Level: Warning, Text: fmt.Sprintf("%s not seen for %s (limit %s)",
				deviceName(w.Device), humanDuration(age), humanDuration(w.MaxAge))})
		}
	}
	return findings
}

func deviceName(d Device) string {
	if d.Name != "" {
		return d.Name
	}
	return d.ID
}

func freshSummary(findings []freshFinding, quiet time.Duration) Result {
	level := OK
	for _, f := range findings {
		level = max(level, f.Level)
	}
	if level == OK {
		return Result{Level: OK, Line: "SYNCTHING FRESHNESS OK: last data from a peer " + humanDuration(quiet) + " ago"}
	}
	sort.SliceStable(findings, func(i, j int) bool { return findings[i].Level > findings[j].Level })
	texts := make([]string, 0, len(findings))
	for _, f := range findings {
		texts = append(texts, f.Text)
	}
	return Result{Level: level, Line: fmt.Sprintf("SYNCTHING FRESHNESS %s: %s", level, strings.Join(texts, "; "))}
}
