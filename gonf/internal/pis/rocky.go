package pis

import (
	"sort"

	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"

	"codeberg.org/snonux/conf/gonf/internal/paths"
)

// rockyTimerSource maps short hostnames to the per-host timer unit file
// (OnCalendar *:05 on pi2, *:35 on pi3 — plan §12).
var rockyTimerSource = map[string]string{
	"pi2": paths.Frontends + "/systemd/unattended-upgrade-rocky.timer.pi2",
	"pi3": paths.Frontends + "/systemd/unattended-upgrade-rocky.timer.pi3",
}

func rockyHosts() []string {
	hosts := make([]string, 0, len(rockyTimerSource))
	for host := range rockyTimerSource {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)
	return hosts
}

// Rocky carries the unattended-upgrade deployment for pi2/pi3.
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

// DescUnattendedKsh returns the description for the ksh package.
func (Rocky) DescUnattendedKsh() string {
	return "Install ksh (Rocky AT&T ksh93) for the unattended-upgrade script"
}

// UnattendedKsh installs ksh via dnf.
func (Rocky) UnattendedKsh() {
	for _, host := range rockyHosts() {
		WhenHostname(host, func() {
			Package("ksh")
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
