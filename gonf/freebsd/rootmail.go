package freebsd

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/mailbox"
)

// RootMail stops the f-hosts filling root's mailbox (task 2k2). DMA delivers
// root's mail to /var/mail/root and nothing reads or forwards it, so the
// user decided to log instead and keep Gogios as the alert channel. Periodic
// output goes to /var/log/*.log (Periodic.Conf), the unattended-upgrade cron
// job to its log (Unattended.Cron), the apcupsd clients no longer mail
// (Apcupsd), and the goprecords upload job pipes its output (curl's error
// when goprecords is unreachable: 361 of f0's mails, mostly around the
// nightly power-off) into syslog. RootMail used to add that logger pipe to
// the hand-made /etc/crontab line in place; since task lk2 Goprecords owns
// the job (root's crontab, pipe included) and removes that line, so the
// rewrite is gone. What remains here is the mailbox itself.
//
// Register with OnCluster(cluster.NameFreeBSD).
type RootMail struct {
	RequiresRoot
}

// Archive archives /var/mail/root to a dated .gz and empties it when above 1
// MiB.
//
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
