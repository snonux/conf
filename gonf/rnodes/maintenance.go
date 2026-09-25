// Package rnodes declares Rocky k3s-node maintenance tasks.
package rnodes

import (
	. "github.com/snonux/gonf/api"

	"codeberg.org/snonux/conf/gonf/paths"
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
