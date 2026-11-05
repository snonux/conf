//go:build linux

package recovery

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// WorkerIdentity binds readiness to the exact generation, boot and process birth.
type WorkerIdentity struct {
	Generation string `json:"generation"`
	Boot       string `json:"boot"`
	Deadline   int64  `json:"deadline_boottime_ns"`
	PID        int    `json:"pid"`
	Birth      string `json:"proc_start_ticks"`
}

// Launch starts a session detached from SSH and checks the child's readiness pipe.
// The child uses the saved deadline even when Launch replaces a previous worker.
func Launch(executable, dir, config, device, profile string, state State) error {
	return launch(executable, dir, config, device, profile, "worker", state)
}

func launch(executable, dir, config, device, profile, action string, state State) error {
	read, write, err := os.Pipe()
	if err != nil {
		return errors.New("create readiness pipe")
	}
	defer func() { _ = read.Close() }()
	defer func() { _ = write.Close() }()
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer func() { _ = null.Close() }()
	cmd := exec.Command(executable, "-state", dir, "-config", config, "-device", device, "-profile", profile, action, state.Generation)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin = null
	cmd.Stdout = null
	cmd.Stderr = null
	cmd.ExtraFiles = []*os.File{write}
	if err := cmd.Start(); err != nil {
		return errors.New("start detached worker")
	}
	// Reap failures; a successful long-lived worker is orphaned to init when
	// the short-lived CLI exits. Tests may keep the launching process alive.
	defer func() { go func() { _ = cmd.Wait() }() }()
	if err := write.Close(); err != nil {
		_ = cmd.Process.Kill()
		return errors.New("close parent readiness writer")
	}
	type result struct {
		identity WorkerIdentity
		err      error
	}
	ready := make(chan result, 1)
	go func() {
		var identity WorkerIdentity
		err := json.NewDecoder(io.LimitReader(read, 2048)).Decode(&identity)
		ready <- result{identity, err}
	}()
	select {
	case response := <-ready:
		identity := response.identity
		if response.err != nil || identity.PID != cmd.Process.Pid || identity.Generation != state.Generation || identity.Boot != state.Boot || identity.Deadline != state.Deadline {
			_ = cmd.Process.Kill()
			return errors.New("invalid worker readiness")
		}
		birth, err := processBirth(identity.PID)
		if err != nil || birth != identity.Birth {
			_ = cmd.Process.Kill()
			return errors.New("worker exited before readiness")
		}
		encoded, err := json.Marshal(identity)
		if err != nil {
			return errors.New("encode worker identity")
		}
		if err := Atomic(filepath.Join(dir, "worker.json"), encoded); err != nil {
			_ = cmd.Process.Kill()
			return err
		}
		return nil
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		return errors.New("worker readiness timeout")
	}
}

// Work checks the durable deadline before readiness and before every expiry attempt.
// Failed reboot requests retain restoring state and retry; no config is written live.
func Work(e Engine, generation string, ready *os.File) error {
	state, err := e.checked(generation)
	if err != nil {
		return err
	}
	if state.Boot != e.Boot || (state.Phase != "armed" && state.Phase != "trial" && state.Phase != "restoring") {
		return errors.New("worker cannot guard this state")
	}
	now, err := e.Now()
	if err != nil {
		return err
	}
	if now >= state.Deadline {
		return errors.New("worker started after deadline")
	}
	birth, err := processBirth(os.Getpid())
	if err != nil {
		return err
	}
	identity := WorkerIdentity{generation, e.Boot, state.Deadline, os.Getpid(), birth}
	if err := json.NewEncoder(ready).Encode(identity); err != nil {
		return errors.New("publish worker readiness")
	}
	if err := ready.Close(); err != nil {
		return errors.New("close worker readiness")
	}
	for {
		state, err = e.Store.Load()
		if err != nil {
			return err
		}
		if state.Device != e.Device || state.Generation != generation || state.Boot != e.Boot || state.Phase == "confirmed" || state.Phase == "restored" {
			return nil
		}
		now, err = e.Now()
		if err != nil {
			_ = e.Expire(generation)
			time.Sleep(time.Second)
			continue
		}
		if now >= state.Deadline || state.Phase == "restoring" {
			if err := e.Expire(generation); err != nil {
				time.Sleep(time.Second)
				continue
			}
			// A successful reboot syscall should terminate us; retry if the kernel stays up.
			time.Sleep(time.Second)
			continue
		}
		remaining := time.Duration(state.Deadline - now)
		if remaining > time.Second {
			remaining = time.Second
		}
		time.Sleep(remaining)
	}
}

func processBirth(pid int) (string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", errors.New("read worker process birth")
	}
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 {
		return "", errors.New("invalid process identity")
	}
	fields := strings.Fields(string(data[end+1:]))
	if len(fields) < 20 || fields[0] == "Z" || fields[0] == "X" {
		return "", errors.New("incomplete process identity")
	}
	if _, err := strconv.ParseUint(fields[19], 10, 64); err != nil {
		return "", errors.New("invalid process birth")
	}
	return fields[19], nil
}

// LaunchCanary starts the bounded additive worker with the same checked handshake.
func LaunchCanary(executable, dir, config, device, profile string, state State) error {
	return launch(executable, dir, config, device, profile, "canary-worker", state)
}
