//go:build linux

package recovery

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// Store holds private transaction data; lock lives on a separate, stable inode.
type Store struct{ Dir string }

// Locked serializes all commands including atomic replacements of state.json.
func (s Store) Locked(fn func() error) error {
	if err := os.MkdirAll(s.Dir, 0700); err != nil {
		return errors.New("create private state directory")
	}
	info, err := os.Lstat(s.Dir)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 || !ownedByEffectiveUID(info) {
		return errors.New("state directory must be a real private directory")
	}
	fd, err := syscall.Open(filepath.Join(s.Dir, "lock"), syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return errors.New("open transaction lock")
	}
	file := os.NewFile(uintptr(fd), "lock")
	defer func() { _ = file.Close() }()
	lockInfo, err := file.Stat()
	if err != nil || !lockInfo.Mode().IsRegular() || lockInfo.Mode().Perm() != 0600 || !ownedByEffectiveUID(lockInfo) {
		return errors.New("lock must be private owned regular file")
	}
	if err := syscall.Flock(fd, syscall.LOCK_EX); err != nil {
		return errors.New("lock transaction")
	}
	defer func() { _ = syscall.Flock(fd, syscall.LOCK_UN) }()
	return fn()
}

// Load never includes private file contents in errors.
func (s Store) Load() (State, error) {
	var state State
	if err := readPrivate(filepath.Join(s.Dir, "state.json"), &state); err != nil {
		return state, err
	}
	if err := state.Validate(); err != nil {
		return state, err
	}
	return state, nil
}

// Save syncs the file and containing directory before acknowledging a transition.
func (s Store) Save(state State) error {
	if err := state.Validate(); err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return errors.New("encode private state")
	}
	return Atomic(filepath.Join(s.Dir, "state.json"), data)
}

// Atomic replaces a regular private file without following a destination symlink.
func Atomic(path string, data []byte) error {
	return atomicFile(path, data, func(string) error { return nil })
}

func atomicFile(path string, data []byte, checkpoint func(string) error) error {
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return errors.New("refuse non-regular destination")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.New("inspect destination")
	}
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, ".jkrecover-*")
	if err != nil {
		return errors.New("create private temporary file")
	}
	name := file.Name()
	defer func() { _ = os.Remove(name) }()
	defer func() { _ = file.Close() }()
	if err := file.Chmod(0600); err != nil {
		return errors.New("protect temporary file")
	}
	if _, err := file.Write(data); err != nil {
		return errors.New("write private file")
	}
	if err := checkpoint("written"); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return errors.New("sync private file")
	}
	if err := file.Close(); err != nil {
		return errors.New("close private file")
	}
	if err := checkpoint("synced"); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		return errors.New("replace private file")
	}
	if err := checkpoint("renamed"); err != nil {
		return err
	}
	directory, err := os.Open(dir)
	if err != nil {
		return errors.New("open persistence directory")
	}
	defer func() { _ = directory.Close() }()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync persistence directory: %w", err)
	}
	return nil
}

func ownedByEffectiveUID(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Geteuid())
}
