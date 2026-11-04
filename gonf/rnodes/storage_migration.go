package rnodes

import (
	"fmt"

	"github.com/snonux/conf/gonf/storagemigration"
	. "github.com/snonux/gonf/api"
)

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
