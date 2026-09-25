package freebsd

import (
	"fmt"

	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"
)

// Periodic tunes periodic(8) on the f-hosts: when it runs, what it walks, and
// the ZFS scrub it drives.
//
// The stock /etc/crontab runs "periodic daily" at 03:01, but f0-f3 are
// powered off most nights (f3sctl "power off" around midnight, wake around
// 10:00), so from 2026-08-30 to 2026-09-25 f0's daily run happened once. With
// it went the daily security run, and no pool had been scrubbed since
// 2026-05-17. Periodic now runs in the afternoon, one hour per host so the
// storage pair f0/f1 never scrub or walk their pools at the same time, and
// threshold-based scrubs catch up on the next daily run whenever one was
// missed.
//
// Moving periodic into working hours moves its disk walks there too, next to
// the NFS traffic of the k3s pods, so the walks over the 2.25 TB NFS dataset
// are removed rather than scheduled: zdata gets setuid=off (the daily
// security run's setuid/neggrpperm find skips nosuid filesystems, and nothing
// on the NFS data should be setuid anyway), and the weekly locate database,
// which nobody queries on these hosts, is off.
//
// Register with OnCluster(cluster.NameFreeBSD); every f-host carries a
// PeriodicSchedule.
type Periodic struct {
	RequiresRoot
}

// PeriodicSchedule is a FreeBSD host's periodic hour (host data, see
// gonf/cluster): daily at :01, weekly (Saturday) at :15 and monthly (the 1st)
// at :30 of this hour.
type PeriodicSchedule struct {
	Hour string
}

// periodicLines are the three periodic lines of /etc/crontab: their fixed
// minute, day-of-month, month and day-of-week fields, and the periodic
// argument. The hour is what Times rewrites.
var periodicLines = []struct{ minute, dom, month, dow, what string }{
	{"1", `\*`, `\*`, `\*`, "daily"},
	{"15", `\*`, `\*`, "6", "weekly"},
	{"30", "1", `\*`, `\*`, "monthly"},
}

// periodicLineRE matches one periodic line of /etc/crontab with any hour
// (the hour is \2). BSD sed -E and grep -E share the syntax.
func periodicLineRE(minute, dom, month, dow, what string) string {
	sp := `[[:space:]]+`
	return `^(` + minute + sp + `)([0-9]+)(` + sp + dom + sp + month + sp + dow +
		sp + `root` + sp + `periodic ` + what + `)$`
}

// periodicRewrite returns the sed command that sets the hour of the three
// periodic lines to hour, and the guard that is true while any of them has a
// different hour. Both use the same regexes, so the guard is false exactly
// when the sed would change nothing.
func periodicRewrite(hour string) (sed, guard string) {
	sed = `sed -i '' -E`
	guard = ""
	for _, l := range periodicLines {
		re := periodicLineRE(l.minute, l.dom, l.month, l.dow, l.what)
		sed += fmt.Sprintf(` -e 's/%s/\1%s\3/'`, re, hour)
		if guard != "" {
			guard += " || "
		}
		// A matching line whose hour is not the wanted one.
		guard += fmt.Sprintf(`grep -E '%s' /etc/crontab | grep -Ev '^[0-9]+[[:space:]]+%s[[:space:]]' | grep -q .`, re, hour)
	}
	return sed + " /etc/crontab", guard
}

// DescTimes returns the description for the crontab rewrite.
func (Periodic) DescTimes() string {
	return "Run periodic daily/weekly/monthly in each host's afternoon hour (hosts are off at night)"
}

// Times sets the hour of the periodic lines in /etc/crontab. cron(8)
// re-reads /etc/crontab on change, so no restart is needed.
func (Periodic) Times() {
	EachHost(func(s PeriodicSchedule) {
		sed, guard := periodicRewrite(s.Hour)
		Command("sh", List("-c", sed), OnlyIf("sh", List("-c", guard)))
	})
}

// DescConf returns the description for the periodic.conf settings.
func (Periodic) DescConf() string {
	return "periodic.conf: ZFS scrubs (30-day threshold) on, weekly locate off"
}

// Conf enables the daily periodic scrub check -- each pool is scrubbed once
// its last scrub is older than the threshold, so a scrub missed while a host
// was off happens on the next daily run -- and turns the weekly locate walk
// off. A scrub cut by the nightly power-off resumes on the next import, but
// may redo up to ~2h of work (ZFS checkpoints scrub progress periodically,
// not continuously).
func (Periodic) Conf() {
	File("/etc/periodic.conf",
		WithLine(`daily_scrub_zfs_enable="YES"`),
		WithLine(`daily_scrub_zfs_default_threshold="30"`),
		WithLine(`weekly_locate_enable="NO"`),
		WithMode(0o644))
}

// DescNosuid returns the description for the zdata setuid property.
func (Periodic) DescNosuid() string {
	return "zfs set setuid=off on zdata (NFS data; skipped by the daily setuid walk)"
}

// Nosuid sets setuid=off on the zdata pool root, inherited by the NFS dataset
// and the zrepl sinks. Hosts without zdata (f3) skip it via the guard.
func (Periodic) Nosuid() {
	Command("zfs", List("set", "setuid=off", "zdata"),
		OnlyIf("sh", List("-c", `zfs get -H -o value setuid zdata 2>/dev/null | grep -qx on`)))
}
