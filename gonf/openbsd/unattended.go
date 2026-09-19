// Package openbsd declares OpenBSD frontend host tasks for the conf
// repository — most notably the
// unattended-upgrade deployment per frontends/docs/unattended-upgrades.*.
package openbsd

import (
	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"

	"codeberg.org/snonux/conf/gonf/internal/fleet"
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

// Unattended carries the unattended-upgrade deployment tasks for the
// frontend hosts. The embedded RequiresRoot marker declares the execution
// contract on the struct itself: every task applies as root on the OpenBSD
// frontends. Register with WithFleet(fleet.NameFrontends). Per-host cron
// hours live on each Host via WithValue(fleet.ValueUnattendedCron, …).
type Unattended struct {
	RequiresRoot
}

// OptsSmoke opts Ping out of the struct-level Privileged default: the
// pipeline smoke test must stay unprivileged.
func (Unattended) OptsPing() TaskOptions { return TaskOptions{} }

// DescPing returns the description shown for the frontends_ping task.
func (Unattended) DescPing() string {
	return "Verify the gonf push pipeline to this host"
}

// Ping is a minimal no-op task used to verify the gonf push pipeline to the
// OpenBSD frontends: the Unless guard makes the command never run.
func (Unattended) Ping() {
	Command("true", nil,
		Unless("true", nil),
		WithName("ping"),
	)
}

// DescUnattendedScript returns the description for the wrapper deployment.
func (Unattended) DescUnattendedScript() string {
	return "Install /usr/local/sbin/unattended-upgrade wrapper (0755 root:wheel)"
}

// UnattendedScript installs the hardened ksh wrapper script.
func (Unattended) UnattendedScript() {
	InstallFile("/usr/local/sbin/unattended-upgrade",
		paths.Frontends+"/scripts/unattended-upgrade.sh",
		WithMode(0o755), WithOwner("root"), WithGroup("wheel"))
}

// DescUnattendedServices returns the description for the restart list.
func (Unattended) DescUnattendedServices() string {
	return "Install /etc/unattended-upgrade-services (0644 root:wheel)"
}

// UnattendedServices installs the daemon restart list.
func (Unattended) UnattendedServices() {
	File("/etc/unattended-upgrade-services",
		WithContent(unattendedServicesContent),
		WithMode(0o644), WithOwner("root"), WithGroup("wheel"))
}

// DescUnattendedCron returns the description for the per-host cron schedule.
func (Unattended) DescUnattendedCron() string {
	return "Root cron: unattended-upgrade base/pkgs/reboot, per-host schedule"
}

// UnattendedCron installs the three root cron jobs on every frontend host,
// each host with its own window (morning on blowfish, evening on
// fishfinger): record once, evaluate per destination.
func (Unattended) UnattendedCron() {
	for _, host := range FleetHosts() {
		w := MustHostValue[[3]string](host, fleet.ValueUnattendedCron)
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
func (Unattended) DescUnattendedNewsyslog() string {
	return "Append unattended-upgrade log rotation to /etc/newsyslog.conf"
}

// UnattendedNewsyslog appends the rotation line; the explicit mode matches
// the deployed file (0644) so no attribute churn happens on apply.
func (Unattended) UnattendedNewsyslog() {
	File("/etc/newsyslog.conf", WithLine(unattendedNewsyslogLine), WithMode(0o644))
}
