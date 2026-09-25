package freebsd

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/mailbox"
)

// RootMail stops the f-hosts filling root's mailbox (task 2k2). DMA delivers
// root's mail to /var/mail/root and nothing reads or forwards it, so the
// user decided to log instead and keep Gogios as the alert channel. Periodic
// output goes to /var/log/*.log (Periodic.Conf), the unattended-upgrade cron
// job to its log (Unattended.Cron), and the apcupsd clients no longer mail
// (Apcupsd). What remained: the goprecords upload line of /etc/crontab,
// whose only output is curl's error when goprecords is unreachable (361 of
// f0's mails, mostly around the nightly power-off), and the mailbox itself.
//
// Register with OnCluster(cluster.NameFreeBSD).
type RootMail struct {
	RequiresRoot
}

// goprecordsCrontabRE matches the goprecords line of /etc/crontab while it
// still ends in the bare script path, i.e. before Goprecords appended the
// logger pipe. BSD sed -E and grep -E share the syntax.
const goprecordsCrontabRE = `^(.*[[:space:]]/usr/local/bin/goprecords-upload-client\.sh)$`

// DescGoprecords returns the description for the crontab rewrite.
func (RootMail) DescGoprecords() string {
	return "/etc/crontab: goprecords upload errors to syslog (logger), not root mail"
}

// Goprecords pipes the goprecords upload job's output into syslog (tag
// goprecords-upload, so /var/log/messages keeps the errors with a
// timestamp) instead of letting cron mail it. The line is hand-made (not in
// the stock crontab and not a gonf Cron, which manages root's crontab, not
// /etc/crontab), so it is rewritten in place like Periodic.Times does; the
// guard is false once the pipe is there. cron(8) re-reads /etc/crontab on
// change.
func (RootMail) Goprecords() {
	Command("sh", List("-c",
		`sed -i '' -E 's#`+goprecordsCrontabRE+`#\1 2>\&1 | logger -t goprecords-upload#' /etc/crontab`),
		OnlyIf("sh", List("-c", `grep -Eq '`+goprecordsCrontabRE+`' /etc/crontab`)),
		WithName("goprecords-crontab-logger"))
}

// DescArchive returns the description for the mailbox archive.
func (RootMail) DescArchive() string {
	return "Archive /var/mail/root to a dated .gz and empty it when above 1 MiB"
}

// Archive gzips /var/mail/root into a dated archive and empties it while it
// is larger than mailbox.ArchiveThreshold (see package mailbox). DMA locks
// the mailbox with flock(2) for delivery, so the script runs under
// lockf(1) on the mailbox: -k keeps lockf from deleting the mailbox
// afterwards (it removes its lock file by default), -t 60 gives up rather
// than hanging the apply behind a stuck delivery.
func (RootMail) Archive() {
	Command("lockf", List("-k", "-t", "60", mailbox.Root, "sh", "-c", mailbox.ArchiveScript()),
		OnlyIf("sh", List("-c", mailbox.Guard())),
		WithName("archive-root-mailbox"))
}
