// Package rnodes declares Rocky k3s-node maintenance tasks.
package rnodes

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// Maintenance carries the root-only r-node maintenance tasks. Register it
// with OnCluster(cluster.NameRockyK3s) so every task applies on r0, r1, and
// r2 only.
type Maintenance struct {
	RequiresRoot
}

// NFSMountMonitor installs and enables the r-node NFS mount monitor.
//
// NFSMountMonitor installs the health check, its systemd units, and the
// shutdown ordering helpers. Every changed managed input reloads systemd once
// and restarts the timer once; the marker and drain services still converge to
// enabled and active on every run. Split into three phases (dirs, files,
// systemd wiring) that run in this order.
func (Maintenance) NFSMountMonitor() {
	ensureNFSMountMonitorDirs()
	inputs := installNFSMountMonitorFiles("nfs-mount-monitor")
	activateNFSMountMonitorUnits(inputs)
}

// ensureNFSMountMonitorDirs creates the state, node_exporter textfile
// collector, and systemd drop-in directories the monitor's files land in.
func ensureNFSMountMonitorDirs() {
	EnsureDir("/var/lib/nfs-mount-monitor",
		RootPrivate)
	EnsureDir("/var/lib/node_exporter",
		RootOwned)
	EnsureDir("/var/lib/node_exporter/textfile_collector",
		RootOwned)
	EnsureDir("/etc/systemd/system/data-nfs-k3svolumes.mount.d",
		RootOwned)
	EnsureDir("/etc/systemd/system/k3s.service.d",
		RootOwned)
}

// installNFSMountMonitorFiles installs the monitor script, its systemd
// units, and the shutdown/ordering helpers from monitorDir's assets. It
// returns them in the same order NFSMountMonitor used to feed FanIn, since
// activateNFSMountMonitorUnits only needs them as an undifferentiated
// change-fan-in set.
func installNFSMountMonitorFiles(monitorDir string) []Resource {
	return []Resource{
		InstallFile("/usr/local/bin/check-nfs-mount.sh",
			paths.RNodeAsset(monitorDir+"/check-nfs-mount.sh"),
			RootExec),
		InstallFile("/etc/default/nfs-mount-monitor",
			paths.RNodeAsset(monitorDir+"/nfs-mount-monitor.default"),
			RootOwned),
		InstallFile("/etc/systemd/system/nfs-mount-monitor.service",
			paths.RNodeAsset(monitorDir+"/nfs-mount-monitor.service"),
			RootOwned),
		InstallFile("/etc/systemd/system/nfs-mount-monitor.timer",
			paths.RNodeAsset(monitorDir+"/nfs-mount-monitor.timer"),
			RootOwned),
		InstallFile("/etc/systemd/system/nfs-shutdown-marker.service",
			paths.RNodeAsset(monitorDir+"/nfs-shutdown-marker.service"),
			RootOwned),
		InstallFile("/usr/local/bin/k3s-nfs-drain.sh",
			paths.RNodeAsset(monitorDir+"/k3s-nfs-drain.sh"),
			RootExec),
		InstallFile("/etc/systemd/system/k3s-nfs-drain.service",
			paths.RNodeAsset(monitorDir+"/k3s-nfs-drain.service"),
			RootOwned),
		InstallFile(
			"/etc/systemd/system/data-nfs-k3svolumes.mount.d/10-stunnel-ordering.conf",
			paths.RNodeAsset(monitorDir+"/nfs-stunnel-ordering.conf"),
			RootOwned),
		InstallFile("/etc/systemd/system/k3s.service.d/10-nfs-ordering.conf",
			paths.RNodeAsset(monitorDir+"/k3s-nfs-ordering.conf"),
			RootOwned),
	}
}

// activateNFSMountMonitorUnits wires inputs (from installNFSMountMonitorFiles)
// as the FanIn change set: any changed file reloads systemd once and
// restarts the timer once, while the marker and drain services still
// converge to enabled and active on every run.
func activateNFSMountMonitorUnits(inputs []Resource) {
	SystemdUnits(
		FanIn(inputs...),
		ActivateTimer("nfs-mount-monitor", WithRestart),
		ActivateServices(List("nfs-shutdown-marker", "k3s-nfs-drain")),
	)
}

// PersistentJournal enables bounded persistent systemd journals on r-nodes.
//
// PersistentJournal creates persistent journal storage and installs its
// journald drop-in. A changed drop-in creates journal state and restarts
// journald before the unconditional journal flush.
func (Maintenance) PersistentJournal() {
	EnsureDir("/etc/systemd/journald.conf.d", RootOwned)
	EnsureDir("/var/log/journal", Perm(0o2755, "root:systemd-journal"))

	dropIn := InstallFile("/etc/systemd/journald.conf.d/10-persistent.conf",
		paths.RNodeAsset("journald-persistent.conf"),
		RootOwned)
	tmpfiles := Sh("systemd-tmpfiles --create --prefix /var/log/journal", OnChange(dropIn))
	journald := Sh("systemctl restart systemd-journald", OnChange(dropIn))
	Sh("journalctl --flush", DependsOn(tmpfiles, journald))
}

// nfsUnderlyingCheck runs its argument against the directory the NFS mount
// covers, reached through a private bind mount of / (the mount itself hides
// it). $U is that directory; the bind mount is removed afterwards.
func nfsUnderlyingCheck(body string) string {
	return `set -e; B=$(mktemp -d /run/nfsunder.XXXXXX); mount --bind / "$B"; ` +
		`U="$B/data/nfs/k3svolumes"; trap 'umount "$B"; rmdir "$B"' EXIT; ` + body
}

// NFSMountpointGuard makes the empty directory under the NFS mount immutable
// (no local writes when NFS is absent).
//
// NFSMountpointGuard sets chattr +i on the empty directory under
// /data/nfs/k3svolumes. Pods whose hostPath PVs point into the NFS mount used
// to start while it was missing (boot races, repair windows) and wrote to the
// node's root disk instead: fresh Postgres clusters, Valkey dumps, app config,
// 94-429 MB per node, removed on 2026-09-25. With the directory immutable,
// such a pod fails to start until NFS is back, and mounting over the
// directory still works.
//
// It only acts on an empty directory: leftover content means a pod already
// wrote locally, which needs a human to look at it, so the guard stays false
// and the task keeps reporting a pending change.
func (Maintenance) NFSMountpointGuard() {
	Command("sh", List("-c", nfsUnderlyingCheck(
		`[ -z "$(ls -A "$U")" ] && chattr +i "$U"`)),
		OnlyIf("sh", List("-c", nfsUnderlyingCheck(
			`! lsattr -d "$U" | cut -d' ' -f1 | grep -q i`))))
}

// ImageGC installs the k3s kubelet image-GC drop-in (collect at 70% disk,
// down to 60%).
//
// ImageGC installs /etc/rancher/k3s/config.yaml.d/50-image-gc.yaml. It does
// not restart k3s: this task runs on r0-r2 in parallel, and restarting all
// three control-plane members at once would drop etcd quorum. The r-VMs
// reboot with the nightly f-host power-off, so the drop-in is live by the next
// morning; restart k3s by hand, one node at a time, to apply it sooner.
func (Maintenance) ImageGC() {
	EnsureDir("/etc/rancher/k3s/config.yaml.d", RootOwned)
	InstallFile("/etc/rancher/k3s/config.yaml.d/50-image-gc.yaml",
		paths.RNodeAsset("k3s-image-gc.yaml"), RootPrivate)
}

// RetiredNFSCron removes the legacy per-minute check-nfs-mount.sh root cron
// line (the timer runs it).
//
// RetiredNFSCron removes the hand-added root crontab line that ran
// check-nfs-mount.sh every minute on top of nfs-mount-monitor.timer (found
// 2026-09-25): two schedulers for one repair script, with a log file
// (/var/log/nfs-mount-check.log) that was never rotated.
//
// A guarded Command rather than NoCron: NoCron only removes gonf-managed
// blocks, and WithLegacyCommand is only honoured when a Cron is adopted, not
// when one is removed, so the unmanaged line needs an exact-match delete.
func (Maintenance) RetiredNFSCron() {
	const line = "* * * * * /usr/local/bin/check-nfs-mount.sh >> /var/log/nfs-mount-check.log 2>&1"
	Command("sh", List("-c", `crontab -l | grep -vxF '`+line+`' | crontab -`),
		OnlyIf("sh", List("-c", `crontab -l 2>/dev/null | grep -qxF '`+line+`'`)))
	NoFile("/var/log/nfs-mount-check.log")
}
