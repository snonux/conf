package freebsd

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// Monitoring manages what Prometheus scrapes from the f-hosts
// (f3s-observability skill, references/freebsd.md; the scrape targets live in
// f3s/prometheus/additional-scrape-configs.yaml): node_exporter, bound to the
// host's WireGuard address, and the ZFS pool/dataset metrics its textfile
// collector exposes. Until 2026-09-25 all of it was hand-made (blog part 8)
// and had drifted (task kk2):
//   - rc.conf set only node_exporter_args="--web.listen-address=<wg0>:9100".
//     The port's rc.d script always passes its own --web.listen-address
//     (node_exporter_listen_address, default ":9100") as well, so every
//     node_exporter also listened on all interfaces, the LAN included. The
//     listen address now goes into node_exporter_listen_address and the args
//     are empty, which leaves only the wg0 socket.
//   - zfs_pool_metrics.sh was in no repository, owned by paul, and missing on
//     f3 (whose zroot had no pool metrics at all). It is now installed
//     root-owned on all four from f3s/freebsd-hosts/node-exporter/.
//
// Register with OnCluster(cluster.NameFreeBSD); every f-host carries a
// NodeExporterHost.
type Monitoring struct {
	RequiresRoot
}

// NodeExporterHost is a FreeBSD host's node_exporter setting (host data, see
// gonf/cluster).
type NodeExporterHost struct {
	// ListenAddress is the "ip:port" node_exporter binds: the host's wg0
	// address, where Prometheus scrapes it.
	ListenAddress string
}

const (
	// nodeExporterTextfileDir is the port's default textfile directory, set
	// explicitly so the script and the exporter provably agree.
	nodeExporterTextfileDir = "/var/tmp/node_exporter"
	zfsMetricsScript        = "/usr/local/bin/zfs_pool_metrics.sh"
	// zfsMetricsCommand is the hand-added root crontab line of f0-f2 byte for
	// byte, so CronAt adopts it there.
	zfsMetricsCommand = zfsMetricsScript + " >/dev/null 2>&1"
)

// DescPackage returns the description for the node_exporter package.
func (Monitoring) DescPackage() string {
	return "Install node_exporter"
}

// Package installs node_exporter.
func (Monitoring) Package() {
	Packages("node_exporter")
}

// DescNodeExporter returns the description for the node_exporter service.
func (Monitoring) DescNodeExporter() string {
	return "rc.conf node_exporter keys (wg0 listen address, textfile dir); enable, run, restart on change"
}

// OptsNodeExporter records the package (and its rc.d script) first.
func (Monitoring) OptsNodeExporter() TaskOptions {
	return TaskOptions{Needs("package")}
}

// NodeExporter owns the node_exporter_* rc.conf lines via WithKeyedLine
// (the rest of rc.conf is shared with other tasks, hence WithName), the
// textfile directory (as the rc.d start_precmd creates it: nobody, 1755),
// and keeps the service enabled and running. It restarts only when one of
// its rc.conf lines changed.
func (Monitoring) NodeExporter() {
	EachHost(func(h NodeExporterHost) {
		conf := File(rcConf, WithName("rc.conf node_exporter"), Perm(0o644, Root),
			WithKeyedLine("node_exporter_listen_address=", `node_exporter_listen_address="`+h.ListenAddress+`"`),
			WithKeyedLine("node_exporter_textfile_dir=", `node_exporter_textfile_dir="`+nodeExporterTextfileDir+`"`),
			WithKeyedLine("node_exporter_args=", `node_exporter_args=""`))
		EnsureDir(nodeExporterTextfileDir, Perm(0o1755, "nobody:nobody"))
		Service("node_exporter", WithRestart, OnChange(conf))
	})
}

// DescZFSMetrics returns the description for the ZFS textfile metrics.
func (Monitoring) DescZFSMetrics() string {
	return "Install " + zfsMetricsScript + " (ZFS pool/dataset textfile metrics) and its every-minute root cron job"
}

// OptsZFSMetrics records the textfile directory first.
func (Monitoring) OptsZFSMetrics() TaskOptions {
	return TaskOptions{Needs("node_exporter")}
}

// ZFSMetrics installs the script root-owned (it runs as root from cron) and
// schedules it every minute; the script itself publishes atomically into the
// textfile directory, where node_exporter reads it on each scrape.
func (Monitoring) ZFSMetrics() {
	script := InstallFile(zfsMetricsScript, paths.FHostAsset("node-exporter/zfs_pool_metrics.sh"),
		Perm(0o755, Root))
	CronAt("zfs-pool-metrics", "* * * * *", zfsMetricsCommand, DependsOn(script))
}
