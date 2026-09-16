// Package pis declares Raspberry Pi host tasks for the conf repository —
// NetBSD pi0/pi1 and Rocky pi2/pi3 unattended upgrades per
// frontends/docs/unattended-upgrades-pi.plan.md.
package pis

import (
	"sort"

	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"

	"codeberg.org/snonux/conf/gonf/internal/paths"
)

// netbsdServicesContent is the rc.d restart list for pi0/pi1. Service name
// is wireguard (not wireguard-go) — matches /etc/rc.d/wireguard on the hosts.
const netbsdServicesContent = `# Daemons to restart after unattended package updates (one per line).
# Names must match /etc/rc.d/<name> on NetBSD pi0/pi1.
bozohttpd
wireguard
npf
uptimed
dserver
sshd
`

// netbsdNewsyslogLine rotates /var/log/unattended-upgrade.log (NetBSD
// newsyslog.conf format — same columns as the stock authlog/cron lines).
const netbsdNewsyslogLine = "/var/log/unattended-upgrade.log\troot:wheel\t600  5    1024 *    Z"

// netbsdCronWindows maps short hostnames to pkgs/reboot hours (minutes
// fixed at 10/50 per plan §6: pi0 morning, pi1 evening outside gogios).
var netbsdCronWindows = map[string][2]string{
	"pi0": {"2", "2"},
	"pi1": {"22", "22"},
}

func netbsdHosts() []string {
	hosts := make([]string, 0, len(netbsdCronWindows))
	for host := range netbsdCronWindows {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)
	return hosts
}

// NetBSD carries the unattended-upgrade deployment for pi0/pi1.
// Every task body is wrapped in WhenHostname so a mistaken push to a Rocky
// Pi is a no-op on the destination.
type NetBSD struct {
	RequiresRoot
}

// DescUnattendedScript returns the description for the wrapper deployment.
func (NetBSD) DescUnattendedScript() string {
	return "Install /usr/local/sbin/unattended-upgrade-netbsd (0755 root:wheel)"
}

// UnattendedScript installs the NetBSD ksh wrapper.
func (NetBSD) UnattendedScript() {
	for _, host := range netbsdHosts() {
		WhenHostname(host, func() {
			dir := EnsureDir("/usr/local/sbin",
				WithMode(0o755), WithOwner("root"), WithGroup("wheel"))
			InstallFile("/usr/local/sbin/unattended-upgrade-netbsd",
				paths.Frontends+"/scripts/unattended-upgrade-netbsd.sh",
				WithMode(0o755), WithOwner("root"), WithGroup("wheel"),
				DependsOn(dir))
		})
	}
}

// DescUnattendedServices returns the description for the restart list.
func (NetBSD) DescUnattendedServices() string {
	return "Install /etc/unattended-upgrade-services (0644 root:wheel)"
}

// UnattendedServices installs the rc.d restart list.
func (NetBSD) UnattendedServices() {
	for _, host := range netbsdHosts() {
		WhenHostname(host, func() {
			File("/etc/unattended-upgrade-services",
				WithContent(netbsdServicesContent),
				WithMode(0o644), WithOwner("root"), WithGroup("wheel"))
		})
	}
}

// DescUnattendedCron returns the description for the per-host cron schedule.
func (NetBSD) DescUnattendedCron() string {
	return "Root cron: unattended-upgrade-netbsd pkgs/reboot, per-host schedule"
}

// UnattendedCron installs pkgs + reboot cron jobs with per-host windows.
func (NetBSD) UnattendedCron() {
	for _, host := range netbsdHosts() {
		w := netbsdCronWindows[host]
		WhenHostname(host, func() { netbsdCronJobs(w) })
	}
}

func netbsdCronJobs(w [2]string) {
	Cron("unattended-upgrade-netbsd-pkgs",
		WithCommand("/usr/local/sbin/unattended-upgrade-netbsd pkgs"),
		WithMinute("10"), WithHour(w[0]))
	Cron("unattended-upgrade-netbsd-reboot",
		WithCommand("/usr/local/sbin/unattended-upgrade-netbsd reboot"),
		WithMinute("50"), WithHour(w[1]))
}

// DescUnattendedNewsyslog returns the description for the rotation line.
func (NetBSD) DescUnattendedNewsyslog() string {
	return "Append unattended-upgrade log rotation to /etc/newsyslog.conf"
}

// UnattendedNewsyslog appends the rotation line.
func (NetBSD) UnattendedNewsyslog() {
	for _, host := range netbsdHosts() {
		WhenHostname(host, func() {
			File("/etc/newsyslog.conf", WithLine(netbsdNewsyslogLine), WithMode(0o644))
		})
	}
}
