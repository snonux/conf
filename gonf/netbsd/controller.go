package netbsd

import (
	"github.com/snonux/conf/gonf/endpointfiles"
	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

// Controller owns the accepted full inventory overlay, retaining compiled paths.
// Since p34 the overlay also opts f3sctl into the plugs' native HTTPS
// (shelly_https, with shelly_http_fallback while callers migrate) and this
// owner installs the public f3s Shelly CA certificate at f3sctl's default CA
// path. The CA's private key never leaves the controller host.
type Controller struct{ RequiresRoot }

const (
	shellyCADir  = "/usr/local/etc/f3sctl"
	shellyCAFile = shellyCADir + "/shelly-ca.pem"
)

func (Controller) OptsMigrationCandidate() TaskOptions { return TaskOptions{Operational()} }

// MigrationCandidate stages the full overlay and its recognized legacy input.
func (Controller) MigrationCandidate() {
	installShellyCA()
	InstallFile("/usr/local/etc/f3sctl.json.triggerfish-candidate", paths.GonfAsset("netbsd", "controller.json"), RootOwned)
	InstallFile("/usr/local/etc/f3sctl.json.triggerfish-legacy", paths.GonfAsset("netbsd", "shelly-endpoints.json"), RootOwned)
}

// Config preserves custom configuration and installs the accepted inventory.
func (Controller) Config() {
	legacy := InstallFile("/usr/local/etc/f3sctl.json.triggerfish-legacy", paths.GonfAsset("netbsd", "shelly-endpoints.json"), RootOwned)
	candidate := InstallFile("/usr/local/etc/f3sctl.json.triggerfish-candidate", paths.GonfAsset("netbsd", "controller.json"), RootOwned)
	// The CA is in place before a configuration that requires it is applied.
	ca := installShellyCA()
	Command(endpointfiles.Helper, List("apply", "controller"), DependsOn(legacy, candidate, ca),
		Unless("cmp", List("-s", "/usr/local/etc/f3sctl.json", "/usr/local/etc/f3sctl.json.triggerfish-candidate")))
}

// installShellyCA installs the public CA certificate that f3sctl pins for the
// plugs' HTTPS (world-readable: the CGI runs as _httpd).
func installShellyCA() Dependency {
	EnsureDir(shellyCADir, RootOwned)
	return InstallFile(shellyCAFile, paths.GonfAsset("netbsd", "shelly-ca.pem"), RootOwned)
}
