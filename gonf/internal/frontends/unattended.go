// Package tasks declares the configuration tasks for the conf repository.
package tasks

import (
	"sort"

	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"

	"codeberg.org/snonux/conf/gonf/internal/paths"
)

// unattendedServicesContent is the daemon restart list deployed to
// /etc/unattended-upgrade-services (implementation doc section 4).
const unattendedServicesContent = `# Daemons to restart after unattended security updates (one per line).
# gogios is cron-driven (no daemon); rsync is inetd-spawned, so the
# inetd listener itself is listed. Restarting sshd never drops sessions.
relayd
httpd
nsd
smtpd
sshd
inetd
uptimed
node_exporter
dserver
#gorum
`

// unattendedNewsyslogLine is appended to /etc/newsyslog.conf so
// /var/log/unattended-upgrade.log rotates (implementation doc section 6).
// The literal matches the line added to frontends/etc/newsyslog.conf, which
// the Rex-deployed wholesale copy carries too, so both mechanisms converge.
const unattendedNewsyslogLine = "/var/log/unattended-upgrade.log\t\troot:wheel\t600  5     1024  *     Z"

// unattendedCronWindows maps frontend hosts to their daily unattended-upgrade
// cron hours (base, pkgs, reboot); the minutes are fixed at 10/40/10
// (implementation doc section 2: blowfish in the morning, fishfinger in the
// evening, staggered so both hosts never upgrade simultaneously).
var unattendedCronWindows = map[string][3]string{
	"blowfish":   {"6", "6", "7"},
	"fishfinger": {"22", "22", "23"},
}

// The unattended tasks below are methods on Frontends (declared in
// frontends.go alongside the pipeline-test Ping task).

// DescUnattendedScript returns the description for the wrapper deployment.
func (Frontends) DescUnattendedScript() string {
	return "Install /usr/local/sbin/unattended-upgrade wrapper (0755 root:wheel)"
}

// OptsUnattendedScript marks the wrapper deployment as privileged.
func (Frontends) OptsUnattendedScript() TaskOptions { return TaskOptions{Privileged()} }

// UnattendedScript installs the hardened ksh wrapper script.
func (Frontends) UnattendedScript() {
	InstallFile("/usr/local/sbin/unattended-upgrade",
		paths.Frontends+"/scripts/unattended-upgrade.sh",
		WithMode(0o755), WithOwner("root"), WithGroup("wheel"))
}

// DescUnattendedServices returns the description for the restart list.
func (Frontends) DescUnattendedServices() string {
	return "Install /etc/unattended-upgrade-services (0644 root:wheel)"
}

// OptsUnattendedServices marks the restart-list deployment as privileged.
func (Frontends) OptsUnattendedServices() TaskOptions { return TaskOptions{Privileged()} }

// UnattendedServices installs the daemon restart list.
func (Frontends) UnattendedServices() {
	File("/etc/unattended-upgrade-services",
		WithContent(unattendedServicesContent),
		WithMode(0o644), WithOwner("root"), WithGroup("wheel"))
}

// DescUnattendedCron returns the description for the per-host cron schedule.
func (Frontends) DescUnattendedCron() string {
	return "Root cron: unattended-upgrade base/pkgs/reboot, per-host schedule"
}

// OptsUnattendedCron marks the cron deployment as privileged. The per-host
// schedule selection happens inside the task body via WhenHostname recipes.
func (Frontends) OptsUnattendedCron() TaskOptions { return TaskOptions{Privileged()} }

// UnattendedCron installs the three root cron jobs on every frontend host,
// each host with its own window (morning on blowfish, evening on
// fishfinger): record once, evaluate per destination.
func (Frontends) UnattendedCron() {
	hosts := make([]string, 0, len(unattendedCronWindows))
	for host := range unattendedCronWindows {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts) // deterministic plan op order
	for _, host := range hosts {
		w := unattendedCronWindows[host]
		WhenHostname(host, func() { unattendedCronJobs(w) })
	}
}

// unattendedCronJobs registers the three unattended-upgrade root cron jobs
// (base, pkgs, reboot) with the given base, pkgs, and reboot hours. The
// minutes come from the decided schedule (10/40/10).
func unattendedCronJobs(w [3]string) {
	Cron("unattended-upgrade-base",
		WithCommand("/usr/local/sbin/unattended-upgrade base"),
		WithMinute("10"), WithHour(w[0]))
	Cron("unattended-upgrade-pkgs",
		WithCommand("/usr/local/sbin/unattended-upgrade pkgs"),
		WithMinute("40"), WithHour(w[1]))
	Cron("unattended-upgrade-reboot",
		WithCommand("/usr/local/sbin/unattended-upgrade reboot"),
		WithMinute("10"), WithHour(w[2]))
}

// DescUnattendedNewsyslog returns the description for the rotation line.
func (Frontends) DescUnattendedNewsyslog() string {
	return "Append unattended-upgrade log rotation to /etc/newsyslog.conf"
}

// OptsUnattendedNewsyslog marks the rotation line as privileged.
func (Frontends) OptsUnattendedNewsyslog() TaskOptions { return TaskOptions{Privileged()} }

// UnattendedNewsyslog appends the rotation line; the explicit mode matches
// the deployed file (0644) so no attribute churn happens on apply.
func (Frontends) UnattendedNewsyslog() {
	File("/etc/newsyslog.conf", WithLine(unattendedNewsyslogLine), WithMode(0o644))
}
