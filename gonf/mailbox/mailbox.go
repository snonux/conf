// Package mailbox holds the root-mailbox archiving shared by the FreeBSD
// f-hosts and the NetBSD Pis (conf task 2k2).
//
// Neither group forwards root's mail anywhere: DMA (FreeBSD) and postfix
// (NetBSD) deliver it to /var/mail/root, which nobody reads. Periodic and
// cron output now go to log files and Gogios stays the alert channel, so
// what is left in the mailbox is history. A mailbox above ArchiveThreshold
// is compressed into a dated archive next to it and then emptied, rather than
// deleted.
package mailbox

import "fmt"

// Root is the mailbox this package archives.
const Root = "/var/mail/root"

// ArchiveThreshold is the size, in 512-byte blocks (find -size), above which
// Root is archived: 2048 blocks = 1 MiB. Below it an archive is not worth a
// file; a quiet mailbox therefore stays as it is.
const ArchiveThreshold = 2048

// Guard is true (exit 0) while Root is larger than ArchiveThreshold. find
// exits 0 whether or not it matched, so grep turns "printed a path" into the
// exit status. A missing mailbox is false.
func Guard() string {
	return fmt.Sprintf(`find %s -size +%d 2>/dev/null | grep -q .`, Root, ArchiveThreshold)
}

// ArchiveScript returns the sh(1) script that archives Root into
// /var/mail/root-YYYYMMDD-HHMMSS.gz (0600, the seconds keep a second run on
// the same day from overwriting the first archive) and then truncates Root in
// place, so the mailbox keeps its owner, group and mode for the MTA. The
// mailbox is only emptied when gzip succeeded. The caller runs the script
// under the lock the host's MTA honours (see the recipes); a message
// delivered between gzip and the truncation would otherwise be lost.
func ArchiveScript() string {
	return fmt.Sprintf(`set -eu
umask 077
archive=%s-$(date +%%Y%%m%%d-%%H%%M%%S).gz
gzip -c %s >"$archive"
: >%s`, Root, Root, Root)
}

// Dotlocked wraps script in the mailbox dot-lock (Root + ".lock", created
// exclusively with noclobber) that postfix's local(8) takes for delivery on
// NetBSD (mailbox_delivery_lock = flock, dotlock). If a delivery holds the
// lock the script fails instead of racing it; the next apply retries.
// FreeBSD's DMA locks with flock(2) instead, which lockf(1) takes.
func Dotlocked(script string) string {
	return fmt.Sprintf(`set -C
: >%[1]s.lock || exit 1
trap 'rm -f %[1]s.lock' EXIT
set +C
%[2]s`, Root, script)
}
