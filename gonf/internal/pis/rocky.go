package pis

import (
	"sort"

	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"

	"codeberg.org/snonux/conf/gonf/internal/paths"
)

// rockyTimerSource maps short hostnames to the per-host timer unit file
// (plan §6/§12: pi2/r0 *:05, r1 *:25, pi3 *:35, r2 *:45).
var rockyTimerSource = map[string]string{
	"pi2": paths.Frontends + "/systemd/unattended-upgrade-rocky.timer.pi2",
	"pi3": paths.Frontends + "/systemd/unattended-upgrade-rocky.timer.pi3",
	"r0":  paths.Frontends + "/systemd/unattended-upgrade-rocky.timer.r0",
	"r1":  paths.Frontends + "/systemd/unattended-upgrade-rocky.timer.r1",
	"r2":  paths.Frontends + "/systemd/unattended-upgrade-rocky.timer.r2",
}

func rockyHosts() []string {
	hosts := make([]string, 0, len(rockyTimerSource))
	for host := range rockyTimerSource {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)
	return hosts
}

// Rocky carries the unattended-upgrade deployment for all Rocky hosts:
// pi2/pi3 and the k3s nodes r0/r1/r2.
type Rocky struct {
	RequiresRoot
}

// DescUnattendedGonfLink returns the description for the sudo PATH link.
func (Rocky) DescUnattendedGonfLink() string {
	return "Symlink /usr/bin/gonf → /usr/local/bin/gonf (sudo secure_path)"
}

// UnattendedGonfLink puts gonf on sudo's secure_path so privileged push
// works (sudo -n gonf …). The bootstrap installs to /usr/local/bin.
func (Rocky) UnattendedGonfLink() {
	for _, host := range rockyHosts() {
		WhenHostname(host, func() {
			Link("/usr/bin/gonf", WithSymlink("/usr/local/bin/gonf"))
		})
	}
}

// DescUnattendedPackages returns the description for required packages.
func (Rocky) DescUnattendedPackages() string {
	return "Install ksh + yum-utils (needs-restarting) for unattended-upgrade"
}

// UnattendedPackages installs ksh (script interpreter) and yum-utils
// (needs-restarting -s/-r).
func (Rocky) UnattendedPackages() {
	for _, host := range rockyHosts() {
		WhenHostname(host, func() {
			Package("ksh")
			Package("yum-utils")
		})
	}
}

// DescUnattendedScript returns the description for the wrapper deployment.
func (Rocky) DescUnattendedScript() string {
	return "Install /usr/local/sbin/unattended-upgrade-rocky (0755 root:root)"
}

// UnattendedScript installs the Rocky ksh wrapper.
func (Rocky) UnattendedScript() {
	for _, host := range rockyHosts() {
		WhenHostname(host, func() {
			dir := EnsureDir("/usr/local/sbin",
				WithMode(0o755), WithOwner("root"), WithGroup("root"))
			InstallFile("/usr/local/sbin/unattended-upgrade-rocky",
				paths.Frontends+"/scripts/unattended-upgrade-rocky.sh",
				WithMode(0o755), WithOwner("root"), WithGroup("root"),
				DependsOn(dir))
		})
	}
}

// DescUnattendedStampDir returns the description for the stamp directory.
func (Rocky) DescUnattendedStampDir() string {
	return "Ensure /var/lib/unattended-upgrade stamp directory"
}

// UnattendedStampDir creates the persistent stamp directory.
func (Rocky) UnattendedStampDir() {
	for _, host := range rockyHosts() {
		WhenHostname(host, func() {
			EnsureDir("/var/lib/unattended-upgrade",
				WithMode(0o700), WithOwner("root"), WithGroup("root"))
		})
	}
}

// DescUnattendedUnits returns the description for systemd unit files.
func (Rocky) DescUnattendedUnits() string {
	return "Install unattended-upgrade-rocky.service + per-host .timer"
}

// UnattendedUnits installs the oneshot service and per-host timer, then
// reloads systemd and enables the timer.
func (Rocky) UnattendedUnits() {
	svcSrc := paths.Frontends + "/systemd/unattended-upgrade-rocky.service"
	for _, host := range rockyHosts() {
		timerSrc := rockyTimerSource[host]
		WhenHostname(host, func() {
			svc := InstallFile("/etc/systemd/system/unattended-upgrade-rocky.service",
				svcSrc,
				WithMode(0o644), WithOwner("root"), WithGroup("root"))
			timer := InstallFile("/etc/systemd/system/unattended-upgrade-rocky.timer",
				timerSrc,
				WithMode(0o644), WithOwner("root"), WithGroup("root"))
			reload := DaemonReload(DependsOn(svc, timer), IfChanged)
			Timer("unattended-upgrade-rocky", DependsOn(reload))
		})
	}
}

// DescUnattendedLogrotate returns the description for logrotate.
func (Rocky) DescUnattendedLogrotate() string {
	return "Install /etc/logrotate.d/unattended-upgrade"
}

// UnattendedLogrotate installs the logrotate snippet.
func (Rocky) UnattendedLogrotate() {
	src := paths.Frontends + "/systemd/unattended-upgrade.logrotate"
	for _, host := range rockyHosts() {
		WhenHostname(host, func() {
			InstallFile("/etc/logrotate.d/unattended-upgrade",
				src,
				WithMode(0o644), WithOwner("root"), WithGroup("root"))
		})
	}
}
