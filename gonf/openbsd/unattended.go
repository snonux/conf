// Package openbsd declares OpenBSD frontend host tasks for the conf
// repository — most notably the
// unattended-upgrade deployment per frontends/docs/unattended-upgrades.md.
package openbsd

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// unattendedServicesAsset is the operator-edited daemon restart list
// deployed to /etc/unattended-upgrade-services (archived implementation doc section
// 4): a plain, native file (assets/unattended-upgrade-services), not a
// template — its content never varies with recipe input.
func unattendedServicesAsset() string {
	return paths.GonfAsset("openbsd", "unattended-upgrade-services")
}

// unattendedNewsyslogLine is appended to /etc/newsyslog.conf so
// /var/log/unattended-upgrade.log rotates (archived implementation doc section 6).
// The literal matches the line added to frontends/etc/newsyslog.conf, which
// the Rex-deployed wholesale copy carries too, so both mechanisms converge.
const unattendedNewsyslogLine = "/var/log/unattended-upgrade.log\t\troot:wheel\t600  5     1024  *     Z"

// Unattended carries the unattended-upgrade deployment tasks for the
// frontend hosts. The embedded RequiresRoot marker declares the execution
// contract on the struct itself: every task applies as root on the OpenBSD
// frontends. Register with OnCluster(cluster.NameFrontends). Per-host cron
// hours live on each Host as WithData(UnattendedSchedule{…}).
type Unattended struct {
	RequiresRoot
}

// UnattendedSchedule is a frontend's per-host cron window (host data, see
// gonf/cluster): the hours of the base and pkgs jobs, and the hour of the
// audit and reboot jobs.
type UnattendedSchedule struct {
	BaseHour  string
	PkgsHour  string
	AuditHour string
}

// OptsPing opts Ping out of the struct-level Privileged default with
// Unprivileged() (an OptsX companion adds to RequiresRoot rather than
// replacing it, and the pipeline smoke test must stay unprivileged) and marks
// it Operational: it is a diagnostic, run by name, never part of the
// frontends setup aggregate (pattern aggregates skip Operational tasks).
func (Unattended) OptsPing() TaskOptions { return TaskOptions{Unprivileged(), Operational()} }

// Ping verifies the gonf push pipeline to this host.
func (Unattended) Ping() {
	Noop("ping")
}

// Script installs /usr/local/sbin/unattended-upgrade wrapper (0755
// root:wheel).
func (Unattended) Script() {
	InstallFile("/usr/local/sbin/unattended-upgrade",
		paths.FrontendAsset("scripts/unattended-upgrade.sh"),
		RootExec)
}

// Services installs /etc/unattended-upgrade-services (0644 root:wheel).
func (Unattended) Services() {
	InstallFile("/etc/unattended-upgrade-services", unattendedServicesAsset(),
		RootOwned)
}

// DescCron returns the description for the per-host cron schedule.
func (Unattended) DescCron() string {
	return "Root cron: unattended-upgrade base/pkgs/audit/reboot, per-host schedule"
}

// OptsCron records the wrapper and the restart list before the cron jobs that
// run them.
func (Unattended) OptsCron() TaskOptions {
	return TaskOptions{Needs(Unattended.Script, Unattended.Services)}
}

// Cron installs the four root cron jobs on every frontend host,
// each host with its own window (morning on blowfish, evening on
// fishfinger): record once, evaluate per destination.
func (Unattended) Cron() {
	EachHost(unattendedCronJobs)
}

// unattendedCronJobs registers the four unattended-upgrade root cron jobs
// (base, pkgs, audit, reboot) in the host's window. The audit is scheduled
// five minutes after the latest possible package-job start and before the
// reboot window; lock contention reports failure rather than silently
// losing that day's security evidence. The minutes are 10/40/05/35.
func unattendedCronJobs(s UnattendedSchedule) {
	Cron("unattended-upgrade-base",
		WithCommand("/usr/local/sbin/unattended-upgrade base"),
		WithMinute("10"), WithHour(s.BaseHour))
	Cron("unattended-upgrade-pkgs",
		WithCommand("/usr/local/sbin/unattended-upgrade pkgs"),
		WithMinute("40"), WithHour(s.PkgsHour))
	Cron("unattended-upgrade-audit",
		WithCommand("/usr/local/sbin/unattended-upgrade audit"),
		WithMinute("05"), WithHour(s.AuditHour))
	Cron("unattended-upgrade-reboot",
		WithCommand("/usr/local/sbin/unattended-upgrade reboot"),
		WithMinute("35"), WithHour(s.AuditHour))
}

// Newsyslog appends unattended-upgrade log rotation to /etc/newsyslog.conf.
func (Unattended) Newsyslog() {
	File("/etc/newsyslog.conf", WithLine(unattendedNewsyslogLine), WithMode(0o644))
}
