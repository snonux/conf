package netbsd

import (
	"github.com/snonux/conf/gonf/endpointfiles"
	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

// Controller owns the accepted full inventory overlay, retaining compiled paths.
type Controller struct{ RequiresRoot }

func (Controller) OptsMigrationCandidate() TaskOptions { return TaskOptions{Operational()} }

// MigrationCandidate stages the full overlay and its recognized legacy input.
func (Controller) MigrationCandidate() {
	InstallFile("/usr/local/etc/f3sctl.json.triggerfish-candidate", paths.GonfAsset("netbsd", "controller.json"), RootOwned)
	InstallFile("/usr/local/etc/f3sctl.json.triggerfish-legacy", paths.GonfAsset("netbsd", "shelly-endpoints.json"), RootOwned)
}

// Config preserves custom configuration and installs the accepted inventory.
func (Controller) Config() {
	legacy := InstallFile("/usr/local/etc/f3sctl.json.triggerfish-legacy", paths.GonfAsset("netbsd", "shelly-endpoints.json"), RootOwned)
	candidate := InstallFile("/usr/local/etc/f3sctl.json.triggerfish-candidate", paths.GonfAsset("netbsd", "controller.json"), RootOwned)
	Command(endpointfiles.Helper, List("apply", "controller"), DependsOn(legacy, candidate),
		Unless("cmp", List("-s", "/usr/local/etc/f3sctl.json", "/usr/local/etc/f3sctl.json.triggerfish-candidate")))
}
