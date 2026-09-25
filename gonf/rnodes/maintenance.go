// Package rnodes declares Rocky k3s-node maintenance tasks.
package rnodes

import (
	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"

	"github.com/snonux/conf/gonf/paths"
)

// Maintenance carries the root-only r-node maintenance tasks. Register it
// with OnCluster(cluster.NameRockyK3s) so every task applies on r0, r1, and
// r2 only.
type Maintenance struct {
	RequiresRoot
}

// DescNFSMountMonitor returns the NFS mount monitor task description.
func (Maintenance) DescNFSMountMonitor() string {
	return "Install and enable the r-node NFS mount monitor"
}

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
		Perm(0o700, Root))
	EnsureDir("/var/lib/node_exporter",
		Perm(0o755, Root))
	EnsureDir("/var/lib/node_exporter/textfile_collector",
		Perm(0o755, Root))
	EnsureDir("/etc/systemd/system/data-nfs-k3svolumes.mount.d",
		Perm(0o755, Root))
	EnsureDir("/etc/systemd/system/k3s.service.d",
		Perm(0o755, Root))
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
			Perm(0o755, Root)),
		InstallFile("/etc/default/nfs-mount-monitor",
			paths.RNodeAsset(monitorDir+"/nfs-mount-monitor.default"),
			Perm(0o644, Root)),
		InstallFile("/etc/systemd/system/nfs-mount-monitor.service",
			paths.RNodeAsset(monitorDir+"/nfs-mount-monitor.service"),
			Perm(0o644, Root)),
		InstallFile("/etc/systemd/system/nfs-mount-monitor.timer",
			paths.RNodeAsset(monitorDir+"/nfs-mount-monitor.timer"),
			Perm(0o644, Root)),
		InstallFile("/etc/systemd/system/nfs-shutdown-marker.service",
			paths.RNodeAsset(monitorDir+"/nfs-shutdown-marker.service"),
			Perm(0o644, Root)),
		InstallFile("/usr/local/bin/k3s-nfs-drain.sh",
			paths.RNodeAsset(monitorDir+"/k3s-nfs-drain.sh"),
			Perm(0o755, Root)),
		InstallFile("/etc/systemd/system/k3s-nfs-drain.service",
			paths.RNodeAsset(monitorDir+"/k3s-nfs-drain.service"),
			Perm(0o644, Root)),
		InstallFile(
			"/etc/systemd/system/data-nfs-k3svolumes.mount.d/10-stunnel-ordering.conf",
			paths.RNodeAsset(monitorDir+"/nfs-stunnel-ordering.conf"),
			Perm(0o644, Root)),
		InstallFile("/etc/systemd/system/k3s.service.d/10-nfs-ordering.conf",
			paths.RNodeAsset(monitorDir+"/k3s-nfs-ordering.conf"),
			Perm(0o644, Root)),
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

// DescPersistentJournal returns the persistent journal task description.
func (Maintenance) DescPersistentJournal() string {
	return "Enable bounded persistent systemd journals on r-nodes"
}

// PersistentJournal creates persistent journal storage and installs its
// journald drop-in. A changed drop-in creates journal state and restarts
// journald before the unconditional journal flush.
func (Maintenance) PersistentJournal() {
	EnsureDir("/etc/systemd/journald.conf.d", Perm(0o755, Root))
	EnsureDir("/var/log/journal", Perm(0o2755, "root:systemd-journal"))

	dropIn := InstallFile("/etc/systemd/journald.conf.d/10-persistent.conf",
		paths.RNodeAsset("journald-persistent.conf"),
		Perm(0o644, Root))
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

// DescNFSMountpointGuard returns the mountpoint guard task description.
func (Maintenance) DescNFSMountpointGuard() string {
	return "Make the empty directory under the NFS mount immutable (no local writes when NFS is absent)"
}

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

// DescImageGC returns the kubelet image-GC drop-in task description.
func (Maintenance) DescImageGC() string {
	return "Install the k3s kubelet image-GC drop-in (collect at 70% disk, down to 60%)"
}

// ImageGC installs /etc/rancher/k3s/config.yaml.d/50-image-gc.yaml. It does
// not restart k3s: this task runs on r0-r2 in parallel, and restarting all
// three control-plane members at once would drop etcd quorum. The r-VMs
// reboot with the nightly f-host power-off, so the drop-in is live by the next
// morning; restart k3s by hand, one node at a time, to apply it sooner.
func (Maintenance) ImageGC() {
	EnsureDir("/etc/rancher/k3s/config.yaml.d", Perm(0o755, Root))
	InstallFile("/etc/rancher/k3s/config.yaml.d/50-image-gc.yaml",
		paths.RNodeAsset("k3s-image-gc.yaml"), Perm(0o600, Root))
}
