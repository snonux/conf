package rocky

import (
	"github.com/snonux/conf/gonf/etchosts"
	"github.com/snonux/conf/gonf/fleetfiles"
	. "github.com/snonux/gonf/api"
)

// HostFile manages the Pi/standalone fleet block; rnodes.Base owns r-node hosts.
type HostFile struct{ RequiresRoot }

func (HostFile) WhenConfig() TaskOption { return WhenHostnameIn("pi2", "pi3", "rocky") }

// Config merges accepted fleet names while retaining unrelated local rows.
func (HostFile) Config() {
	data := etchosts.TemplateData()
	fleetfiles.MergeHosts(append(data.LAN, data.WireGuard...))
}
