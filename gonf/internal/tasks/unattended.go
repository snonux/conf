// Package tasks declares the configuration tasks for the conf repository.
package tasks

import (
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

// Unattended tasks on Frontends live in this file; the struct declaration
// and the pipeline-test Ping task are in frontends.go.

// DescUnattendedScript returns the description for the wrapper deployment.
func (Frontends) DescUnattendedScript() string {
	return "Install /usr/local/sbin/unattended-upgrade wrapper (0755 root:wheel)"
}

// OptsUnattendedScript marks the wrapper deployment as privileged.
func (Frontends) OptsUnattendedScript() []TaskOption { return []TaskOption{Privileged()} }

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
func (Frontends) OptsUnattendedServices() []TaskOption { return []TaskOption{Privileged()} }

// UnattendedServices installs the daemon restart list.
func (Frontends) UnattendedServices() {
	File("/etc/unattended-upgrade-services",
		WithContent(unattendedServicesContent),
		WithMode(0o644), WithOwner("root"), WithGroup("wheel"))
}

// DescUnattendedCronBlowfish returns the blowfish cron schedule.
func (Frontends) DescUnattendedCronBlowfish() string {
	return "Root cron on blowfish: base 06:10, pkgs 06:40, reboot 07:10"
}

// OptsUnattendedCronBlowfish gates the blowfish cron jobs by destination
// hostname (plan recipe, evaluated at apply time) and marks them privileged.
func (Frontends) OptsUnattendedCronBlowfish() []TaskOption {
	return []TaskOption{Privileged(), WhenHostnameContains("blowfish")}
}

// UnattendedCronBlowfish installs blowfish's three root cron jobs.
func (Frontends) UnattendedCronBlowfish() {
	unattendedCronJobs("6", "6", "7")
}

// DescUnattendedCronFishfinger returns the fishfinger cron schedule.
func (Frontends) DescUnattendedCronFishfinger() string {
	return "Root cron on fishfinger: base 22:10, pkgs 22:40, reboot 23:10"
}

// OptsUnattendedCronFishfinger gates the fishfinger cron jobs by destination
// hostname (plan recipe) and marks them privileged.
func (Frontends) OptsUnattendedCronFishfinger() []TaskOption {
	return []TaskOption{Privileged(), WhenHostnameContains("fishfinger")}
}

// UnattendedCronFishfinger installs fishfinger's three root cron jobs.
func (Frontends) UnattendedCronFishfinger() {
	unattendedCronJobs("22", "22", "23")
}

// unattendedCronJobs registers the three unattended-upgrade root cron jobs
// (base, pkgs, reboot) with the given base, pkgs, and reboot hours. The
// minutes come from the decided schedule (10/40/10).
func unattendedCronJobs(baseHour, pkgsHour, rebootHour string) {
	Cron("unattended-upgrade-base",
		WithCommand("/usr/local/sbin/unattended-upgrade base"),
		WithMinute("10"), WithHour(baseHour))
	Cron("unattended-upgrade-pkgs",
		WithCommand("/usr/local/sbin/unattended-upgrade pkgs"),
		WithMinute("40"), WithHour(pkgsHour))
	Cron("unattended-upgrade-reboot",
		WithCommand("/usr/local/sbin/unattended-upgrade reboot"),
		WithMinute("10"), WithHour(rebootHour))
}

// DescUnattendedNewsyslog returns the description for the rotation line.
func (Frontends) DescUnattendedNewsyslog() string {
	return "Append unattended-upgrade log rotation to /etc/newsyslog.conf"
}

// OptsUnattendedNewsyslog marks the rotation line as privileged.
func (Frontends) OptsUnattendedNewsyslog() []TaskOption { return []TaskOption{Privileged()} }

// UnattendedNewsyslog appends the rotation line; the explicit mode matches
// the deployed file (0644) so no attribute churn happens on apply.
func (Frontends) UnattendedNewsyslog() {
	File("/etc/newsyslog.conf", WithLine(unattendedNewsyslogLine), WithMode(0o644))
}
