package internal

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Level is a Nagios plugin result; its value is the process exit code.
type Level int

// Nagios/Gogios result levels in exit code order.
const (
	OK Level = iota
	Warning
	Critical
	Unknown
)

// maxErrorLen bounds a folder error message quoted in the summary line.
const maxErrorLen = 80

// Entry is one configured folder with its current database status.
type Entry struct {
	Folder Folder
	Status FolderStatus
}

// Finding is an out-of-sync folder and how it is judged.
type Finding struct {
	Entry
	Level Level
	// Tracked is set when a state directory supplied Since, how long the
	// folder has been out of sync.
	Tracked   bool
	Since     time.Duration
	Escalated bool
}

// Report is the classification of all folders.
type Report struct {
	Total    int
	Paused   int
	Findings []Finding
	Ignored  []Entry
}

// String returns the level name used in the summary line.
func (l Level) String() string {
	switch l {
	case OK:
		return "OK"
	case Warning:
		return "WARNING"
	case Critical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// Classify judges every folder without any history:
//
//   - paused folders and folders with nothing to pull are fine;
//   - a folder matching an ignore rule (within its limits) is ignored,
//     unless it is in state "error";
//   - state "error", or "idle" with needTotalItems > 0 (stuck, the source
//     is gone), is CRITICAL;
//   - any other need or pull errors (a transfer in progress) is WARNING.
func Classify(entries []Entry, rules []IgnoreRule) Report {
	report := Report{Total: len(entries)}
	for _, e := range entries {
		switch {
		case e.Folder.Paused:
			report.Paused++
		case !outOfSync(e.Status):
		case e.Status.State != "error" && ignored(e, rules):
			report.Ignored = append(report.Ignored, e)
		default:
			report.Findings = append(report.Findings, Finding{Entry: e, Level: folderLevel(e.Status)})
		}
	}
	return report
}

// OutOfSyncIDs lists the folder IDs of all findings, for State.Track.
func (r Report) OutOfSyncIDs() []string {
	ids := make([]string, 0, len(r.Findings))
	for _, f := range r.Findings {
		ids = append(ids, f.Folder.ID)
	}
	return ids
}

// ApplyHistory sets each finding's duration from the tracked state and
// escalates a WARNING that has lasted at least escalateAfter to CRITICAL
// (escalateAfter <= 0 disables escalation).
func (r Report) ApplyHistory(st State, now time.Time, escalateAfter time.Duration) Report {
	findings := make([]Finding, len(r.Findings))
	for i, f := range r.Findings {
		if seen, ok := st.FirstSeen[f.Folder.ID]; ok {
			f.Tracked, f.Since = true, now.Sub(seen)
		}
		if f.Level == Warning && escalateAfter > 0 && f.Since >= escalateAfter {
			f.Level, f.Escalated = Critical, true
		}
		findings[i] = f
	}
	r.Findings = findings
	return r
}

// Level is the worst level of all findings.
func (r Report) Level() Level {
	level := OK
	for _, f := range r.Findings {
		level = max(level, f.Level)
	}
	return level
}

// Summary renders the one-line plugin output.
func (r Report) Summary() string {
	var b strings.Builder
	level := r.Level()
	if level == OK {
		fmt.Fprintf(&b, "SYNCTHING OK: %d of %d folders in sync", r.Total-r.Paused-len(r.Ignored), r.Total)
	} else {
		fmt.Fprintf(&b, "SYNCTHING %s: %d of %d folders out of sync: ", level, len(r.Findings), r.Total)
		b.WriteString(joinFindings(r.sortedFindings()))
	}
	if r.Paused > 0 {
		fmt.Fprintf(&b, "; %d paused", r.Paused)
	}
	if len(r.Ignored) > 0 {
		names := make([]string, 0, len(r.Ignored))
		for _, e := range r.Ignored {
			names = append(names, describe(e))
		}
		b.WriteString("; ignored: " + strings.Join(names, ", "))
	}
	return b.String()
}

func (r Report) sortedFindings() []Finding {
	sorted := append([]Finding(nil), r.Findings...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Level > sorted[j].Level })
	return sorted
}

func joinFindings(findings []Finding) string {
	parts := make([]string, 0, len(findings))
	for _, f := range findings {
		text := describe(f.Entry)
		if f.Tracked {
			text += " out of sync since " + humanDuration(f.Since)
		}
		if f.Escalated {
			text += " (escalated)"
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, ", ")
}

// describe renders "<name> <state> need=<items> (<bytes>)" plus pull
// errors and the folder error when present.
func describe(e Entry) string {
	name := e.Folder.Label
	if name == "" {
		name = e.Folder.ID
	}
	text := fmt.Sprintf("%s %s need=%d (%s)", name, e.Status.State, e.Status.NeedTotalItems, humanBytes(e.Status.NeedBytes))
	if n := pullErrors(e.Status); n > 0 {
		text += fmt.Sprintf(" pullErrors=%d", n)
	}
	if e.Status.Error != "" {
		text += fmt.Sprintf(" error=%q", truncate(e.Status.Error, maxErrorLen))
	}
	return text
}

func outOfSync(s FolderStatus) bool {
	return s.NeedTotalItems > 0 || s.NeedBytes > 0 || pullErrors(s) > 0 || s.State == "error"
}

func ignored(e Entry, rules []IgnoreRule) bool {
	for _, rule := range rules {
		if rule.Matches(e.Folder, e.Status) {
			return true
		}
	}
	return false
}

func folderLevel(s FolderStatus) Level {
	if s.State == "error" || (s.State == "idle" && s.NeedTotalItems > 0) {
		return Critical
	}
	return Warning
}

// pullErrors prefers pullErrors and falls back to errors, the name newer
// Syncthing releases also report.
func pullErrors(s FolderStatus) int64 {
	return max(s.PullErrors, s.Errors)
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
