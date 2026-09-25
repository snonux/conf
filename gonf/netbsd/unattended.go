// Package netbsd declares NetBSD pi0/pi1 host tasks for the conf repository —
// unattended upgrades per frontends/docs/unattended-upgrades.md (plan record:
// docs/archive/frontends/docs/unattended-upgrades-pi.plan.md).
package netbsd

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// unattendedServicesAsset is the operator-edited rc.d restart list for
// pi0/pi1: a plain, native file (assets/unattended-upgrade-services), not a
// template. Service name is wireguard (not wireguard-go) — matches
// /etc/rc.d/wireguard on the hosts.
func unattendedServicesAsset() string {
	return paths.GonfAsset("netbsd", "unattended-upgrade-services")
}

// unattendedNewsyslogLine rotates /var/log/unattended-upgrade.log (NetBSD
// newsyslog.conf format — same columns as the stock authlog/cron lines).
const unattendedNewsyslogLine = "/var/log/unattended-upgrade.log\troot:wheel\t600  5    1024 *    Z"

// Unattended carries the unattended-upgrade deployment for pi0/pi1.
// Register with OnCluster(cluster.NameNetBSDPis). Per-host cron hours live on
// each Host as WithData(UnattendedSchedule{…}) and WithData(VulnAuditTime{…}).
type Unattended struct {
	RequiresRoot
}

// UnattendedSchedule is a Pi's per-host window (host data, see gonf/cluster):
// the hours of the pkgs and reboot jobs.
type UnattendedSchedule struct {
	PkgsHour   string
	RebootHour string
}

// DescScript returns the description for the wrapper deployment.
func (Unattended) DescScript() string {
	return "Install /usr/local/sbin/unattended-upgrade-netbsd (0755 root:wheel)"
}

// Script installs the NetBSD ksh wrapper. The file applies after the
// directory without DependsOn: gonf orders a path after its parent.
func (Unattended) Script() {
	EnsureDir("/usr/local/sbin", Perm(0o755, Root))
	InstallFile("/usr/local/sbin/unattended-upgrade-netbsd",
		paths.FrontendAsset("scripts/unattended-upgrade-netbsd.sh"),
		Perm(0o755, Root))
}

// DescServices returns the description for the restart list.
func (Unattended) DescServices() string {
	return "Install /etc/unattended-upgrade-services (0644 root:wheel)"
}

// Services installs the rc.d restart list.
func (Unattended) Services() {
	InstallFile("/etc/unattended-upgrade-services", unattendedServicesAsset(),
		Perm(0o644, Root))
}

// DescCron returns the description for the per-host cron schedule.
func (Unattended) DescCron() string {
	return "Root cron: unattended-upgrade-netbsd pkgs/reboot, per-host schedule"
}

// OptsCron records the wrapper and the restart list before the cron jobs.
func (Unattended) OptsCron() TaskOptions {
	return TaskOptions{Needs("script", "services")}
}

// Cron installs pkgs + reboot cron jobs with per-host windows.
func (Unattended) Cron() {
	EachHost(func(s UnattendedSchedule) {
		Cron("unattended-upgrade-netbsd-pkgs",
			WithCommand("/usr/local/sbin/unattended-upgrade-netbsd pkgs"),
			WithMinute("10"), WithHour(s.PkgsHour))
		Cron("unattended-upgrade-netbsd-reboot",
			WithCommand("/usr/local/sbin/unattended-upgrade-netbsd reboot"),
			WithMinute("50"), WithHour(s.RebootHour))
	})
}

// DescNewsyslog returns the description for the rotation line.
func (Unattended) DescNewsyslog() string {
	return "Append unattended-upgrade log rotation to /etc/newsyslog.conf"
}

// Newsyslog appends the rotation line.
func (Unattended) Newsyslog() {
	File("/etc/newsyslog.conf", WithLine(unattendedNewsyslogLine), WithMode(0o644))
}

// fleetTimezone is the zone of the rest of the home fleet (f0-f3, r0-r2, the
// laptop). The Pis used to run on UTC, which made their logs an off-by-three
// puzzle next to everything else. The frontends stay on CET: they live in a
// German data centre and are not part of the home fleet's log timeline.
const fleetTimezone = "/usr/share/zoneinfo/Europe/Sofia"

// DescTimezone returns the description for the timezone link.
func (Unattended) DescTimezone() string {
	return "Set /etc/localtime to Europe/Sofia (fleet timezone)"
}

// Timezone points /etc/localtime at the fleet zone and restarts cron so its
// schedule (the per-host upgrade/reboot hours, which are now local time) and
// syslogd's timestamps follow it.
//
// Note: the per-host hours in gonf/cluster (UnattendedSchedule,
// VulnAuditTime) are interpreted in this zone from now on; they were UTC.
func (Unattended) Timezone() {
	tz := Link("/etc/localtime", WithSymlink(fleetTimezone))
	Sh("/etc/rc.d/cron restart", OnChange(tz))
	Sh("/etc/rc.d/syslogd restart", OnChange(tz))
}
