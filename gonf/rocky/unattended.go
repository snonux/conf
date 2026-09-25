// Package rocky declares Rocky Linux host tasks for the conf repository —
// unattended upgrades for pi2/pi3 and k3s nodes r0–r2 per
// frontends/docs/unattended-upgrades.md (plan record:
// docs/archive/frontends/docs/unattended-upgrades-pi.plan.md), plus the pi2/pi3
// Raspberry Pi kernel CVE audit (kernelaudit.go, plan §13).
package rocky

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// Unattended carries the unattended-upgrade deployment for all Rocky hosts.
// Register with OnCluster(cluster.NameRockyAll). Per-host OnCalendar lives on
// each Host as WithData(UnattendedCalendar{…}).
type Unattended struct {
	RequiresRoot
}

// UnattendedCalendar is a Rocky host's timer schedule (host data, see
// gonf/cluster), a systemd OnCalendar= expression.
type UnattendedCalendar struct {
	OnCalendar string
}

// DescGonfLink returns the description for the sudo PATH link.
func (Unattended) DescGonfLink() string {
	return "Symlink /usr/bin/gonf → /usr/local/bin/gonf (sudo secure_path)"
}

// GonfLink puts gonf on sudo's secure_path so privileged push
// works (sudo -n gonf …). The bootstrap installs to /usr/local/bin.
func (Unattended) GonfLink() {
	Link("/usr/bin/gonf", WithSymlink("/usr/local/bin/gonf"))
}

// DescPackages returns the description for required packages.
func (Unattended) DescPackages() string {
	return "Install ksh + yum-utils (needs-restarting) for unattended-upgrade"
}

// Packages installs ksh (script interpreter) and yum-utils
// (needs-restarting -s/-r).
func (Unattended) Packages() {
	Packages("ksh", "yum-utils")
}

// DescScript returns the description for the wrapper deployment.
func (Unattended) DescScript() string {
	return "Install /usr/local/sbin/unattended-upgrade-rocky (0755 root:root)"
}

// OptsScript records ksh and yum-utils before the script that uses them.
func (Unattended) OptsScript() TaskOptions {
	return TaskOptions{Needs("packages")}
}

// Script installs the Rocky ksh wrapper (after its directory, which gonf
// orders as the parent).
func (Unattended) Script() {
	EnsureDir("/usr/local/sbin", Perm(0o755, Root))
	InstallFile("/usr/local/sbin/unattended-upgrade-rocky",
		paths.FrontendAsset("scripts/unattended-upgrade-rocky.sh"),
		Perm(0o755, Root))
}

// DescStampDir returns the description for the stamp directory.
func (Unattended) DescStampDir() string {
	return "Ensure /var/lib/unattended-upgrade stamp directory"
}

// StampDir creates the persistent stamp directory.
func (Unattended) StampDir() {
	EnsureDir("/var/lib/unattended-upgrade", Perm(0o700, Root))
}

// DescUnits returns the description for systemd timer install.
func (Unattended) DescUnits() string {
	return "Install unattended-upgrade-rocky SystemdTimer (oneshot + per-host calendar)"
}

// OptsUnits records the script and the stamp directory before the timer.
func (Unattended) OptsUnits() TaskOptions {
	return TaskOptions{Needs("script", "stamp_dir")}
}

// Units installs the oneshot+timer pair via SystemdTimer and
// enables the timer. EachHost supplies each host's OnCalendar under its
// hostname guard.
func (Unattended) Units() {
	EachHost(func(c UnattendedCalendar) {
		SystemdTimer("unattended-upgrade-rocky",
			WithCommand("/usr/local/sbin/unattended-upgrade-rocky daily"),
			WithOnCalendar(c.OnCalendar),
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
	InstallFile("/etc/logrotate.d/unattended-upgrade",
		paths.FrontendAsset("systemd/unattended-upgrade.logrotate"),
		Perm(0o644, Root))
}

// fleetTimezone is the zone of the whole home fleet. r0-r2 already use it;
// pi2/pi3 were on UTC. The relative target matches what timedatectl writes,
// so the link is already correct on the r-nodes and only the Pis change.
const fleetTimezone = "../usr/share/zoneinfo/Europe/Sofia"

// DescTimezone returns the description for the timezone link.
func (Unattended) DescTimezone() string {
	return "Set /etc/localtime to Europe/Sofia (fleet timezone)"
}

// Timezone points /etc/localtime at the fleet zone. systemd picks the change
// up for timers and the journal; crond and rsyslog are restarted so their
// schedules and timestamps follow it. The OnCalendar times in gonf/cluster
// (UnattendedCalendar, KernelAuditCalendar) are local time from now on.
func (Unattended) Timezone() {
	tz := Link("/etc/localtime", WithSymlink(fleetTimezone))
	Sh("systemctl try-restart crond rsyslog", OnChange(tz))
}
