package netbsd

import (
	"github.com/snonux/conf/gonf/endpointfiles"
	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

// Controller owns the accepted full inventory overlay, retaining compiled paths.
// Since p34 the overlay also opts f3sctl into the plugs' native HTTPS
// (shelly_https; shelly_http_fallback is off since the router dropped TCP80,
// so a TLS or dial failure is an error, never a plain-HTTP retry) and this
// owner installs the public f3s Shelly CA certificate at f3sctl's default CA
// path. The CA's private key never leaves the controller host.
type Controller struct{ RequiresRoot }

const (
	shellyCADir  = "/usr/local/etc/f3sctl"
	shellyCAFile = shellyCADir + "/shelly-ca.pem"

	liveController      = "/usr/local/etc/f3sctl.json"
	controllerCandidate = liveController + ".gonf-candidate"
	controllerLegacy    = liveController + ".gonf-legacy"
	// controllerOwned is the endpoint helper's adopted copy (NetBSD state dir).
	controllerOwned = "/var/db/gonf-endpoint-migration/controller-owned"
)

// Config preserves custom configuration and installs the accepted inventory.
//
// The legacy input and the candidate (f3sctl.json.gonf-{legacy,candidate},
// named .triggerfish-{legacy,candidate} until task f74) are this task's
// staging files: apply publishes the candidate to f3sctl.json, and the
// helper counts a live file equal to either as owned. Their names are
// compiled into the endpoint helper (endpointfiles/assets/apply).
//
// The endpoint helper refuses a live f3sctl.json that is not an owned file
// (legacy input, current candidate, or the copy adopt recorded). adopt runs
// before a new candidate replaces the old one, so an unedited live file stays
// owned across a candidate update (it replaced the arm lease task a74
// retired). apply records what it published as well, so adopt reruns only
// while the live file differs from that copy (a hand-edited file).
func (Controller) Config() {
	helper := endpointfiles.InstallHelper("/usr/local/sbin")
	adopt := Command(endpointfiles.Helper, List("adopt", "controller"), DependsOn(helper),
		Unless("cmp", List("-s", liveController, controllerOwned)))
	legacy := InstallFile(controllerLegacy, paths.GonfAsset("netbsd", "shelly-endpoints.json"), RootOwned, DependsOn(adopt))
	candidate := InstallFile(controllerCandidate, paths.GonfAsset("netbsd", "controller.json"), RootOwned, DependsOn(adopt))
	// The CA is in place before a configuration that requires it is applied.
	ca := installShellyCA()
	Command(endpointfiles.Helper, List("apply", "controller"), DependsOn(legacy, candidate, ca),
		Unless("cmp", List("-s", liveController, controllerCandidate)))
}

// OptsRetireTriggerfishFiles runs Config first, so the renamed staging files
// and the helper that reads them are in place before the old ones go.
func (Controller) OptsRetireTriggerfishFiles() TaskOptions {
	return TaskOptions{Operational(), Needs(Controller.Config)}
}

// RetireTriggerfishFiles removes f3sctl.json.triggerfish-{candidate,legacy} (one-shot, f74).
//
// Config stages the same bytes as f3sctl.json.gonf-{candidate,legacy}
// since task f74, and the updated helper no longer reads the old names;
// f3sctl itself reads only f3sctl.json. Delete this task once pi0 and pi1
// ran it.
func (Controller) RetireTriggerfishFiles() {
	NoFile(List(liveController+".triggerfish-candidate", liveController+".triggerfish-legacy"))
}

// installShellyCA installs the public CA certificate that f3sctl pins for the
// plugs' HTTPS (world-readable: the CGI runs as _httpd).
func installShellyCA() Dependency {
	EnsureDir(shellyCADir, RootOwned)
	return InstallFile(shellyCAFile, paths.GonfAsset("netbsd", "shelly-ca.pem"), RootOwned)
}
