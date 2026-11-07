package workstation

import (
	"github.com/snonux/conf/gonf/storagemigration"
	. "github.com/snonux/gonf/api"
)

const earthStunnelConfig = `cert = /etc/stunnel/earth-stunnel.pem
CAfile = /etc/stunnel/ca-cert.pem
client = yes
verify = 2

[nfs-ha]
accept = 127.0.0.1:2323
connect = ` + storagemigration.LabVIP + `:2323
`

func storageClientOptions() TaskOptions {
	return TaskOptions{Operational(), WhenLinux(), WhenHostnameIn("earth")}
}

func (System) OptsNFSClientMigrationPrepare() TaskOptions  { return storageClientOptions() }
func (System) OptsNFSClientMigrationArm() TaskOptions      { return storageClientOptions() }
func (System) OptsNFSClientMigrationStage() TaskOptions    { return storageClientOptions() }
func (System) OptsNFSClientMigrationConfirm() TaskOptions  { return storageClientOptions() }
func (System) OptsNFSClientMigrationRollback() TaskOptions { return storageClientOptions() }
func (System) OptsNFSClientMigrationStatus() TaskOptions   { return storageClientOptions() }

// NFSClientMigrationPrepare installs dormant Earth deadline and boot recovery.
func (System) NFSClientMigrationPrepare() { storagemigration.PrepareClient() }

// NFSClientMigrationArm saves Earth's existing transport with a bounded deadline.
func (System) NFSClientMigrationArm() { Sh(storagemigration.ClientHelper + " arm 900") }

// NFSClientMigrationStage moves Earth's endpoint after its NFS mounts are released.
func (System) NFSClientMigrationStage() { Sh(storagemigration.ClientHelper + " stage") }

// NFSClientMigrationConfirm accepts Earth's verified new TLS transport.
func (System) NFSClientMigrationConfirm() { Sh(storagemigration.ClientHelper + " confirm") }

// NFSClientMigrationRollback restores Earth's original TLS transport.
func (System) NFSClientMigrationRollback() { Sh(storagemigration.ClientHelper + " rollback") }

// NFSClientMigrationStatus reports Earth's pending endpoint transaction.
func (System) NFSClientMigrationStatus() { Sh(storagemigration.ClientHelper + " status") }

// OptsNFSClientMigrationCandidate restricts preparation to earth by name.
func (System) OptsNFSClientMigrationCandidate() TaskOptions {
	return TaskOptions{Operational(), WhenLinux(), WhenHostnameIn("earth")}
}

// NFSClientMigrationCandidate stages earth's existing mutual-TLS client path.
// Certificates and both noauto soft NFS mounts remain owned by the host.
func (System) NFSClientMigrationCandidate() {
	File("/etc/stunnel/stunnel.conf"+storagemigration.Suffix, WithContent(earthStunnelConfig), RootOwned)
}

// OptsNFSClientStunnel limits the canonical transport to Earth.
func (System) OptsNFSClientStunnel() TaskOptions {
	return TaskOptions{WhenLinux(), WhenHostnameIn("earth")}
}

// NFSClientStunnel manages Earth's accepted NFS TLS endpoint without restarting transport.
func (System) NFSClientStunnel() {
	File("/etc/stunnel/stunnel.conf", WithContent(earthStunnelConfig), RootOwned)
}
