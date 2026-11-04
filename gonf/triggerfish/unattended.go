package triggerfish

import (
	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

// Unattended installs official-mirror updates without rebooting the router.
type Unattended struct{ RequiresRoot }

// Script installs the router's base, package and audit wrapper.
func (Unattended) Script() {
	EnsureDir("/usr/local/sbin", RootOwned)
	InstallFile("/usr/local/sbin/unattended-upgrade-triggerfish",
		paths.GonfAsset("triggerfish", "unattended-upgrade"), RootExec,
		WithValidation("/bin/ksh", List("-n", CandidatePath)))
}

func (Unattended) OptsCron() TaskOptions {
	return TaskOptions{Needs(Unattended.Script, Unattended.Newsyslog)}
}

// Cron schedules daily base updates, package updates and an audit in local time.
func (Unattended) Cron() {
	// This single router has no failover peer. Do not schedule a reboot or
	// restart network daemons; the wrapper reports required operator action.
	CronAt("triggerfish-unattended-base", "10 4 * * *", "/usr/local/sbin/unattended-upgrade-triggerfish base")
	CronAt("triggerfish-unattended-pkgs", "40 4 * * *", "/usr/local/sbin/unattended-upgrade-triggerfish pkgs")
	CronAt("triggerfish-unattended-audit", "10 5 * * *", "/usr/local/sbin/unattended-upgrade-triggerfish audit")
}

// Newsyslog rotates the private unattended-upgrade log.
func (Unattended) Newsyslog() {
	File("/etc/newsyslog.conf", RootOwned,
		WithLine("/var/log/unattended-upgrade-triggerfish.log\troot:wheel\t600  5  1024  *  Z"))
}
