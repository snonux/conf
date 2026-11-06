//go:build linux

package recovery

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// deferredLocked uses the existing stable lock inode, with cancellable bounded
// acquisition only for the new role. Legacy Store.Locked semantics are intact.
func deferredLocked(ctx context.Context, store Store, fn func() error) error {
	info, err := os.Lstat(store.Dir)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 || !ownedByEffectiveUID(info) {
		return errors.New("deferred state directory must already be private")
	}
	fd, err := syscall.Open(filepath.Join(store.Dir, "lock"), syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return errors.New("open deferred transaction lock")
	}
	file := os.NewFile(uintptr(fd), "lock")
	defer func() { _ = file.Close() }()
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || !ownedByEffectiveUID(info) {
		return errors.New("deferred lock must be private owned regular file")
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			return errors.New("acquire deferred transaction lock")
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	defer func() { _ = syscall.Flock(fd, syscall.LOCK_UN) }()
	return fn()
}

// LaunchManagementWaiter creates a separate session and separate identity file;
// worker.json remains exclusively the independent canary watchdog's receipt.
// Timeout/error kills and reaps this child. A successful child has its own
// durable <=30s deadline and cannot Ensure without the published identity.
func LaunchManagementWaiter(executable, dir, config, device, profile string, state State) error {
	read, write, err := os.Pipe()
	if err != nil {
		return errors.New("create management waiter pipe")
	}
	defer func() { _ = read.Close() }()
	defer func() { _ = write.Close() }()
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer func() { _ = null.Close() }()
	command := exec.Command(executable, "-state", dir, "-config", config, "-device", device, "-profile", profile, "management-waiter", state.Generation)
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	command.Stdin = null
	command.Stdout = null
	command.Stderr = null
	command.ExtraFiles = []*os.File{write}
	if err := command.Start(); err != nil {
		return errors.New("start detached management waiter")
	}
	success := false
	defer func() {
		if !success {
			_ = command.Process.Kill()
			_ = command.Wait()
			return
		}
		go func() { _ = command.Wait() }()
	}()
	if err := write.Close(); err != nil {
		return errors.New("close management waiter writer")
	}
	type response struct {
		identity WorkerIdentity
		err      error
	}
	received := make(chan response, 1)
	go func() {
		var identity WorkerIdentity
		err := json.NewDecoder(io.LimitReader(read, 2048)).Decode(&identity)
		received <- response{identity, err}
	}()
	select {
	case result := <-received:
		identity := result.identity
		if result.err != nil || identity.PID != command.Process.Pid || identity.Generation != state.Generation || identity.Boot != state.Boot || identity.Deadline != state.Deadline {
			return errors.New("management waiter readiness mismatch")
		}
		birth, err := processBirth(identity.PID)
		if err != nil || birth != identity.Birth {
			return errors.New("management waiter exited before readiness")
		}
		if err := deferredSave(Store{Dir: dir}, deferredWaiterFile, identity); err != nil {
			return err
		}
		success = true
		return nil
	case <-time.After(3 * time.Second):
		return errors.New("management waiter readiness timeout")
	}
}

// WorkManagementWaiter publishes only its process identity before waiting. It
// never publishes network readiness; the continuation separately verifies that
// this exact identity was durably recorded by its launcher before any Ensure.
func WorkManagementWaiter(ctx context.Context, a Adapter, generation string, ready *os.File) error {
	if a.Now == nil {
		return errors.New("management waiter clock required")
	}
	var claim DeferredClaim
	if err := readPrivate(filepath.Join(a.Store.Dir, deferredClaimFile), &claim); err != nil {
		return err
	}
	tick, err := a.Now()
	if err != nil {
		return err
	}
	if claim.Generation != generation || claim.Boot != a.Boot || claim.Hash != a.Profile.Hash() || claim.Attempt != "waiter-attempted" || tick >= claim.WaitUntil {
		return errors.New("stale management waiter")
	}
	birth, err := processBirth(os.Getpid())
	if err != nil {
		return err
	}
	identity := WorkerIdentity{Generation: generation, Boot: a.Boot, Deadline: claim.Deadline, PID: os.Getpid(), Birth: birth}
	if err := json.NewEncoder(ready).Encode(identity); err != nil {
		return errors.New("publish management waiter identity")
	}
	if err := ready.Close(); err != nil {
		return err
	}
	return continueDeferredManagement(ctx, a, nativeDeferredOps(a), identity)
}

// LaunchDeferredCanary uses the existing independent watchdog handshake and
// worker.json identity, selecting only the deferred role's terminal cleanup.
func LaunchDeferredCanary(executable, dir, config, device, profile string, state State) error {
	return launch(executable, dir, config, device, profile, "deferred-canary-worker", state)
}
