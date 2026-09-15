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
#dserver
#gorum
`

// unattendedNewsyslogLine is appended to /etc/newsyslog.conf so
// /var/log/unattended-upgrade.log rotates (implementation doc section 6).
// The literal matches the line added to frontends/etc/newsyslog.conf, which
// the Rex-deployed wholesale copy carries too, so both mechanisms converge.
const unattendedNewsyslogLine = "/var/log/unattended-upgrade.log\t\troot:wheel\t600  5     1024  *     Z"

// RegisterUnattended queues the unattended-upgrade tasks
// (frontends/docs/unattended-upgrades.implementation.md). Every task needs
// root on the OpenBSD frontends, so each one is Privileged; the per-host
// cron schedules are selected with WhenHostnameContains plan recipes, which
// are evaluated on the destination host at apply time.
//
// Note: the frontends aggregate expands via Matching over LOCALLY activated
// tasks, so the hostname-gated cron tasks are absent from aggregate pushes
// recorded on the controller. Push the four task names explicitly to deploy
// everything.
func RegisterUnattended() {
	Task("frontends_unattended_script",
		"Install /usr/local/sbin/unattended-upgrade wrapper (0755 root:wheel)",
		func() {
			InstallFile("/usr/local/sbin/unattended-upgrade",
				paths.Frontends+"/scripts/unattended-upgrade.sh",
				WithMode(0o755), WithOwner("root"), WithGroup("wheel"))
		},
		Privileged())

	Task("frontends_unattended_services",
		"Install /etc/unattended-upgrade-services (0644 root:wheel)",
		func() {
			File("/etc/unattended-upgrade-services",
				WithContent(unattendedServicesContent),
				WithMode(0o644), WithOwner("root"), WithGroup("wheel"))
		},
		Privileged())

	Task("frontends_unattended_cron_blowfish",
		"Root cron on blowfish: base 06:10, pkgs 06:40, reboot 07:10",
		func() {
			unattendedCronJobs("6", "6", "7")
		},
		Privileged(), WhenHostnameContains("blowfish"))

	Task("frontends_unattended_cron_fishfinger",
		"Root cron on fishfinger: base 22:10, pkgs 22:40, reboot 23:10",
		func() {
			unattendedCronJobs("22", "22", "23")
		},
		Privileged(), WhenHostnameContains("fishfinger"))

	Task("frontends_unattended_newsyslog",
		"Append unattended-upgrade log rotation to /etc/newsyslog.conf",
		func() {
			File("/etc/newsyslog.conf", WithLine(unattendedNewsyslogLine), WithMode(0o644))
		},
		Privileged())
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
