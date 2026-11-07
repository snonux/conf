package storagemigration

import (
	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

// ClientHelper guards the existing Linux NFS-over-TLS clients.
const ClientHelper = "/usr/local/sbin/gonf-storage-client"

// PrepareClient installs dormant endpoint recovery without changing transport.
// Callers supply their existing host/profile guards; this is not an aggregate.
func PrepareClient() {
	check := Command("test", List("!", "-e", "/var/lib/gonf-storage-client/pending"))
	python := Command("test", List("-x", "/usr/bin/python3"))
	EnsureDir("/var/lib/gonf-storage-client", RootPrivate, DependsOn(check))
	EnsureDir("/etc/systemd/system/stunnel.service.d", RootOwned)
	helper := InstallFile(ClientHelper, paths.RNodeAsset("storage-migration/client.py"), RootExec, DependsOn(check, python))
	unit := InstallFile("/etc/systemd/system/gonf-storage-client-rollback.service", paths.RNodeAsset("storage-migration/gonf-storage-client-rollback.service"), RootOwned, DependsOn(helper))
	timer := InstallFile("/etc/systemd/system/gonf-storage-client-rollback.timer", paths.RNodeAsset("storage-migration/gonf-storage-client-rollback.timer"), RootOwned, DependsOn(helper))
	dropin := InstallFile("/etc/systemd/system/stunnel.service.d/50-gonf-storage-rollback.conf", paths.RNodeAsset("storage-migration/stunnel-rollback.conf"), RootOwned, DependsOn(helper))
	SystemdUnits(FanIn(unit, timer, dropin), ActivateTimer("gonf-storage-client-rollback", WithRestart))
}
