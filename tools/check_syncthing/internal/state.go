package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// stateFileName is the file kept inside the -state-dir directory.
const stateFileName = "check_syncthing.json"

// State maps a folder ID to the time its out-of-sync condition was first
// seen. A folder is dropped again as soon as it is back in sync.
type State struct {
	FirstSeen map[string]time.Time `json:"firstSeen"`
}

// LoadState reads the state from dir. A missing file is an empty state;
// an unreadable directory is an error. A corrupt file is discarded (an
// empty state that the next save overwrites) so it cannot wedge the check.
func LoadState(dir string) (State, error) {
	empty := State{FirstSeen: map[string]time.Time{}}
	data, err := os.ReadFile(filepath.Join(dir, stateFileName))
	if errors.Is(err, fs.ErrNotExist) {
		return empty, nil
	}
	if err != nil {
		return empty, fmt.Errorf("read state: %w", err)
	}
	var st State
	if json.Unmarshal(data, &st) != nil || st.FirstSeen == nil {
		return empty, nil
	}
	return st, nil
}

// Save writes the state atomically (temporary file plus rename) to dir.
func (s State) Save(dir string) error {
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	tmp, err := os.CreateTemp(dir, stateFileName+".*")
	if err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, stateFileName)); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	return nil
}

// Track records now as the first-seen time for each out-of-sync folder not
// already tracked and forgets every folder that is not out of sync.
func (s State) Track(outOfSync []string, now time.Time) State {
	next := State{FirstSeen: make(map[string]time.Time, len(outOfSync))}
	for _, id := range outOfSync {
		if seen, ok := s.FirstSeen[id]; ok {
			next.FirstSeen[id] = seen
			continue
		}
		next.FirstSeen[id] = now
	}
	return next
}
