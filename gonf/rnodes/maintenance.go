// Package rnodes declares Rocky k3s-node maintenance tasks.
package rnodes

import (
	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"

	"codeberg.org/snonux/conf/gonf/paths"
)

// Maintenance carries the root-only r-node maintenance tasks. Register it
// with cluster.NameRockyK3s so ClusterHosts resolves to r0, r1, and r2.
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
// enabled and active on every run.
func (Maintenance) NFSMountMonitor() {
	WhenHostname(ClusterHosts(), func() {
		monitorDir := "nfs-mount-monitor"

		EnsureDir("/var/lib/nfs-mount-monitor",
			WithMode(0o700), WithOwner("root"), WithGroup("root"))
		EnsureDir("/var/lib/node_exporter",
			WithMode(0o755), WithOwner("root"), WithGroup("root"))
		EnsureDir("/var/lib/node_exporter/textfile_collector",
			WithMode(0o755), WithOwner("root"), WithGroup("root"))
		EnsureDir("/etc/systemd/system/data-nfs-k3svolumes.mount.d",
			WithMode(0o755), WithOwner("root"), WithGroup("root"))
		EnsureDir("/etc/systemd/system/k3s.service.d",
			WithMode(0o755), WithOwner("root"), WithGroup("root"))

		check := InstallFile("/usr/local/bin/check-nfs-mount.sh",
			paths.RNodeAsset(monitorDir+"/check-nfs-mount.sh"),
			WithMode(0o755), WithOwner("root"), WithGroup("root"))
		defaults := InstallFile("/etc/default/nfs-mount-monitor",
			paths.RNodeAsset(monitorDir+"/nfs-mount-monitor.default"),
			WithMode(0o644), WithOwner("root"), WithGroup("root"))
		monitorService := InstallFile("/etc/systemd/system/nfs-mount-monitor.service",
			paths.RNodeAsset(monitorDir+"/nfs-mount-monitor.service"),
			WithMode(0o644), WithOwner("root"), WithGroup("root"))
		monitorTimer := InstallFile("/etc/systemd/system/nfs-mount-monitor.timer",
			paths.RNodeAsset(monitorDir+"/nfs-mount-monitor.timer"),
			WithMode(0o644), WithOwner("root"), WithGroup("root"))
		shutdownMarker := InstallFile("/etc/systemd/system/nfs-shutdown-marker.service",
			paths.RNodeAsset(monitorDir+"/nfs-shutdown-marker.service"),
			WithMode(0o644), WithOwner("root"), WithGroup("root"))
		drainScript := InstallFile("/usr/local/bin/k3s-nfs-drain.sh",
			paths.RNodeAsset(monitorDir+"/k3s-nfs-drain.sh"),
			WithMode(0o755), WithOwner("root"), WithGroup("root"))
		drainService := InstallFile("/etc/systemd/system/k3s-nfs-drain.service",
			paths.RNodeAsset(monitorDir+"/k3s-nfs-drain.service"),
			WithMode(0o644), WithOwner("root"), WithGroup("root"))
		stunnelOrdering := InstallFile(
			"/etc/systemd/system/data-nfs-k3svolumes.mount.d/10-stunnel-ordering.conf",
			paths.RNodeAsset(monitorDir+"/nfs-stunnel-ordering.conf"),
			WithMode(0o644), WithOwner("root"), WithGroup("root"))
		k3sOrdering := InstallFile("/etc/systemd/system/k3s.service.d/10-nfs-ordering.conf",
			paths.RNodeAsset(monitorDir+"/k3s-nfs-ordering.conf"),
			WithMode(0o644), WithOwner("root"), WithGroup("root"))

		SystemdUnits(
			FanIn(check, defaults, monitorService, monitorTimer, shutdownMarker,
				drainScript, drainService, stunnelOrdering, k3sOrdering),
			ActivateTimer("nfs-mount-monitor", WithRestart),
			ActivateServices(List("nfs-shutdown-marker", "k3s-nfs-drain")),
		)
	})
}

// DescPersistentJournal returns the persistent journal task description.
func (Maintenance) DescPersistentJournal() string {
	return "Enable bounded persistent systemd journals on r-nodes"
}

// PersistentJournal creates persistent journal storage and installs its
// journald drop-in. A changed drop-in creates journal state and restarts
// journald before the unconditional journal flush.
func (Maintenance) PersistentJournal() {
	WhenHostname(ClusterHosts(), func() {
		EnsureDir("/etc/systemd/journald.conf.d",
			WithMode(0o755), WithOwner("root"), WithGroup("root"))
		EnsureDir("/var/log/journal",
			WithMode(0o2755), WithOwner("root"), WithGroup("systemd-journal"))

		dropIn := InstallFile("/etc/systemd/journald.conf.d/10-persistent.conf",
			paths.RNodeAsset("journald-persistent.conf"),
			WithMode(0o644), WithOwner("root"), WithGroup("root"))
		tmpfiles := Command("systemd-tmpfiles", List("--create", "--prefix", "/var/log/journal"),
			OnChange(dropIn))
		journald := Command("systemctl", List("restart", "systemd-journald"), OnChange(dropIn))
		Command("journalctl", List("--flush"), DependsOn(tmpfiles, journald))
	})
}
