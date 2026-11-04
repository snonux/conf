package triggerfish

import (
	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

// VPN prepares the separately guarded IPv4 management tunnel.
type VPN struct{ RequiresRoot }

// OptsCandidate excludes private tunnel preparation from routine deployment.
func (VPN) OptsCandidate() TaskOptions { return TaskOptions{Operational()} }

// Candidate stages the private wg2 input without creating an interface.
func (VPN) Candidate() {
	EnsureDir("/etc/gonf/router-profiles/vpn-bootstrap", RootPrivate)
	File("/etc/gonf/router-profiles/vpn-bootstrap/hostname.wg2",
		WithContent(MustSecret(paths.TriggerfishSecret("hostname.wg2"))), RootPrivate)
}
