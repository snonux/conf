package freebsd

import (
	"fmt"
	"strings"

	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// Zrepl manages zrepl on the f-hosts (f3s-storage skill, references/zrepl.md):
// the package, /usr/local/etc/zrepl/zrepl.yml and the service. Until
// 2026-09-25 each zrepl.yml was a hand-written file (task hk2); the live
// files, not the blog posts, were the source of truth for the template:
//   - f0 pushes zdata/enc/nfsdata to f1 every minute (f0_to_f1_nfsdata);
//   - f1 is its sink (zdata/sink);
//   - f3 pushes the freebsd and rocky VM datasets to f2 every 10 minutes
//     (f3_to_f2_freebsd; the name predates the rocky VM);
//   - f2 is that sink (zroot/sink: f2 has no second disk);
//   - every host runs the local_zfs_snapshots snap job (daily at 03:00),
//     excluding the datasets a push job or sink already snapshots.
//
// The file is rendered on the controller from the host's ZreplJobs and the
// template f3s/freebsd-hosts/zrepl/zrepl.yml.tmpl, validated on the host
// with "zrepl configcheck" before it replaces the live file (a failing check
// leaves the live file alone), and zrepl is restarted only when the file
// changed. The restart drops a replication step in flight; zrepl resumes it
// (resumable receives) on the next interval, and the sink side just sees the
// push reconnect.
//
// The package's logging hooks stay as the pkg installed them:
// /usr/local/etc/newsyslog.conf.d/zrepl.conf is the pkg's example
// (share/examples/zrepl/newsyslog.conf) byte for byte, and the weekly
// periodic script 500.zrepl is part of the package.
//
// Register with OnCluster(cluster.NameFreeBSD); every f-host carries a
// ZreplJobs.
type Zrepl struct {
	RequiresRoot
}

const zreplConf = "/usr/local/etc/zrepl/zrepl.yml"

// ZreplJobs is a FreeBSD host's zrepl setup (host data, see gonf/cluster):
// at most one of Push and Sink, plus the filter of the local snap job.
type ZreplJobs struct {
	Push *ZreplPush
	Sink *ZreplSink
	// Snap is the local_zfs_snapshots filesystems filter, in file order.
	Snap []ZreplFilesystem
}

// ZreplPush is a push job: an encrypted (raw) send of Filesystems to the
// sink at Address, snapshotted every Interval and pruned on both sides by
// Grid (plus the last 10 snapshots).
type ZreplPush struct {
	Name        string
	Peer        string // the sink host's inventory name, for the comment
	Address     string // the sink's WireGuard ip:port
	Filesystems []string
	Interval    string // zrepl duration, e.g. "1m", "10m"
	Grid        string
}

// ZreplSink is a sink job serving one push client on Listen (the host's
// WireGuard ip:port) and receiving below RootFS.
type ZreplSink struct {
	Listen     string
	ClientIP   string
	ClientName string
	RootFS     string
}

// ZreplFilesystem is one entry of a filesystems filter ("zroot<": true).
type ZreplFilesystem struct {
	Pattern string
	Include bool
}

// zreplTemplateData is zrepl.yml.tmpl's root. RenderTemplate passes data
// through JSON, so it carries the values the template prints, already
// derived (the host name, the interval comment), rather than methods.
type zreplTemplateData struct {
	Push *zreplPushData
	Sink *zreplSinkData
	Snap []ZreplFilesystem
}

type zreplPushData struct {
	ZreplPush
	IntervalNote string
}

type zreplSinkData struct {
	ZreplSink
	Self string
}

// zreplTemplateDataFor derives the template root of host's jobs.
func zreplTemplateDataFor(host string, jobs ZreplJobs) zreplTemplateData {
	data := zreplTemplateData{Snap: jobs.Snap}
	if jobs.Push != nil {
		data.Push = &zreplPushData{ZreplPush: *jobs.Push, IntervalNote: intervalNote(jobs.Push.Interval)}
	}
	if jobs.Sink != nil {
		data.Sink = &zreplSinkData{ZreplSink: *jobs.Sink, Self: host}
	}
	return data
}

// intervalNote spells a minute interval out for the comment next to it:
// "1m" is "every minute", "10m" is "every 10 minutes". Any other duration
// is quoted as is.
func intervalNote(interval string) string {
	minutes, ok := strings.CutSuffix(interval, "m")
	switch {
	case !ok || minutes == "":
		return "every " + interval
	case minutes == "1":
		return "every minute"
	default:
		return "every " + minutes + " minutes"
	}
}

// renderZrepl renders host's zrepl.yml.
func renderZrepl(host string, jobs ZreplJobs) (string, error) {
	if jobs.Push != nil && jobs.Sink != nil {
		return "", fmt.Errorf("zrepl on %s: a host is either a push source or a sink, not both", host)
	}
	rendered, err := RenderTemplate(paths.FHostAsset("zrepl/zrepl.yml.tmpl"), zreplTemplateDataFor(host, jobs))
	if err != nil {
		return "", fmt.Errorf("render zrepl.yml.tmpl for %s: %w", host, err)
	}
	return rendered, nil
}

// DescPackage returns the description for the zrepl package.
func (Zrepl) DescPackage() string {
	return "Install zrepl on the f-hosts"
}

// Package installs zrepl (its rc.d script and config directory).
func (Zrepl) Package() {
	Packages("zrepl")
}

// DescConfig returns the description for zrepl.yml and the service.
func (Zrepl) DescConfig() string {
	return "zrepl.yml per f-host (push/sink + local snap job), zrepl configcheck before install, restart zrepl on change"
}

// OptsConfig records the package (and /usr/local/etc/zrepl) first.
func (Zrepl) OptsConfig() TaskOptions {
	return TaskOptions{Needs("package")}
}

// Config installs the rendered zrepl.yml and keeps zrepl enabled and
// running (zrepl_enable="YES", as all four hosts had it). WithValidation
// runs "zrepl configcheck" on a candidate next to the live file; a failing
// check keeps the live file and fires no restart.
func (Zrepl) Config() {
	EachHostNamed(func(host string, jobs ZreplJobs) {
		text, err := renderZrepl(host, jobs)
		if err != nil {
			Refuse("File", zreplConf, err)
			return
		}
		conf := File(zreplConf, WithContent(text),
			WithValidation("zrepl", List("configcheck", "--config", CandidatePath)),
			Perm(0o644, Root))
		Service("zrepl", WithRestart, OnChange(conf))
	})
}
