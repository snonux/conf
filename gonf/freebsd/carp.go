package freebsd

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// Carp deploys the CARP helper scripts of the storage pair f0/f1 (the
// f3s-storage-ha VIP, see the f3s-storage skill): /usr/local/bin/carp on both
// and, on f0 only, the minutely carp-auto-failback.sh that reclaims MASTER
// after f0 comes back. Both used to be hand-installed copies with no source in
// the repo; the sources now live in f3s/freebsd-hosts/carp next to
// carpcontrol.sh. carpcontrol.sh itself (the devd hook) is still installed by
// hand as the f3s-storage skill describes.
//
// Register with OnCluster(cluster.NameFreeBSD); the bodies narrow further to
// the CARP members with WhenHostname, since f2/f3 carry no CARP interface.
type Carp struct {
	RequiresRoot
}

const (
	carpScript         = "/usr/local/bin/carp"
	carpFailbackScript = "/usr/local/bin/carp-auto-failback.sh"
)

// carpMembers are the hosts that share the storage VIP; carpFailbackHost is
// the preferred MASTER that the failback cron job promotes.
var carpMembers = List("f0", "f1")

const carpFailbackHost = "f0"

// carpFailbackNewsyslogLine keeps five compressed 1 MB generations.
const carpFailbackNewsyslogLine = "/var/log/carp-auto-failback.log\t\troot:wheel\t644  5     1024  *     Z"

// DescScript returns the description for the carp CLI.
func (Carp) DescScript() string {
	return "Install " + carpScript + " on the CARP members f0/f1 (0755 root:wheel)"
}

// Script installs the carp state/management CLI on f0 and f1.
func (Carp) Script() {
	WhenHostname(carpMembers, func() {
		InstallFile(carpScript, paths.FHostAsset("carp/carp"), Perm(0o755, Root))
	})
}

// DescFailbackScript returns the description for the failback script.
func (Carp) DescFailbackScript() string {
	return "Install " + carpFailbackScript + " on f0 (0755 root:wheel)"
}

// OptsFailbackScript records the carp CLI first: the failback script calls
// "carp state" and "carp master".
func (Carp) OptsFailbackScript() TaskOptions {
	return TaskOptions{Needs("script")}
}

// FailbackScript installs the failback script on f0.
func (Carp) FailbackScript() {
	WhenHostname(carpFailbackHost, func() {
		InstallFile(carpFailbackScript,
			paths.FHostAsset("carp/carp-auto-failback.sh"), Perm(0o755, Root))
	})
}

// DescFailbackCron returns the description for the failback cron job.
func (Carp) DescFailbackCron() string {
	return "Root cron on f0: carp-auto-failback.sh every minute"
}

// OptsFailbackCron records the script before the job.
func (Carp) OptsFailbackCron() TaskOptions {
	return TaskOptions{Needs("failback_script")}
}

// DescFailbackNewsyslog returns the description for the log rotation line.
func (Carp) DescFailbackNewsyslog() string {
	return "Rotate /var/log/carp-auto-failback.log on f0 via /etc/newsyslog.conf"
}

// FailbackNewsyslog rotates the failback log: while f0 sits in INIT or is
// blocked from failing back, the script appends a SKIP line every minute.
func (Carp) FailbackNewsyslog() {
	WhenHostname(carpFailbackHost, func() {
		File("/etc/newsyslog.conf", WithLine(carpFailbackNewsyslogLine), WithMode(0o644))
	})
}

// FailbackCron runs the failback check every minute on f0. The first deploy
// on 2026-09-25 did NOT adopt the identical hand-added line (gonf bug, tracked
// as its own task) and the job ran twice until the unmanaged line was removed
// by hand; on a freshly built f0 there is no such line to collide with.
func (Carp) FailbackCron() {
	WhenHostname(carpFailbackHost, func() {
		CronAt("carp-auto-failback", "* * * * *", carpFailbackScript)
	})
}
