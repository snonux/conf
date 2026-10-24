package pihole

import (
	"github.com/snonux/conf/gonf/hostfirewall"
	. "github.com/snonux/gonf/api"
)

// Firewall stages the Pi source guard alongside the existing firewalld policy.
type Firewall struct{ RequiresRoot }

// OptsStage excludes firewall staging from ordinary Pi-hole deployments.
func (Firewall) OptsStage() TaskOptions { return TaskOptions{Operational()} }

// Stage installs a candidate and guarded firewall activation tools without loading rules.
func (Firewall) Stage() {
	hostfirewall.Stage(hostfirewall.Profile{Interface: "eth0"})
}
