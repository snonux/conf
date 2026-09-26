package freebsd

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/goprecords"
	"github.com/snonux/conf/gonf/paths"
)

// Goprecords manages the hourly uptimed upload to goprecords on f0-f3 (task
// lk2), the f-host counterpart of frontends_goprecords. Until 2026-09-25 it
// was hand-made (f3s blog part 2): the client script was an older copy (the
// goprecords repo's, without the /usr/pkg PATH entries the repo copy gained
// for the NetBSD Pis; harmless on FreeBSD), the token a hand-copied file and
// the schedule a hand-added /etc/crontab line that RootMail.Goprecords later
// piped into logger. Goprecords now owns all of it: the schedule moved to
// root's crontab (gonf's Cron), with the same minute and the same logger
// pipe, and the /etc/crontab line is removed.
//
// Register with OnCluster(cluster.NameFreeBSD); every host carries a
// GoprecordsClient.
type Goprecords struct {
	RequiresRoot
}

// GoprecordsClient is an f-host's goprecords identity (host data, see
// gonf/cluster): Host is the GOPRECORDS_HOST its uploads are filed under,
// the name its token was issued for on the server (--create-client-key),
// and the name of its vault entry Infra/goprecords-token-<Host>.
type GoprecordsClient struct {
	Host string
}

const (
	// goprecordsSchedule is the hand-made line's schedule, kept: hourly on
	// the hour on all four hosts.
	// The command (goprecords.CronCommand) keeps the client's output in
	// /var/log/messages instead of root's mailbox (task 2k2, RootMail).
	goprecordsSchedule = "0 * * * *"
	// etcCrontabGoprecordsRE matches the legacy /etc/crontab line, with or
	// without RootMail's logger pipe. BSD sed -E and grep -E share the
	// syntax; it contains slashes, so sed addresses it as \#...#.
	etcCrontabGoprecordsRE = `[[:space:]]/usr/local/bin/goprecords-upload-client\.sh([[:space:]]|$)`
)

// GoprecordsToken is the logical secret reference of host's upload token.
// cmd/gonf maps it to the vault entry Infra/goprecords-token-<host>.
func GoprecordsToken(host string) string {
	return paths.FHostSecret("goprecords/" + host + ".token")
}

// OptsUpload records uptimed first: the client uploads its records file.
func (Goprecords) OptsUpload() TaskOptions {
	return TaskOptions{Needs(Base.Uptimed)}
}

// Upload installs the goprecords upload client, token and hourly root cron
// (output to syslog).
//
// Upload installs goprecords.Client and schedules it hourly in root's
// crontab. A host without a token gets nothing beyond curl and keeps its
// hand-made setup (the optional-token policy, see goprecords.Client).
//
// The legacy /etc/crontab line is deleted only after the new cron entry is
// in place, so no hour goes without an upload; cron(8) re-reads
// /etc/crontab on change. The delete is a guarded sed like RootMail's
// rewrite used to be: the line is hand-made, so there is no exact line to
// hand to WithoutLine.
func (Goprecords) Upload() {
	EachHost(func(c GoprecordsClient) {
		host := c.Host
		client, ok := goprecords.Client(GoprecordsToken(host))
		if !ok {
			return
		}
		cron := CronAt("goprecords-upload", goprecordsSchedule, goprecords.CronCommand(host),
			DependsOn(client))
		Command("sed", List("-i", "", "-E", `\#`+etcCrontabGoprecordsRE+`#d`, "/etc/crontab"),
			OnlyIf("grep", List("-Eq", etcCrontabGoprecordsRE, "/etc/crontab")),
			WithName("etc-crontab-goprecords-remove"), DependsOn(cron))
	})
}
