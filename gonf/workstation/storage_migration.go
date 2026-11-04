package workstation

import (
	"github.com/snonux/conf/gonf/storagemigration"
	. "github.com/snonux/gonf/api"
)

// OptsNFSClientMigrationCandidate restricts preparation to earth by name.
func (System) OptsNFSClientMigrationCandidate() TaskOptions {
	return TaskOptions{Operational(), WhenLinux(), WhenHostnameIn("earth")}
}

// NFSClientMigrationCandidate stages earth's existing mutual-TLS client path.
// Certificates and both noauto soft NFS mounts remain owned by the host.
func (System) NFSClientMigrationCandidate() {
	File("/etc/stunnel/stunnel.conf"+storagemigration.Suffix, WithContent(`cert = /etc/stunnel/earth-stunnel.pem
CAfile = /etc/stunnel/ca-cert.pem
client = yes
verify = 2

[nfs-ha]
accept = 127.0.0.1:2323
connect = `+storagemigration.LabVIP+`:2323
`), RootOwned)
}
