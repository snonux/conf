// Package rocky declares Rocky Linux host tasks for the conf repository —
// unattended upgrades for pi2/pi3 and k3s nodes r0–r2 per
// frontends/docs/unattended-upgrades-pi.plan.md.
package rocky

import (
	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"

	"codeberg.org/snonux/conf/gonf/internal/fleet"
	"codeberg.org/snonux/conf/gonf/internal/paths"
)

// Unattended carries the unattended-upgrade deployment for all Rocky hosts.
// Register with WithFleet(fleet.NameRockyAll). Per-host OnCalendar lives on
// each Host via WithValue(fleet.ValueUnattendedOnCalendar, …).
type Unattended struct {
	RequiresRoot
}

// DescUnattendedGonfLink returns the description for the sudo PATH link.
func (Unattended) DescUnattendedGonfLink() string {
	return "Symlink /usr/bin/gonf → /usr/local/bin/gonf (sudo secure_path)"
}

// UnattendedGonfLink puts gonf on sudo's secure_path so privileged push
// works (sudo -n gonf …). The bootstrap installs to /usr/local/bin.
func (Unattended) UnattendedGonfLink() {
	WhenHostname(FleetHosts(), func() {
		Link("/usr/bin/gonf", WithSymlink("/usr/local/bin/gonf"))
	})
}

// DescUnattendedPackages returns the description for required packages.
func (Unattended) DescUnattendedPackages() string {
	return "Install ksh + yum-utils (needs-restarting) for unattended-upgrade"
}

// UnattendedPackages installs ksh (script interpreter) and yum-utils
// (needs-restarting -s/-r).
func (Unattended) UnattendedPackages() {
	WhenHostname(FleetHosts(), func() {
		Package("ksh")
		Package("yum-utils")
	})
}

// DescUnattendedScript returns the description for the wrapper deployment.
func (Unattended) DescUnattendedScript() string {
	return "Install /usr/local/sbin/unattended-upgrade-rocky (0755 root:root)"
}

// UnattendedScript installs the Rocky ksh wrapper.
func (Unattended) UnattendedScript() {
	WhenHostname(FleetHosts(), func() {
		dir := EnsureDir("/usr/local/sbin",
			WithMode(0o755), WithOwner("root"), WithGroup("root"))
		InstallFile("/usr/local/sbin/unattended-upgrade-rocky",
			paths.Frontends+"/scripts/unattended-upgrade-rocky.sh",
			WithMode(0o755), WithOwner("root"), WithGroup("root"),
			DependsOn(dir))
	})
}

// DescUnattendedStampDir returns the description for the stamp directory.
func (Unattended) DescUnattendedStampDir() string {
	return "Ensure /var/lib/unattended-upgrade stamp directory"
}

// UnattendedStampDir creates the persistent stamp directory.
func (Unattended) UnattendedStampDir() {
	WhenHostname(FleetHosts(), func() {
		EnsureDir("/var/lib/unattended-upgrade",
			WithMode(0o700), WithOwner("root"), WithGroup("root"))
	})
}

// DescUnattendedUnits returns the description for systemd timer install.
func (Unattended) DescUnattendedUnits() string {
	return "Install unattended-upgrade-rocky SystemdTimer (oneshot + per-host calendar)"
}

// UnattendedUnits installs the oneshot+timer pair via SystemdTimer and
// enables the timer. Per-host OnCalendar still needs a loop; identical
// bodies use WhenHostname(FleetHosts()).
func (Unattended) UnattendedUnits() {
	for _, host := range FleetHosts() {
		calendar := MustHostValue[string](host, fleet.ValueUnattendedOnCalendar)
		WhenHostname(host, func() {
			SystemdTimer("unattended-upgrade-rocky",
				WithCommand("/usr/local/sbin/unattended-upgrade-rocky daily"),
				WithOnCalendar(calendar),
				WithOnBootSec("10min"),
				WithPersistent,
				WithDescription("Hourly unattended-upgrade check (updates once per day)"),
				WithServiceDescription("Unattended upgrade (Rocky daily mode)"),
				WithAfter("network-online.target"),
				WithWants("network-online.target"),
			)
		})
	}
}

// DescUnattendedLogrotate returns the description for logrotate.
func (Unattended) DescUnattendedLogrotate() string {
	return "Install /etc/logrotate.d/unattended-upgrade"
}

// UnattendedLogrotate installs the logrotate snippet.
func (Unattended) UnattendedLogrotate() {
	src := paths.Frontends + "/systemd/unattended-upgrade.logrotate"
	WhenHostname(FleetHosts(), func() {
		InstallFile("/etc/logrotate.d/unattended-upgrade",
			src,
			WithMode(0o644), WithOwner("root"), WithGroup("root"))
	})
}
