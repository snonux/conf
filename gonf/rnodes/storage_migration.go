package rnodes

import (
	"fmt"

	"github.com/snonux/conf/gonf/storagemigration"
	. "github.com/snonux/gonf/api"
)

func (Maintenance) OptsNFSClientMigrationPrepare() TaskOptions  { return TaskOptions{Operational()} }
func (Maintenance) OptsNFSClientMigrationArm() TaskOptions      { return TaskOptions{Operational()} }
func (Maintenance) OptsNFSClientMigrationStage() TaskOptions    { return TaskOptions{Operational()} }
func (Maintenance) OptsNFSClientMigrationConfirm() TaskOptions  { return TaskOptions{Operational()} }
func (Maintenance) OptsNFSClientMigrationRollback() TaskOptions { return TaskOptions{Operational()} }
func (Maintenance) OptsNFSClientMigrationStatus() TaskOptions   { return TaskOptions{Operational()} }

// NFSClientMigrationPrepare installs dormant deadline and pre-stunnel boot recovery.
func (Maintenance) NFSClientMigrationPrepare() {
	storagemigration.PrepareClient()
}

// NFSClientMigrationArm saves the existing transport with a fifteen-minute deadline.
func (Maintenance) NFSClientMigrationArm() { Sh(storagemigration.ClientHelper + " arm 900") }

// NFSClientMigrationStage moves the TLS endpoint while writers and monitors are held.
func (Maintenance) NFSClientMigrationStage() { Sh(storagemigration.ClientHelper + " stage") }

// NFSClientMigrationConfirm accepts the verified new transport without retiring OLD.
func (Maintenance) NFSClientMigrationConfirm() { Sh(storagemigration.ClientHelper + " confirm") }

// NFSClientMigrationRollback restores the original transport without changing mounts.
func (Maintenance) NFSClientMigrationRollback() { Sh(storagemigration.ClientHelper + " rollback") }

// NFSClientMigrationStatus reports the client transaction without TLS credentials.
func (Maintenance) NFSClientMigrationStatus() { Sh(storagemigration.ClientHelper + " status") }

// OptsNFSClientMigrationCandidate excludes client preparation from aggregates.
func (Maintenance) OptsNFSClientMigrationCandidate() TaskOptions { return TaskOptions{Operational()} }

// NFSClientMigrationCandidate stages transport changes without touching mounts.
func (Maintenance) NFSClientMigrationCandidate() {
	EachHostNamed(func(host string, _ K3sNode) {
		content, err := storagemigration.Client(fmt.Sprintf(stunnelConfFormat, host))
		if err != nil {
			Refuse("NFS client candidate", host, err)
			return
		}
		File("/etc/stunnel/stunnel.conf"+storagemigration.Suffix, WithContent(content), RootOwned)
	})
}
