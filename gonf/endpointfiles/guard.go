// Package endpointfiles guards the homelab's named endpoint configuration files.
package endpointfiles

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

// Helper is shared by the existing owners; it never changes network addresses.
const Helper = "/usr/local/sbin/gonf-endpoint-migration"

// Recovery installs host-local expiry/boot recovery for a named service profile.
type Recovery struct {
	RequiresRoot
	Platform string
	// InstallDirectory supports Fedora's sbin -> bin alias. Runtime paths
	// remain stable; validation writes directly into the real directory.
	InstallDirectory string
}

type trial struct {
	Profile string `json:"profile"`
	Seconds uint   `json:"seconds"`
}

func (Recovery) OptsPrepare() TaskOptions  { return TaskOptions{Operational()} }
func (Recovery) OptsArm() TaskOptions      { return TaskOptions{Operational()} }
func (Recovery) OptsApply() TaskOptions    { return TaskOptions{Operational()} }
func (Recovery) OptsConfirm() TaskOptions  { return TaskOptions{Operational()} }
func (Recovery) OptsRollback() TaskOptions { return TaskOptions{Operational()} }
func (Recovery) OptsStatus() TaskOptions   { return TaskOptions{Operational()} }

// Prepare installs independent expiry and early file restoration without activation.
func (g Recovery) Prepare() {
	state := "/var/db/gonf-endpoint-migration"
	if g.Platform == "linux" {
		state = "/var/lib/gonf-endpoint-migration"
	}
	check := Command("test", List("!", "-d", state+"/pending"))
	locking := Command("/bin/sh", List("-ec", `case "$(uname -s)" in Linux|NetBSD) command -v flock >/dev/null ;; FreeBSD|OpenBSD) perl -MFcntl=:flock -e 'exit 0' ;; *) exit 1 ;; esac`))
	installDirectory := g.InstallDirectory
	if installDirectory == "" {
		installDirectory = "/usr/local/sbin"
	}
	EnsureDir(installDirectory, RootOwned)
	helper := InstallFile(filepath.Join(installDirectory, filepath.Base(Helper)), paths.GonfAsset("endpointfiles", "guard"), RootExec,
		WithValidation("/bin/sh", List("-n", CandidatePath)), DependsOn(check, locking))
	setup := Sh(Helper+" setup", DependsOn(helper))
	CronAt("gonf-endpoint-migration", "* * * * *", Helper+" watchdog", DependsOn(setup))
	switch g.Platform {
	case "linux":
		unit := File("/etc/systemd/system/gonf-endpoint-boot.service", RootOwned,
			WithContent("[Unit]\nDescription=Restore unconfirmed endpoint files\nDefaultDependencies=no\nAfter=local-fs.target\nBefore=network-pre.target NetworkManager.service docker.service\n[Service]\nType=oneshot\nExecStart="+Helper+" boot\n[Install]\nWantedBy=sysinit.target\n"), DependsOn(setup))
		reload := Sh("systemctl daemon-reload", OnChange(unit))
		Command("systemctl", List("enable", "gonf-endpoint-boot.service"), DependsOn(reload),
			Unless("systemctl", List("is-enabled", "--quiet", "gonf-endpoint-boot.service")))
	case "freebsd", "netbsd":
		require, before := "FILESYSTEMS hostname", "netif routing"
		if g.Platform == "netbsd" {
			require, before = "network", "npf NETWORKING"
		}
		File("/etc/rc.d/gonf_endpoint_migration", RootExec, DependsOn(setup),
			WithContent("#!/bin/sh\n# PROVIDE: gonf_endpoint_migration\n# REQUIRE: "+require+"\n# BEFORE: "+before+"\n# KEYWORD: nojail\n. /etc/rc.subr\nname=gonf_endpoint_migration\nrcvar=$name\nstart_cmd=\""+Helper+" boot\"\nstop_cmd=\":\"\nload_rc_config \"$name\"\nrun_rc_command \"$1\"\n"))
		EnsureDir("/etc/rc.conf.d", RootOwned)
		File("/etc/rc.conf.d/gonf_endpoint_migration", WithContent("gonf_endpoint_migration=YES\n"), RootOwned)
	case "openbsd":
		Sh(Helper+" install-openbsd-boot", DependsOn(setup))
	default:
		Refuse("endpoint guard", g.Platform, fmt.Errorf("unsupported platform"))
	}
}

// Arm snapshots only the profile's owned files under a fixed host-local deadline.
func (Recovery) Arm() { invoke("arm") }

// Apply publishes staged candidates only while their original lease is valid.
func (Recovery) Apply() { invoke("apply") }

// Confirm accepts a profile after separate fresh service and management checks.
func (Recovery) Confirm() { invoke("confirm") }

// Rollback restores saved files and the previously active service runtime.
func (Recovery) Rollback() { Sh(Helper + " rollback") }

// Status reports the profile/deadline without reading configuration values.
func (Recovery) Status() { Sh(Helper + " status") }

func invoke(action string) {
	var t trial
	decoder := json.NewDecoder(strings.NewReader(os.Getenv("GONF_ENDPOINT_TRIAL")))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&t); err != nil {
		Refuse("endpoint trial", action, fmt.Errorf("valid GONF_ENDPOINT_TRIAL required"))
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		Refuse("endpoint trial", action, fmt.Errorf("trailing trial data"))
		return
	}
	switch t.Profile {
	case "hosts", "ups", "garage", "dtail", "controller", "fan", "pins", "dns":
	default:
		Refuse("endpoint trial", action, fmt.Errorf("unsupported profile"))
		return
	}
	if t.Seconds == 0 {
		t.Seconds = 900
	}
	if t.Seconds < 60 || t.Seconds > 1800 {
		Refuse("endpoint trial", action, fmt.Errorf("window must be 60..1800 seconds"))
		return
	}
	args := List(action, t.Profile)
	if action == "arm" {
		args = append(args, strconv.FormatUint(uint64(t.Seconds), 10))
	}
	Command(Helper, args)
}
