package dtail

import (
	"github.com/snonux/conf/gonf/migrationaddress"
	. "github.com/snonux/gonf/api"
)

// OptsMigrationCandidate keeps bind candidates out of normal aggregates.
func (Deployment) OptsMigrationCandidate() TaskOptions { return TaskOptions{Operational()} }

// MigrationCandidate stages dserver's new LAN bind without touching the daemon.
// Existing wg0-bound frontend listeners have no LAN migration candidate.
func (Deployment) MigrationCandidate() {
	EachHost(func(host ServerHost) {
		if host.Platform == "openbsd" {
			return
		}
		address, err := migrationaddress.Lab(host.BindAddress)
		if err != nil {
			Refuse("dserver candidate", host.BindAddress, err)
			return
		}
		host.BindAddress = address
		content, err := render(host)
		if err != nil {
			Refuse("dserver candidate", address, err)
			return
		}
		path := "/etc/dserver/dtail.json.triggerfish-candidate"
		if host.Platform == "freebsd" {
			path = "/usr/local" + path
		}
		File(path, WithContent(content), RootOwned)
	})
}
