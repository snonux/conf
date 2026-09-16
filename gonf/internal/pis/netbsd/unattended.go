// Package netbsd declares NetBSD pi0/pi1 host tasks for the conf repository —
// unattended upgrades per frontends/docs/unattended-upgrades-pi.plan.md.
package netbsd

import (
	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"

	"codeberg.org/snonux/conf/gonf/internal/fleet"
	"codeberg.org/snonux/conf/gonf/internal/paths"
)

// unattendedServicesContent is the rc.d restart list for pi0/pi1. Service name
// is wireguard (not wireguard-go) — matches /etc/rc.d/wireguard on the hosts.
const unattendedServicesContent = `# Daemons to restart after unattended package updates (one per line).
# Names must match /etc/rc.d/<name> on NetBSD pi0/pi1.
bozohttpd
wireguard
npf
uptimed
dserver
sshd
`

// unattendedNewsyslogLine rotates /var/log/unattended-upgrade.log (NetBSD
// newsyslog.conf format — same columns as the stock authlog/cron lines).
const unattendedNewsyslogLine = "/var/log/unattended-upgrade.log\troot:wheel\t600  5    1024 *    Z"

// Unattended carries the unattended-upgrade deployment for pi0/pi1.
// Register with WithFleet(fleet.NameNetBSDPis). Per-host cron hours live on
// each Host via WithValue(fleet.ValueUnattendedCron, …).
type Unattended struct {
	RequiresRoot
}

// DescUnattendedScript returns the description for the wrapper deployment.
func (Unattended) DescUnattendedScript() string {
	return "Install /usr/local/sbin/unattended-upgrade-netbsd (0755 root:wheel)"
}

// UnattendedScript installs the NetBSD ksh wrapper.
func (Unattended) UnattendedScript() {
	WhenHostname(FleetHosts(), func() {
		dir := EnsureDir("/usr/local/sbin",
			WithMode(0o755), WithOwner("root"), WithGroup("wheel"))
		InstallFile("/usr/local/sbin/unattended-upgrade-netbsd",
			paths.Frontends+"/scripts/unattended-upgrade-netbsd.sh",
			WithMode(0o755), WithOwner("root"), WithGroup("wheel"),
			DependsOn(dir))
	})
}

// DescUnattendedServices returns the description for the restart list.
func (Unattended) DescUnattendedServices() string {
	return "Install /etc/unattended-upgrade-services (0644 root:wheel)"
}

// UnattendedServices installs the rc.d restart list.
func (Unattended) UnattendedServices() {
	WhenHostname(FleetHosts(), func() {
		File("/etc/unattended-upgrade-services",
			WithContent(unattendedServicesContent),
			WithMode(0o644), WithOwner("root"), WithGroup("wheel"))
	})
}

// DescUnattendedCron returns the description for the per-host cron schedule.
func (Unattended) DescUnattendedCron() string {
	return "Root cron: unattended-upgrade-netbsd pkgs/reboot, per-host schedule"
}

// UnattendedCron installs pkgs + reboot cron jobs with per-host windows.
func (Unattended) UnattendedCron() {
	for _, host := range FleetHosts() {
		w := MustHostValue[[2]string](host, fleet.ValueUnattendedCron)
		WhenHostname(host, func() { unattendedCronJobs(w) })
	}
}

func unattendedCronJobs(w [2]string) {
	Cron("unattended-upgrade-netbsd-pkgs",
		WithCommand("/usr/local/sbin/unattended-upgrade-netbsd pkgs"),
		WithMinute("10"), WithHour(w[0]))
	Cron("unattended-upgrade-netbsd-reboot",
		WithCommand("/usr/local/sbin/unattended-upgrade-netbsd reboot"),
		WithMinute("50"), WithHour(w[1]))
}

// DescUnattendedNewsyslog returns the description for the rotation line.
func (Unattended) DescUnattendedNewsyslog() string {
	return "Append unattended-upgrade log rotation to /etc/newsyslog.conf"
}

// UnattendedNewsyslog appends the rotation line.
func (Unattended) UnattendedNewsyslog() {
	WhenHostname(FleetHosts(), func() {
		File("/etc/newsyslog.conf", WithLine(unattendedNewsyslogLine), WithMode(0o644))
	})
}
