package netbsd

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/mailbox"
)

// Periodic sends the output of NetBSD's /etc/daily and /etc/weekly on
// pi0/pi1 to log files instead of root's mailbox (task 2k2). postfix
// delivers root's mail to /var/mail/root and nothing reads or forwards it
// (pi1 had 189 unread messages); the user decided to log instead and keep
// Gogios as the alert channel.
//
// NetBSD has no periodic.conf output knob: the stock root crontab pipes the
// scripts into "sendmail -t" (the scripts print their own To:/Subject:
// headers), so the crontab lines are what changes. The monthly line is
// commented out in the stock crontab and stays so.
//
// Register with OnCluster(cluster.NameNetBSDPis).
type Periodic struct {
	RequiresRoot
}

// Newsyslog.conf lines of the two logs (NetBSD format, like the stock
// ones): daily.log is rotated weekly (168 h) and keeps 8 generations,
// weekly.log every 4 weeks (672 h), keeping 6; Z = gzip, N = no daemon to
// signal. The interval counts from the last rotation, so a Pi that is off at
// some hour does not skip it.
const (
	dailyLogNewsyslog  = "/var/log/daily.log\troot:wheel\t640  8    *    168  ZN"
	weeklyLogNewsyslog = "/var/log/weekly.log\troot:wheel\t640  6    *    672  ZN"
)

// DescConf returns the description for the daily.conf override.
func (Periodic) DescConf() string {
	return "daily.conf: security output inline in the daily output (no separate mail)"
}

// Conf turns separate_security_email off, so /etc/daily prints the security
// run's output into its own output (the log) instead of mailing it to root
// with mail(1) ("daily insecurity output").
func (Periodic) Conf() {
	File("/etc/daily.conf", WithLine("separate_security_email=NO"), RootOwned)
}

// Logs creates /var/log/daily.log and weekly.log (0640) and rotates them
// with newsyslog.
func (Periodic) Logs() {
	EnsureFile("/var/log/daily.log", Perm(0o640, Root))
	EnsureFile("/var/log/weekly.log", Perm(0o640, Root))
	File("/etc/newsyslog.conf",
		WithLine(dailyLogNewsyslog), WithLine(weeklyLogNewsyslog),
		WithMode(0o644))
	NoFile(List("/var/log/daily.out", "/var/log/weekly.out"))
}

// DescCron returns the description for the daily/weekly crontab lines.
func (Periodic) DescCron() string {
	return "Root cron: /etc/daily and /etc/weekly append to /var/log/*.log instead of sendmail"
}

// OptsCron creates the logs (with their mode) and the daily.conf override
// before the jobs write to them.
func (Periodic) OptsCron() TaskOptions {
	return TaskOptions{Needs(Periodic.Conf, Periodic.Logs)}
}

// Cron adopts the stock daily and weekly lines (same times: 04:15 daily,
// 05:30 on Saturday) and appends their output, headers included, to the
// logs. The To:/Subject: header lines the scripts print for sendmail stay
// in the log as a dated separator between runs.
func (Periodic) Cron() {
	Cron("periodic-daily",
		WithCommand("/bin/sh /etc/daily >>/var/log/daily.log 2>&1"),
		WithMinute("15"), WithHour("4"),
		WithLegacyCommand("/bin/sh /etc/daily 2>&1 | tee /var/log/daily.out | sendmail -t"))
	Cron("periodic-weekly",
		WithCommand("/bin/sh /etc/weekly >>/var/log/weekly.log 2>&1"),
		WithMinute("30"), WithHour("5"), WithWeekday("6"),
		WithLegacyCommand("/bin/sh /etc/weekly 2>&1 | tee /var/log/weekly.out | sendmail -t"))
}

// Archive archives /var/mail/root to a dated .gz and empties it when above 1
// MiB.
func (Periodic) Archive() {
	Command("sh", List("-c", mailbox.Dotlocked(mailbox.ArchiveScript())),
		OnlyIf("sh", List("-c", mailbox.Guard())),
		WithName("archive-root-mailbox"))
}
