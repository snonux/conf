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

// GonfLink symlinks /usr/bin/gonf → /usr/local/bin/gonf (sudo secure_path).
//
// GonfLink puts gonf on sudo's secure_path so privileged push
// works (sudo -n gonf …). The bootstrap installs to /usr/local/bin.
func (Unattended) GonfLink() {
	Symlink("/usr/bin/gonf", "/usr/local/bin/gonf")
}

// Packages installs ksh + yum-utils (needs-restarting) for
// unattended-upgrade.
func (Unattended) Packages() {
	Packages("ksh", "yum-utils")
}

// OptsScript records ksh and yum-utils before the script that uses them.
func (Unattended) OptsScript() TaskOptions {
	return TaskOptions{Needs(Unattended.Packages)}
}

// Script installs /usr/local/sbin/unattended-upgrade-rocky (0755 root:root).
func (Unattended) Script() {
	EnsureDir("/usr/local/sbin", RootOwned)
	InstallFile("/usr/local/sbin/unattended-upgrade-rocky",
		paths.FrontendAsset("scripts/unattended-upgrade-rocky.sh"),
		RootExec)
}

// StampDir ensures /var/lib/unattended-upgrade stamp directory.
func (Unattended) StampDir() {
	EnsureDir("/var/lib/unattended-upgrade", RootPrivate)
}

// OptsUnits records the script and the stamp directory before the timer.
func (Unattended) OptsUnits() TaskOptions {
	return TaskOptions{Needs(Unattended.Script, Unattended.StampDir)}
}

// Units installs unattended-upgrade-rocky SystemdTimer (oneshot + per-host
// calendar).
//
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

// Logrotate installs /etc/logrotate.d/unattended-upgrade.
func (Unattended) Logrotate() {
	InstallFile("/etc/logrotate.d/unattended-upgrade",
		paths.FrontendAsset("systemd/unattended-upgrade.logrotate"),
		RootOwned)
}

// fleetTimezone is the zone of the whole home fleet. r0-r2 already use it;
// pi2/pi3 were on UTC. The relative target matches what timedatectl writes,
// so the link is already correct on the r-nodes and only the Pis change.
const fleetTimezone = "../usr/share/zoneinfo/Europe/Sofia"

// Timezone sets /etc/localtime to Europe/Sofia (fleet timezone).
//
// Timezone points /etc/localtime at the fleet zone. systemd picks the change
// up for timers and the journal; crond and rsyslog are restarted so their
// schedules and timestamps follow it. The OnCalendar times in gonf/cluster
// (UnattendedCalendar, KernelAuditCalendar) are local time from now on.
func (Unattended) Timezone() {
	tz := Symlink("/etc/localtime", fleetTimezone)
	Sh("systemctl try-restart crond rsyslog", OnChange(tz))
}
