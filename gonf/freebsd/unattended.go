// Package freebsd declares FreeBSD f0–f3 host tasks for the conf repository —
// unattended package upgrades per
// frontends/docs/unattended-upgrades.md (plan record:
// docs/archive/frontends/docs/unattended-upgrades-freebsd.plan.md).
package freebsd

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// unattendedServicesAsset is the operator-edited rc.d restart list for
// f0–f3 after pkg upgrades: a plain, native file
// (assets/unattended-upgrade-services), not a template. Never list vm /
// networking there — guest stop belongs to the reboot path (vm stopall
// then reboot).
func unattendedServicesAsset() string {
	return paths.GonfAsset("freebsd", "unattended-upgrade-services")
}

// unattendedNewsyslogLine rotates /var/log/unattended-upgrade.log.
const unattendedNewsyslogLine = "/var/log/unattended-upgrade.log\t\troot:wheel\t600  5     1024  *     Z"

// Unattended carries the unattended-upgrade deployment for FreeBSD hosts.
// Register with OnCluster(cluster.NameFreeBSD). The per-host hourly minute
// lives on each Host as WithData(UnattendedSchedule{…}); the reboot policy
// (f3 never reboots) is hardcoded in the script.
type Unattended struct {
	RequiresRoot
}

// UnattendedSchedule is a FreeBSD host's cron minute (host data, see
// gonf/cluster): the hourly job runs at this minute past every hour.
type UnattendedSchedule struct {
	Minute string
}

// Packages installs ksh (ksh93) for unattended-upgrade-freebsd.
func (Unattended) Packages() {
	Package("ksh")
}

// OptsScript records ksh, the script's interpreter, before the script.
func (Unattended) OptsScript() TaskOptions {
	return TaskOptions{Needs(Unattended.Packages)}
}

// Script installs /usr/local/sbin/unattended-upgrade-freebsd (0755
// root:wheel).
func (Unattended) Script() {
	EnsureDir("/usr/local/sbin", RootOwned)
	InstallFile("/usr/local/sbin/unattended-upgrade-freebsd",
		paths.FrontendAsset("scripts/unattended-upgrade-freebsd.sh"),
		RootExec)
}

// Services installs /etc/unattended-upgrade-services (0644 root:wheel).
func (Unattended) Services() {
	InstallFile("/etc/unattended-upgrade-services", unattendedServicesAsset(),
		RootOwned)
}

// StampDir ensures /var/lib/unattended-upgrade stamp directory.
func (Unattended) StampDir() {
	EnsureDir("/var/lib/unattended-upgrade", RootPrivate)
}

// DescCron returns the description for the hourly cron.
func (Unattended) DescCron() string {
	return "Root cron: unattended-upgrade-freebsd daily, per-host hourly minute"
}

// OptsCron records the script, the restart list and the stamp directory
// before the job.
func (Unattended) OptsCron() TaskOptions {
	return TaskOptions{Needs(Unattended.Script, Unattended.Services, Unattended.StampDir)}
}

// Cron installs the hourly daily-mode job (stamp-gated in-script).
// Boot catch-up is the next hourly tick (gonf Cron has no @reboot field).
//
// The script tees every line it prints into /var/log/unattended-upgrade.log,
// so stdout is dropped and stderr (a failing tool's own messages) is
// appended to the same log: cron no longer mails root, whose local mailbox
// nobody reads (task 2k2).
func (Unattended) Cron() {
	EachHost(func(s UnattendedSchedule) {
		Cron("unattended-upgrade-freebsd-daily",
			WithCommand("/usr/local/sbin/unattended-upgrade-freebsd daily >/dev/null 2>>/var/log/unattended-upgrade.log"),
			WithMinute(s.Minute), WithHour("*"),
			WithCronEnv("PATH=/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin:/usr/local/sbin"),
		)
	})
}

// Newsyslog appends unattended-upgrade log rotation to /etc/newsyslog.conf.
func (Unattended) Newsyslog() {
	File("/etc/newsyslog.conf", WithLine(unattendedNewsyslogLine), WithMode(0o644))
}

// RetiredPackages removes retired packages (python311 and its py311-*
// stack).
func (Unattended) RetiredPackages() {
	NoPackage("python311")
}
