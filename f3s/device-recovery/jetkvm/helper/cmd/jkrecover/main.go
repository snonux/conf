//go:build linux

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"syscall"

	"github.com/snonux/conf/jetkvm-recovery/internal/recovery"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	dir := flag.String("state", "/userdata/jkrecover", "private durable directory")
	config := flag.String("config", "/userdata/kvm_config.json", "vendor config path")
	device := flag.String("device", "", "pinned hardware device ID")
	profilePath := flag.String("profile", "/userdata/jkrecover/profile.json", "root-private management profile")
	version := flag.Bool("version", false, "print version")
	targetProof := flag.Bool("fresh-target", false, "controller attests fresh authenticated literal target session")
	recoveryProof := flag.Bool("fresh-recovery", false, "controller attests fresh authenticated literal recovery session")
	flag.Parse()
	if *version {
		fmt.Println(recovery.Version)
		return nil
	}
	if flag.NArg() < 1 {
		return fmt.Errorf("action required: bootstrap, canary-keep, management-boot, pre-app, arm, confirm, status, worker, canary-worker")
	}
	if os.Geteuid() != 0 {
		return fmt.Errorf("root required")
	}
	if err := recovery.Prerequisites(); err != nil {
		return err
	}
	identity, err := recovery.Identity()
	if err != nil {
		return err
	}
	if identity != *device {
		return fmt.Errorf("hardware identity mismatch")
	}
	boot, err := recovery.BootID()
	if err != nil {
		return err
	}
	// Read the pinned profile privately; network values are never printed.
	data, err := recovery.ReadPrivate(*profilePath)
	if err != nil {
		return fmt.Errorf("management profile must be owned private mode 0600")
	}
	var profile recovery.Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return fmt.Errorf("invalid management profile")
	}
	if profile.Device != *device {
		return fmt.Errorf("profile hardware identity mismatch")
	}
	if err := profile.Validate(); err != nil {
		return err
	}
	store := recovery.Store{Dir: *dir}
	adapter := recovery.Adapter{Store: store, Boot: boot, Profile: profile, Runner: recovery.Native{}, Now: recovery.BootTime}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable")
	}
	engine := recovery.Engine{Store: store, Config: *config, Device: *device, Boot: boot, Now: recovery.BootTime, Management: adapter.Accepted, Reboot: func() error { syscall.Sync(); return syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART) }}
	engine.Start = func(state recovery.State) error {
		return recovery.Launch(executable, *dir, *config, *device, *profilePath, state)
	}
	switch flag.Arg(0) {
	case "bootstrap":
		return recovery.StartCanary(executable, *config, *profilePath, boot, adapter, recovery.BootTime)
	case "canary-keep":
		return recovery.KeepCanary(adapter, boot, *recoveryProof)
	case "management-boot":
		if err := recovery.NoVendorApp(); err != nil {
			return err
		}
		now, err := recovery.BootTime()
		if err != nil {
			return err
		}
		return recovery.ManagementBoot(adapter, boot, now, func(state recovery.State) error {
			return recovery.LaunchCanary(executable, *dir, *config, *device, *profilePath, state)
		})
	case "pre-app":
		if err := recovery.NoVendorApp(); err != nil {
			return err
		}
		engine.Management = adapter.EnsureAccepted
		return engine.PreApp()
	case "arm":
		if flag.NArg() != 3 {
			return fmt.Errorf("arm requires generation and private target-network JSON file")
		}
		target, err := recovery.ReadPrivate(flag.Arg(2))
		if err != nil {
			return fmt.Errorf("target file must be owned private mode 0600")
		}
		return engine.Arm(flag.Arg(1), target)
	case "status":
		if flag.NArg() != 2 {
			return fmt.Errorf("status requires generation")
		}
		status, err := engine.Status(flag.Arg(1))
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(status)
	case "confirm":
		if flag.NArg() != 2 {
			return fmt.Errorf("confirm requires generation")
		}
		return engine.Confirm(flag.Arg(1), *targetProof, *recoveryProof)
	case "worker", "canary-worker":
		if flag.NArg() != 2 {
			return fmt.Errorf("worker requires generation")
		}
		ready := os.NewFile(3, "readiness")
		if ready == nil {
			return fmt.Errorf("readiness descriptor missing")
		}
		defer func() { _ = ready.Close() }()
		if flag.Arg(0) == "canary-worker" {
			return recovery.WorkCanary(adapter, boot, flag.Arg(1), recovery.BootTime, ready)
		}
		return recovery.Work(engine, flag.Arg(1), ready)
	default:
		return fmt.Errorf("unknown action")
	}
}
