package netbsd

import (
	"github.com/snonux/conf/gonf/etchosts"
	"github.com/snonux/conf/gonf/fleetfiles"
	. "github.com/snonux/gonf/api"
)

// HostFile owns the fleet portion of the NetBSD controllers' host databases.
type HostFile struct{ RequiresRoot }

// Config merges accepted fleet names while retaining local and loopback rows.
func (HostFile) Config() {
	data := etchosts.TemplateData()
	fleetfiles.MergeHosts(append(data.LAN, data.WireGuard...))
}
