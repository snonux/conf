// Package freebsd declares FreeBSD f0–f3 host tasks for the conf repository —
// unattended package upgrades per
// frontends/docs/unattended-upgrades.md (plan record:
// docs/archive/frontends/docs/unattended-upgrades-freebsd.plan.md).
package freebsd

import (
	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"

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

// DescPackages returns the description for required packages.
func (Unattended) DescPackages() string {
	return "Install ksh (ksh93) for unattended-upgrade-freebsd"
}

// Packages installs ksh (script interpreter, house rule).
func (Unattended) Packages() {
	Package("ksh")
}

// DescScript returns the description for the wrapper deployment.
func (Unattended) DescScript() string {
	return "Install /usr/local/sbin/unattended-upgrade-freebsd (0755 root:wheel)"
}

// OptsScript records ksh, the script's interpreter, before the script.
// Privileged() is repeated because the per-method companion replaces the
// RequiresRoot struct default.
func (Unattended) OptsScript() TaskOptions {
	return TaskOptions{Privileged(), Needs("packages")}
}

// Script installs the FreeBSD ksh wrapper (after its directory, which gonf
// orders as the parent).
func (Unattended) Script() {
	EnsureDir("/usr/local/sbin", Perm(0o755, Root))
	InstallFile("/usr/local/sbin/unattended-upgrade-freebsd",
		paths.FrontendAsset("scripts/unattended-upgrade-freebsd.sh"),
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

// DescStampDir returns the description for the stamp directory.
func (Unattended) DescStampDir() string {
	return "Ensure /var/lib/unattended-upgrade stamp directory"
}

// StampDir creates the persistent stamp directory.
func (Unattended) StampDir() {
	EnsureDir("/var/lib/unattended-upgrade", Perm(0o700, Root))
}

// DescCron returns the description for the hourly cron.
func (Unattended) DescCron() string {
	return "Root cron: unattended-upgrade-freebsd daily, per-host hourly minute"
}

// OptsCron records the script, the restart list and the stamp directory
// before the job. Privileged() is repeated because the per-method companion
// replaces the RequiresRoot struct default.
func (Unattended) OptsCron() TaskOptions {
	return TaskOptions{Privileged(), Needs("script", "services", "stamp_dir")}
}

// Cron installs the hourly daily-mode job (stamp-gated in-script).
// Boot catch-up is the next hourly tick (gonf Cron has no @reboot field).
func (Unattended) Cron() {
	EachHost(func(s UnattendedSchedule) {
		Cron("unattended-upgrade-freebsd-daily",
			WithCommand("/usr/local/sbin/unattended-upgrade-freebsd daily"),
			WithMinute(s.Minute), WithHour("*"),
			WithCronEnv("PATH=/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin:/usr/local/sbin"),
		)
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

// DescRetiredPackages returns the description for the removed packages.
func (Unattended) DescRetiredPackages() string {
	return "Remove retired packages (python311 and its py311-* stack)"
}

// RetiredPackages keeps packages absent that nothing on the f-hosts needs any
// more but that unattended-upgrade-freebsd would otherwise keep patching (or,
// as with python311, keep flagging in pkg audit: it only upgrades, it never
// removes orphans). python311 and its py311-* modules were leftovers of an
// older py311 toolchain after awscli moved to python312; removed 2026-09-25.
// pkg delete takes the py311-* modules that depend on python311 with it.
func (Unattended) RetiredPackages() {
	NoPackage("python311")
}
