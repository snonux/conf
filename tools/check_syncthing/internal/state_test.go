package internal

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateRoundTripAndTrack(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)

	st, err := LoadState(dir)
	if err != nil || len(st.FirstSeen) != 0 {
		t.Fatalf("missing file: %+v, %v", st, err)
	}
	st = st.Track([]string{"a", "b"}, t0)
	if err := st.Save(dir); err != nil {
		t.Fatal(err)
	}
	st, err = LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	// a stays out of sync (keeps t0), b recovers (dropped), c is new.
	st = st.Track([]string{"a", "c"}, t0.Add(time.Hour))
	want := map[string]time.Time{"a": t0, "c": t0.Add(time.Hour)}
	if len(st.FirstSeen) != len(want) {
		t.Fatalf("FirstSeen = %v, want %v", st.FirstSeen, want)
	}
	for id, seen := range want {
		if !st.FirstSeen[id].Equal(seen) {
			t.Fatalf("FirstSeen[%s] = %v, want %v", id, st.FirstSeen[id], seen)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("state dir holds %v (%v), want only the state file", entries, err)
	}
}

func TestLoadStateCorruptIsEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, stateFileName), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := LoadState(dir)
	if err != nil || st.FirstSeen == nil || len(st.FirstSeen) != 0 {
		t.Fatalf("corrupt file: %+v, %v", st, err)
	}
}

func TestSaveStateMissingDir(t *testing.T) {
	st := State{FirstSeen: map[string]time.Time{}}
	if err := st.Save(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("want error for a missing state directory")
	}
}
