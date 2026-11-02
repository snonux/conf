package internal

import (
	"strings"
	"testing"
	"time"
)

var galaxyRule = IgnoreRule{Folder: "Galaxy Tab S10 FE DCIM", MaxItems: 4, MaxBytes: 20226448}

func entry(id, label string, status FolderStatus) Entry {
	return Entry{Folder: Folder{ID: id, Label: label}, Status: status}
}

func TestClassify(t *testing.T) {
	inSync := FolderStatus{State: "idle"}
	tests := []struct {
		name        string
		entries     []Entry
		wantLevel   Level
		wantIgnored int
		wantSummary string
	}{
		{
			name:        "all in sync",
			entries:     []Entry{entry("a", "A", inSync), entry("b", "", FolderStatus{State: "scanning"})},
			wantLevel:   OK,
			wantSummary: "SYNCTHING OK: 2 of 2 folders in sync",
		},
		{
			name:        "syncing transfer is WARNING",
			entries:     []Entry{entry("a", "A", FolderStatus{State: "syncing", NeedTotalItems: 3, NeedBytes: 1500})},
			wantLevel:   Warning,
			wantSummary: "SYNCTHING WARNING: 1 of 1 folders out of sync: A syncing need=3 (1.5 kB)",
		},
		{
			name:        "pull errors are WARNING",
			entries:     []Entry{entry("a", "A", FolderStatus{State: "idle", PullErrors: 2})},
			wantLevel:   Warning,
			wantSummary: "A idle need=0 (0 B) pullErrors=2",
		},
		{
			name:        "errors field counts as pull errors",
			entries:     []Entry{entry("a", "A", FolderStatus{State: "idle", Errors: 1})},
			wantLevel:   Warning,
			wantSummary: "pullErrors=1",
		},
		{
			name:        "stuck idle folder is CRITICAL",
			entries:     []Entry{entry("a", "A", inSync), entry("p", "Photos", FolderStatus{State: "idle", NeedTotalItems: 4, NeedBytes: 20226448})},
			wantLevel:   Critical,
			wantSummary: "SYNCTHING CRITICAL: 1 of 2 folders out of sync: Photos idle need=4 (20.2 MB)",
		},
		{
			name:        "error state is CRITICAL with message",
			entries:     []Entry{entry("a", "A", FolderStatus{State: "error", Error: "folder path missing"})},
			wantLevel:   Critical,
			wantSummary: `A error need=0 (0 B) error="folder path missing"`,
		},
		{
			name:        "known stuck folder within limits is ignored",
			entries:     []Entry{entry("a", "A", inSync), entry("g", "Galaxy Tab S10 FE DCIM", FolderStatus{State: "idle", NeedTotalItems: 4, NeedBytes: 20226448, PullErrors: 4})},
			wantLevel:   OK,
			wantIgnored: 1,
			wantSummary: "SYNCTHING OK: 1 of 2 folders in sync; ignored: Galaxy Tab S10 FE DCIM idle need=4 (20.2 MB) pullErrors=4",
		},
		{
			name:        "known folder beyond its limits alerts again",
			entries:     []Entry{entry("g", "Galaxy Tab S10 FE DCIM", FolderStatus{State: "idle", NeedTotalItems: 5, NeedBytes: 25000000})},
			wantLevel:   Critical,
			wantSummary: "Galaxy Tab S10 FE DCIM idle need=5 (25.0 MB)",
		},
		{
			name:        "ignored folder in error state still alerts",
			entries:     []Entry{entry("g", "Galaxy Tab S10 FE DCIM", FolderStatus{State: "error", NeedTotalItems: 4})},
			wantLevel:   Critical,
			wantSummary: "SYNCTHING CRITICAL",
		},
		{
			name:        "paused folders are skipped",
			entries:     []Entry{{Folder: Folder{ID: "p", Paused: true}}, entry("a", "A", inSync)},
			wantLevel:   OK,
			wantSummary: "SYNCTHING OK: 1 of 2 folders in sync; 1 paused",
		},
		{
			name: "CRITICAL findings come first",
			entries: []Entry{
				entry("w", "W", FolderStatus{State: "syncing", NeedTotalItems: 1}),
				entry("c", "C", FolderStatus{State: "idle", NeedTotalItems: 1}),
			},
			wantLevel:   Critical,
			wantSummary: "out of sync: C idle need=1 (0 B), W syncing need=1 (0 B)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := Classify(tt.entries, []IgnoreRule{galaxyRule})
			if got := report.Level(); got != tt.wantLevel {
				t.Errorf("Level = %v, want %v", got, tt.wantLevel)
			}
			if len(report.Ignored) != tt.wantIgnored {
				t.Errorf("Ignored = %d, want %d", len(report.Ignored), tt.wantIgnored)
			}
			if got := report.Summary(); !strings.Contains(got, tt.wantSummary) {
				t.Errorf("Summary = %q, want containing %q", got, tt.wantSummary)
			}
		})
	}
}

func TestApplyHistoryEscalation(t *testing.T) {
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	syncing := entry("s", "S", FolderStatus{State: "syncing", NeedTotalItems: 2})
	stuck := entry("g", "G", FolderStatus{State: "idle", NeedTotalItems: 4, NeedBytes: 20226448})
	tests := []struct {
		name          string
		entries       []Entry
		age           time.Duration
		escalateAfter time.Duration
		wantLevel     Level
		wantSummary   string
	}{
		{"fresh WARNING stays", []Entry{syncing}, 0, time.Hour, Warning, "S syncing need=2 (0 B) out of sync since 0m"},
		{"WARNING below threshold", []Entry{syncing}, 59 * time.Minute, time.Hour, Warning, "out of sync since 59m"},
		{"WARNING at threshold escalates", []Entry{syncing}, time.Hour, time.Hour, Critical, "out of sync since 1h00m (escalated)"},
		{"escalation disabled", []Entry{syncing}, 5 * time.Hour, 0, Warning, "out of sync since 5h00m"},
		{"stuck shows duration", []Entry{stuck}, 2*time.Hour + 5*time.Minute, time.Hour, Critical, "G idle need=4 (20.2 MB) out of sync since 2h05m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := Classify(tt.entries, nil)
			st := State{FirstSeen: map[string]time.Time{}}.Track(report.OutOfSyncIDs(), now.Add(-tt.age))
			report = report.ApplyHistory(st, now, tt.escalateAfter)
			if got := report.Level(); got != tt.wantLevel {
				t.Errorf("Level = %v, want %v", got, tt.wantLevel)
			}
			if got := report.Summary(); !strings.Contains(got, tt.wantSummary) {
				t.Errorf("Summary = %q, want containing %q", got, tt.wantSummary)
			}
			if tt.wantLevel == Warning && strings.Contains(report.Summary(), "escalated") {
				t.Errorf("Summary = %q, must not say escalated", report.Summary())
			}
		})
	}
}

func TestTruncateLongError(t *testing.T) {
	long := strings.Repeat("é", maxErrorLen+5)
	got := describe(entry("a", "A", FolderStatus{State: "error", Error: long}))
	if !strings.Contains(got, strings.Repeat("é", maxErrorLen)+`..."`) || strings.Contains(got, strings.Repeat("é", maxErrorLen+1)) {
		t.Fatalf("describe = %q, want error cut at %d runes", got, maxErrorLen)
	}
}
