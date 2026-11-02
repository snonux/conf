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
	st, err := readStateFile[State](dir, stateFileName)
	if st.FirstSeen == nil {
		st.FirstSeen = map[string]time.Time{}
	}
	return st, err
}

// Save writes the state atomically to dir.
func (s State) Save(dir string) error {
	return writeStateFile(dir, stateFileName, s)
}

// readStateFile decodes dir/name. A missing or corrupt file gives the zero
// value and no error, so it cannot wedge the check; only a read failure is
// an error.
func readStateFile[T any](dir, name string) (T, error) {
	var zero, v T
	data, err := os.ReadFile(filepath.Join(dir, name))
	if errors.Is(err, fs.ErrNotExist) {
		return zero, nil
	}
	if err != nil {
		return zero, fmt.Errorf("read state: %w", err)
	}
	if json.Unmarshal(data, &v) != nil {
		return zero, nil
	}
	return v, nil
}

// writeStateFile writes v to dir/name atomically (temporary file plus
// rename).
func writeStateFile(dir, name string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	tmp, err := os.CreateTemp(dir, name+".*")
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
	if err := os.Rename(tmp.Name(), filepath.Join(dir, name)); err != nil {
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
