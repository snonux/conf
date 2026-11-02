package internal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testAPIKey = "s3cr3t-test-api-key"

// fakeSyncthing serves folders and per-folder db/status bodies (raw JSON)
// and rejects requests without the test API key like Syncthing does.
type fakeSyncthing struct {
	folders  string
	statuses map[string]string
	code     int // when set, every request answers with this status
}

func (f fakeSyncthing) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-API-Key") != testAPIKey {
		http.Error(w, "CSRF Error", http.StatusForbidden)
		return
	}
	if f.code != 0 {
		http.Error(w, "boom", f.code)
		return
	}
	switch r.URL.Path {
	case "/rest/config/folders":
		_, _ = w.Write([]byte(f.folders))
	case "/rest/db/status":
		body, ok := f.statuses[r.URL.Query().Get("folder")]
		if !ok {
			http.Error(w, "no such folder", http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(body))
	default:
		http.NotFound(w, r)
	}
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// liveLike mirrors the f3s instance: one healthy folder and the known
// stuck Galaxy Tab DCIM folder.
func liveLike(t *testing.T) fakeSyncthing {
	return fakeSyncthing{
		folders: readFixture(t, "folders.json"),
		statuses: map[string]string{
			"docs-1234":       readFixture(t, "status_idle.json"),
			"sm-x526b-photos": readFixture(t, "status_stuck.json"),
		},
	}
}

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func baseConfig(t *testing.T, url string) Config {
	return Config{
		URL:        url,
		APIKeyFile: writeFile(t, testAPIKey+"\n"),
		APIKeyEnv:  "CHECK_SYNCTHING_TEST_KEY",
		Timeout:    5 * time.Second,
	}
}

func TestRun(t *testing.T) {
	ignoreGalaxy := "# disconnected source, see f3s/syncthing/README.md\nGalaxy Tab S10 FE DCIM; max-items=4; max-bytes=20226448\n"
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		server    func(t *testing.T) fakeSyncthing
		configure func(t *testing.T, cfg *Config)
		wantLevel Level
		wantLine  string
	}{
		{
			name:      "stuck folder is CRITICAL",
			server:    liveLike,
			wantLevel: Critical,
			wantLine:  "SYNCTHING CRITICAL: 1 of 2 folders out of sync: Galaxy Tab S10 FE DCIM idle need=4 (20.2 MB)",
		},
		{
			name:   "ignored stuck folder is OK",
			server: liveLike,
			configure: func(t *testing.T, cfg *Config) {
				cfg.IgnoreFile = writeFile(t, ignoreGalaxy)
			},
			wantLevel: OK,
			wantLine:  "SYNCTHING OK: 1 of 2 folders in sync; ignored: Galaxy Tab S10 FE DCIM idle need=4 (20.2 MB)",
		},
		{
			name:   "key from environment",
			server: liveLike,
			configure: func(t *testing.T, cfg *Config) {
				cfg.APIKeyFile = ""
				t.Setenv(cfg.APIKeyEnv, testAPIKey)
			},
			wantLevel: Critical,
			wantLine:  "SYNCTHING CRITICAL",
		},
		{
			name:      "HTTP error is UNKNOWN",
			server:    func(*testing.T) fakeSyncthing { return fakeSyncthing{code: http.StatusInternalServerError} },
			wantLevel: Unknown,
			wantLine:  "SYNCTHING UNKNOWN: GET /rest/config/folders: HTTP 500",
		},
		{
			name:   "wrong API key is UNKNOWN",
			server: liveLike,
			configure: func(t *testing.T, cfg *Config) {
				cfg.APIKeyFile = writeFile(t, "wrong-key")
			},
			wantLevel: Unknown,
			wantLine:  "HTTP 403",
		},
		{
			name: "bad folders JSON is UNKNOWN",
			server: func(*testing.T) fakeSyncthing {
				return fakeSyncthing{folders: "<html>login</html>"}
			},
			wantLevel: Unknown,
			wantLine:  "SYNCTHING UNKNOWN: decode /rest/config/folders",
		},
		{
			name: "bad status JSON is UNKNOWN",
			server: func(t *testing.T) fakeSyncthing {
				f := liveLike(t)
				f.statuses["docs-1234"] = `{"state": 42}`
				return f
			},
			wantLevel: Unknown,
			wantLine:  "SYNCTHING UNKNOWN: folder docs-1234: decode /rest/db/status",
		},
		{
			name: "status HTTP error is UNKNOWN",
			server: func(t *testing.T) fakeSyncthing {
				f := liveLike(t)
				delete(f.statuses, "docs-1234")
				return f
			},
			wantLevel: Unknown,
			wantLine:  "SYNCTHING UNKNOWN: folder docs-1234: GET /rest/db/status: HTTP 404",
		},
		{
			name:   "missing API key is UNKNOWN",
			server: liveLike,
			configure: func(t *testing.T, cfg *Config) {
				cfg.APIKeyFile = ""
			},
			wantLevel: Unknown,
			wantLine:  "SYNCTHING UNKNOWN: no API key: pass -api-key-file or set $CHECK_SYNCTHING_TEST_KEY",
		},
		{
			name:   "empty API key file is UNKNOWN",
			server: liveLike,
			configure: func(t *testing.T, cfg *Config) {
				cfg.APIKeyFile = writeFile(t, " \n")
			},
			wantLevel: Unknown,
			wantLine:  "is empty",
		},
		{
			name:   "broken ignore file is UNKNOWN",
			server: liveLike,
			configure: func(t *testing.T, cfg *Config) {
				cfg.IgnoreFile = writeFile(t, "Galaxy; max-items=lots\n")
			},
			wantLevel: Unknown,
			wantLine:  "SYNCTHING UNKNOWN: ignore file",
		},
		{
			name:   "unwritable state dir turns WARNING into UNKNOWN",
			server: syncingOnly,
			configure: func(t *testing.T, cfg *Config) {
				cfg.StateDir = filepath.Join(t.TempDir(), "missing")
			},
			wantLevel: Unknown,
			wantLine:  "SYNCTHING UNKNOWN: state error: write state",
		},
		{
			name:   "non-positive timeout is UNKNOWN",
			server: liveLike,
			configure: func(t *testing.T, cfg *Config) {
				cfg.Timeout = 0
			},
			wantLevel: Unknown,
			wantLine:  "SYNCTHING UNKNOWN: timeout must be positive",
		},
		{
			name:   "unwritable state dir keeps CRITICAL",
			server: liveLike,
			configure: func(t *testing.T, cfg *Config) {
				cfg.StateDir = filepath.Join(t.TempDir(), "missing")
			},
			wantLevel: Critical,
			wantLine:  "; state error:",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.server(t))
			defer srv.Close()
			cfg := baseConfig(t, srv.URL)
			if tt.configure != nil {
				tt.configure(t, &cfg)
			}
			got := Run(context.Background(), cfg, now)
			if got.Level != tt.wantLevel {
				t.Errorf("Level = %v, want %v (line %q)", got.Level, tt.wantLevel, got.Line)
			}
			if !strings.Contains(got.Line, tt.wantLine) {
				t.Errorf("Line = %q, want containing %q", got.Line, tt.wantLine)
			}
			assertOneLineNoSecret(t, got.Line)
		})
	}
}

func syncingOnly(t *testing.T) fakeSyncthing {
	return fakeSyncthing{
		folders:  `[{"id":"music-1","label":"Music","paused":false}]`,
		statuses: map[string]string{"music-1": readFixture(t, "status_syncing.json")},
	}
}

func assertOneLineNoSecret(t *testing.T, line string) {
	t.Helper()
	if strings.Contains(line, "\n") {
		t.Errorf("Line %q spans several lines", line)
	}
	if strings.Contains(line, testAPIKey) {
		t.Errorf("Line %q leaks the API key", line)
	}
}

func TestRunUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // nothing listens there any more
	got := Run(context.Background(), baseConfig(t, url), time.Now())
	if got.Level != Unknown || !strings.HasPrefix(got.Line, "SYNCTHING UNKNOWN: GET /rest/config/folders: ") {
		t.Fatalf("got %v %q, want UNKNOWN connection error", got.Level, got.Line)
	}
	assertOneLineNoSecret(t, got.Line)
}

// TestRunDoesNotFollowRedirects guards the API key: a redirect must end the
// check as UNKNOWN instead of sending X-API-Key to the redirect target.
func TestRunDoesNotFollowRedirects(t *testing.T) {
	leaked := make(chan string, 1)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked <- r.Header.Get("X-API-Key")
	}))
	defer target.Close()
	redirector := httptest.NewServer(http.RedirectHandler(target.URL+"/rest/config/folders", http.StatusFound))
	defer redirector.Close()

	got := Run(context.Background(), baseConfig(t, redirector.URL), time.Now())
	if got.Level != Unknown || got.Line != "SYNCTHING UNKNOWN: GET /rest/config/folders: HTTP 302" {
		t.Errorf("got %v %q, want UNKNOWN HTTP 302", got.Level, got.Line)
	}
	select {
	case key := <-leaked:
		t.Fatalf("redirect target was contacted (X-API-Key present: %t)", key != "")
	default:
	}
}

func TestRunTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()
	cfg := baseConfig(t, srv.URL)
	cfg.Timeout = 50 * time.Millisecond
	got := Run(context.Background(), cfg, time.Now())
	if got.Level != Unknown || !strings.Contains(got.Line, "deadline exceeded") {
		t.Fatalf("got %v %q, want UNKNOWN timeout", got.Level, got.Line)
	}
}

// TestRunEscalationAcrossRuns drives the state file through consecutive
// runs: WARNING, still WARNING before the threshold, CRITICAL after it, and
// a reset once the folder is back in sync.
func TestRunEscalationAcrossRuns(t *testing.T) {
	fake := syncingOnly(t)
	srv := httptest.NewServer(&fake)
	defer srv.Close()
	cfg := baseConfig(t, srv.URL)
	cfg.StateDir = t.TempDir()
	cfg.EscalateAfter = time.Hour
	t0 := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)

	steps := []struct {
		at        time.Duration
		status    string
		wantLevel Level
		wantLine  string
	}{
		{0, "status_syncing.json", Warning, "Music syncing need=12 (48.0 MB) out of sync since 0m"},
		{55 * time.Minute, "status_syncing.json", Warning, "out of sync since 55m"},
		{2*time.Hour + 5*time.Minute, "status_syncing.json", Critical, "out of sync since 2h05m (escalated)"},
		{2*time.Hour + 10*time.Minute, "status_idle.json", OK, "SYNCTHING OK: 1 of 1 folders in sync"},
		{3 * time.Hour, "status_syncing.json", Warning, "out of sync since 0m"},
	}
	for _, step := range steps {
		fake.statuses["music-1"] = readFixture(t, step.status)
		got := Run(context.Background(), cfg, t0.Add(step.at))
		if got.Level != step.wantLevel || !strings.Contains(got.Line, step.wantLine) {
			t.Fatalf("at +%v: got %v %q, want %v containing %q", step.at, got.Level, got.Line, step.wantLevel, step.wantLine)
		}
	}
}

func TestRunUnknownKeepsState(t *testing.T) {
	fake := syncingOnly(t)
	srv := httptest.NewServer(&fake)
	defer srv.Close()
	cfg := baseConfig(t, srv.URL)
	cfg.StateDir = t.TempDir()
	cfg.EscalateAfter = time.Hour
	t0 := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)

	if got := Run(context.Background(), cfg, t0); got.Level != Warning {
		t.Fatalf("first run: %v %q", got.Level, got.Line)
	}
	fake.code = http.StatusBadGateway
	if got := Run(context.Background(), cfg, t0.Add(30*time.Minute)); got.Level != Unknown {
		t.Fatalf("outage run: %v %q", got.Level, got.Line)
	}
	fake.code = 0
	got := Run(context.Background(), cfg, t0.Add(90*time.Minute))
	if got.Level != Critical || !strings.Contains(got.Line, "since 1h30m (escalated)") {
		t.Fatalf("after outage: %v %q, want the first-seen time kept", got.Level, got.Line)
	}
}
