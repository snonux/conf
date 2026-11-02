package internal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	earthID  = "EARTH-ID"
	pixelID  = "PIXEL-ID"
	galaxyID = "GALAXY-ID"
)

// fakeFresh serves the three endpoints the freshness mode reads. Tests
// change inBytes, connected and lastSeen between runs.
type fakeFresh struct {
	inBytes   map[string]int64
	connected map[string]bool
	lastSeen  map[string]time.Time
	rejectAt  string // path that answers 403
}

func newFakeFresh(t0 time.Time) *fakeFresh {
	return &fakeFresh{
		inBytes:   map[string]int64{earthID: 1000, pixelID: 0, galaxyID: 0},
		connected: map[string]bool{earthID: true},
		lastSeen:  map[string]time.Time{earthID: t0, pixelID: t0, galaxyID: time.Unix(0, 0).UTC()},
	}
}

func (f *fakeFresh) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-API-Key") != testAPIKey || r.URL.Path == f.rejectAt {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	var body any
	switch r.URL.Path {
	case "/rest/config/devices":
		body = []map[string]string{
			{"deviceID": earthID, "name": "earth"}, {"deviceID": pixelID, "name": "Pixel 7 Pro"},
			{"deviceID": galaxyID, "name": "Galaxy Tab S10 FE"}, {"deviceID": "SELF-ID", "name": "f3s"},
		}
	case "/rest/system/connections":
		conns := map[string]map[string]any{}
		for id, n := range f.inBytes {
			conns[id] = map[string]any{"connected": f.connected[id], "inBytesTotal": n}
		}
		body = map[string]any{"connections": conns, "total": map[string]any{"inBytesTotal": 0}}
	case "/rest/config/folders":
		body = []any{}
	case "/rest/stats/device":
		stats := map[string]map[string]any{}
		for id, seen := range f.lastSeen {
			stats[id] = map[string]any{"lastSeen": seen.Format(time.RFC3339)}
		}
		body = stats
	default:
		http.NotFound(w, r)
		return
	}
	_ = json.NewEncoder(w).Encode(body)
}

func freshConfig(t *testing.T, url string) Config {
	cfg := baseConfig(t, url)
	cfg.Mode = ModeFreshness
	cfg.StateDir = t.TempDir()
	cfg.Period = 24 * time.Hour
	cfg.EscalateAfter = 24 * time.Hour
	return cfg
}

func runFresh(t *testing.T, cfg Config, now time.Time, wantLevel Level, wantLine string) {
	t.Helper()
	got := Run(context.Background(), cfg, now)
	if got.Level != wantLevel || !strings.Contains(got.Line, wantLine) {
		t.Fatalf("at %s: got %v %q, want %v containing %q", now.Format(time.RFC3339), got.Level, got.Line, wantLevel, wantLine)
	}
	assertOneLineNoSecret(t, got.Line)
}

// sampleUntil runs the check every 30 minutes, like Gogios, from just
// after from up to and excluding to, and returns to for the asserted run.
func sampleUntil(t *testing.T, cfg Config, from, to time.Time) time.Time {
	t.Helper()
	for at := from.Add(30 * time.Minute); at.Before(to); at = at.Add(30 * time.Minute) {
		if got := Run(context.Background(), cfg, at); got.Level == Unknown {
			t.Fatalf("sample at %s: %q", at.Format(time.RFC3339), got.Line)
		}
	}
	return to
}

// TestFreshnessTransferLifecycle drives the received-byte clock across
// half-hourly runs: baseline, quiet but within the period, WARNING after
// it, CRITICAL once the stale condition lasted -escalate-after, and OK with
// the escalation clock reset once a counter grows again.
func TestFreshnessTransferLifecycle(t *testing.T) {
	t0 := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	fake := newFakeFresh(t0)
	srv := httptest.NewServer(fake)
	defer srv.Close()
	cfg := freshConfig(t, srv.URL)
	at := func(d time.Duration) time.Time { return t0.Add(d) }

	runFresh(t, cfg, t0, OK, "SYNCTHING FRESHNESS OK: baseline recorded")
	runFresh(t, cfg, at(30*time.Minute), OK, "last data from a peer 30m ago")
	runFresh(t, cfg, sampleUntil(t, cfg, at(30*time.Minute), at(23*time.Hour+30*time.Minute)), OK, "23h30m ago")
	runFresh(t, cfg, at(24*time.Hour), Warning, "SYNCTHING FRESHNESS WARNING: no data from any peer for 24h00m (limit 24h00m)")
	runFresh(t, cfg, sampleUntil(t, cfg, at(24*time.Hour), at(47*time.Hour+30*time.Minute)), Warning, "for 47h30m")
	runFresh(t, cfg, at(48*time.Hour), Critical, "for 48h00m (limit 24h00m) (escalated)")

	fake.inBytes[earthID] = 5000
	runFresh(t, cfg, at(48*time.Hour+30*time.Minute), OK, "last data from a peer 0m ago")
	// The escalation clock restarted: a new quiet spell warns first.
	runFresh(t, cfg, sampleUntil(t, cfg, at(48*time.Hour+30*time.Minute), at(72*time.Hour+30*time.Minute)), Warning, "no data from any peer for 24h00m")
}

// TestFreshnessCounterDropIsNotData covers a reconnect or Syncthing
// restart: the counter drops, which must only set a new baseline for that
// device, not count as data.
func TestFreshnessCounterDropIsNotData(t *testing.T) {
	t0 := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	fake := newFakeFresh(t0)
	srv := httptest.NewServer(fake)
	defer srv.Close()
	cfg := freshConfig(t, srv.URL)

	runFresh(t, cfg, t0, OK, "baseline recorded")
	fake.inBytes[earthID] = 10 // reconnect: the counter starts over
	runFresh(t, cfg, sampleUntil(t, cfg, t0, t0.Add(24*time.Hour)), Warning, "no data from any peer for 24h00m")
	fake.inBytes[earthID] = 20 // grows from the new baseline: real data
	runFresh(t, cfg, t0.Add(24*time.Hour+30*time.Minute), OK, "0m ago")
}

// TestFreshnessNewPeerIsNotData: a peer that appears with bytes already
// counted has nothing to compare with, so it proves nothing yet.
func TestFreshnessNewPeerIsNotData(t *testing.T) {
	t0 := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	fake := newFakeFresh(t0)
	delete(fake.inBytes, pixelID)
	srv := httptest.NewServer(fake)
	defer srv.Close()
	cfg := freshConfig(t, srv.URL)

	runFresh(t, cfg, t0, OK, "baseline recorded")
	runFresh(t, cfg, sampleUntil(t, cfg, t0, t0.Add(23*time.Hour+30*time.Minute)), OK, "23h30m ago")
	fake.inBytes[pixelID] = 4096
	runFresh(t, cfg, t0.Add(24*time.Hour), Warning, "no data from any peer for 24h00m")
}

// TestFreshnessPauseStartsNewBaseline covers the check being paused (f3s
// taken down, frontend failover): old counters and last-seen times must
// not raise a false alert on the first run back.
func TestFreshnessPauseStartsNewBaseline(t *testing.T) {
	t0 := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	fake := newFakeFresh(t0)
	srv := httptest.NewServer(fake)
	defer srv.Close()
	cfg := freshConfig(t, srv.URL)
	cfg.Devices = []DeviceRule{{"Pixel 7 Pro", 72 * time.Hour}}

	runFresh(t, cfg, t0, OK, "baseline recorded")
	runFresh(t, cfg, sampleUntil(t, cfg, t0, t0.Add(30*time.Hour)), Warning, "no data from any peer for 30h00m")
	// Paused for 4 days; Syncthing restarted, earth's counter is lower.
	back := t0.Add(126 * time.Hour)
	fake.inBytes[earthID] = 50
	runFresh(t, cfg, back, OK, "baseline recorded (first run or after a pause)")
	fake.inBytes[earthID] = 70
	fake.lastSeen[pixelID] = back
	runFresh(t, cfg, back.Add(30*time.Minute), OK, "last data from a peer 0m ago")
	// Within maxRunGap the clock continues: a 2h gap is not a pause.
	runFresh(t, cfg, back.Add(150*time.Minute), OK, "last data from a peer 2h00m ago")
}

// TestFreshnessIgnoredDeviceDataDoesNotCount: bytes from an ignored device
// are no proof of a working sync.
func TestFreshnessIgnoredDeviceDataDoesNotCount(t *testing.T) {
	t0 := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	fake := newFakeFresh(t0)
	srv := httptest.NewServer(fake)
	defer srv.Close()
	cfg := freshConfig(t, srv.URL)
	cfg.IgnoreFile = writeFile(t, "device: Galaxy Tab S10 FE\n")

	runFresh(t, cfg, t0, OK, "baseline recorded")
	fake.inBytes[galaxyID] = 999999
	runFresh(t, cfg, sampleUntil(t, cfg, t0, t0.Add(24*time.Hour)), Warning, "no data from any peer")
}

// TestFreshnessSharesStateDirWithFolders runs both modes against one state
// directory, as on the frontends: neither may clobber the other's state.
func TestFreshnessSharesStateDirWithFolders(t *testing.T) {
	t0 := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	fake := newFakeFresh(t0)
	srv := httptest.NewServer(fake)
	defer srv.Close()
	cfg := freshConfig(t, srv.URL)
	folders := cfg
	folders.Mode = ModeFolders

	runFresh(t, cfg, t0, OK, "baseline recorded")
	for at := t0.Add(30 * time.Minute); at.Before(t0.Add(24 * time.Hour)); at = at.Add(30 * time.Minute) {
		runFresh(t, folders, at, OK, "SYNCTHING OK: 0 of 0 folders in sync")
		runFresh(t, cfg, at, OK, "last data from a peer")
	}
	runFresh(t, cfg, t0.Add(24*time.Hour), Warning, "no data from any peer for 24h00m")
}

func TestFreshnessDevices(t *testing.T) {
	t0 := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	now := t0.Add(30 * time.Minute)
	tests := []struct {
		name      string
		rules     []DeviceRule
		ignore    string
		prepare   func(f *fakeFresh)
		wantLevel Level
		wantLine  string
	}{
		{
			name:      "seen within its limit",
			rules:     []DeviceRule{{"Pixel 7 Pro", 72 * time.Hour}},
			prepare:   func(f *fakeFresh) { f.lastSeen[pixelID] = now.Add(-71 * time.Hour) },
			wantLevel: OK,
		},
		{
			name:      "not seen beyond its limit",
			rules:     []DeviceRule{{"Pixel 7 Pro", 72 * time.Hour}},
			prepare:   func(f *fakeFresh) { f.lastSeen[pixelID] = now.Add(-80 * time.Hour) },
			wantLevel: Warning,
			wantLine:  "SYNCTHING FRESHNESS WARNING: Pixel 7 Pro not seen for 80h00m (limit 72h00m)",
		},
		{
			name:  "connected counts as seen",
			rules: []DeviceRule{{pixelID, 72 * time.Hour}},
			prepare: func(f *fakeFresh) {
				f.lastSeen[pixelID] = now.Add(-80 * time.Hour)
				f.connected[pixelID] = true
			},
			wantLevel: OK,
		},
		{
			name:      "never seen",
			rules:     []DeviceRule{{"Galaxy Tab S10 FE", time.Hour}},
			wantLevel: Warning,
			wantLine:  "Galaxy Tab S10 FE never seen (limit 1h00m)",
		},
		{
			name:      "ignored device does not fire",
			rules:     []DeviceRule{{"Galaxy Tab S10 FE", time.Hour}},
			ignore:    "device: " + galaxyID + "\n",
			wantLevel: OK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeFresh(t0)
			if tt.prepare != nil {
				tt.prepare(fake)
			}
			srv := httptest.NewServer(fake)
			defer srv.Close()
			cfg := freshConfig(t, srv.URL)
			cfg.Devices = tt.rules
			if tt.ignore != "" {
				cfg.IgnoreFile = writeFile(t, tt.ignore)
			}
			// The first run only records a baseline, whatever the devices.
			runFresh(t, cfg, t0, OK, "baseline recorded")
			runFresh(t, cfg, now, tt.wantLevel, tt.wantLine)
		})
	}
}

func TestFreshnessConfigAndAPIErrors(t *testing.T) {
	t0 := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	fake := newFakeFresh(t0)
	srv := httptest.NewServer(fake)
	defer srv.Close()
	cfg := freshConfig(t, srv.URL)
	cfg.Devices = []DeviceRule{{"Pixel 9a", time.Hour}}
	runFresh(t, cfg, t0, Unknown, `SYNCTHING UNKNOWN: device "Pixel 9a" is not configured in Syncthing`)

	fake.rejectAt = "/rest/system/connections"
	runFresh(t, freshConfig(t, srv.URL), t0, Critical, "SYNCTHING CRITICAL: GET /rest/system/connections: HTTP 403: API key rejected")
}

func TestFreshnessNeedsStateDirAndPeriod(t *testing.T) {
	fake := newFakeFresh(time.Now())
	srv := httptest.NewServer(fake)
	defer srv.Close()
	cfg := freshConfig(t, srv.URL)
	cfg.StateDir = ""
	runFresh(t, cfg, time.Now(), Unknown, "freshness mode needs -state-dir")
	cfg = freshConfig(t, srv.URL)
	cfg.Period = 0
	runFresh(t, cfg, time.Now(), Unknown, "period must be positive")
	cfg = freshConfig(t, srv.URL)
	cfg.Mode = "fresh"
	runFresh(t, cfg, time.Now(), Unknown, `unknown mode "fresh"`)
}

func TestParseDeviceRule(t *testing.T) {
	good := map[string]DeviceRule{
		"Pixel 7 Pro=72h":  {"Pixel 7 Pro", 72 * time.Hour},
		"odd=name = 30m":   {"odd=name", 30 * time.Minute},
		"ABC-DEF-123=1h5m": {"ABC-DEF-123", time.Hour + 5*time.Minute},
	}
	for in, want := range good {
		if got, err := ParseDeviceRule(in); err != nil || got != want {
			t.Errorf("ParseDeviceRule(%q) = %+v, %v; want %+v", in, got, err, want)
		}
	}
	for _, in := range []string{"Pixel", "=72h", "Pixel=", "Pixel=soon", "Pixel=0s", "Pixel=-1h"} {
		if _, err := ParseDeviceRule(in); err == nil {
			t.Errorf("ParseDeviceRule(%q) succeeded, want an error", in)
		}
	}
}
