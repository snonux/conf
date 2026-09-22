// Package rocky declares Rocky Linux host tasks for the conf repository —
// unattended upgrades for pi2/pi3 and k3s nodes r0–r2 per
// frontends/docs/unattended-upgrades-pi.plan.md, plus the pi2/pi3
// Raspberry Pi kernel CVE audit (kernelaudit.go, plan §13).
package rocky

import (
	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"

	"codeberg.org/snonux/conf/gonf/cluster"
	"codeberg.org/snonux/conf/gonf/paths"
)

// Unattended carries the unattended-upgrade deployment for all Rocky hosts.
// Register with WithCluster(cluster.NameRockyAll). Per-host OnCalendar lives on
// each Host via WithValue(cluster.ValueUnattendedOnCalendar, …).
type Unattended struct {
	RequiresRoot
}

// DescGonfLink returns the description for the sudo PATH link.
func (Unattended) DescGonfLink() string {
	return "Symlink /usr/bin/gonf → /usr/local/bin/gonf (sudo secure_path)"
}

// GonfLink puts gonf on sudo's secure_path so privileged push
// works (sudo -n gonf …). The bootstrap installs to /usr/local/bin.
func (Unattended) GonfLink() {
	WhenHostname(ClusterHosts(), func() {
		Link("/usr/bin/gonf", WithSymlink("/usr/local/bin/gonf"))
	})
}

// DescPackages returns the description for required packages.
func (Unattended) DescPackages() string {
	return "Install ksh + yum-utils (needs-restarting) for unattended-upgrade"
}

// Packages installs ksh (script interpreter) and yum-utils
// (needs-restarting -s/-r).
func (Unattended) Packages() {
	WhenHostname(ClusterHosts(), func() {
		Package("ksh")
		Package("yum-utils")
	})
}

// DescScript returns the description for the wrapper deployment.
func (Unattended) DescScript() string {
	return "Install /usr/local/sbin/unattended-upgrade-rocky (0755 root:root)"
}

// Script installs the Rocky ksh wrapper.
func (Unattended) Script() {
	WhenHostname(ClusterHosts(), func() {
		dir := EnsureDir("/usr/local/sbin",
			WithMode(0o755), WithOwner("root"), WithGroup("root"))
		InstallFile("/usr/local/sbin/unattended-upgrade-rocky",
			paths.Frontends+"/scripts/unattended-upgrade-rocky.sh",
			WithMode(0o755), WithOwner("root"), WithGroup("root"),
			DependsOn(dir))
	})
}

// DescStampDir returns the description for the stamp directory.
func (Unattended) DescStampDir() string {
	return "Ensure /var/lib/unattended-upgrade stamp directory"
}

// StampDir creates the persistent stamp directory.
func (Unattended) StampDir() {
	WhenHostname(ClusterHosts(), func() {
		EnsureDir("/var/lib/unattended-upgrade",
			WithMode(0o700), WithOwner("root"), WithGroup("root"))
	})
}

// DescUnits returns the description for systemd timer install.
func (Unattended) DescUnits() string {
	return "Install unattended-upgrade-rocky SystemdTimer (oneshot + per-host calendar)"
}

// Units installs the oneshot+timer pair via SystemdTimer and
// enables the timer. ForHosts supplies each host's OnCalendar under its
// hostname guard; identical bodies use WhenHostname(ClusterHosts()).
func (Unattended) Units() {
	ForHosts(cluster.ValueUnattendedOnCalendar, func(_ string, calendar string) {
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

// DescLogrotate returns the description for logrotate.
func (Unattended) DescLogrotate() string {
	return "Install /etc/logrotate.d/unattended-upgrade"
}

// Logrotate installs the logrotate snippet.
func (Unattended) Logrotate() {
	src := paths.Frontends + "/systemd/unattended-upgrade.logrotate"
	WhenHostname(ClusterHosts(), func() {
		InstallFile("/etc/logrotate.d/unattended-upgrade",
			src,
			WithMode(0o644), WithOwner("root"), WithGroup("root"))
	})
}
