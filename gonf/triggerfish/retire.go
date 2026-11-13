package triggerfish

import . "github.com/snonux/gonf/api"

// g74 one-shot (IQ openbsdrouter-plan CLEANUP.md item 10a): the router copies
// of the pruned profiles, their readiness receipts, the t14 rejected-stage
// archive and the previous journal. Delete this file once it has run.
const routerGonf = "/etc/gonf"

// prunedProfiles are the profiles g74 removed from the renderer and helper:
// the shared-wire stages, wifi-ipv6-uplink and the wg2 vpn-bootstrap input.
var prunedProfiles = []string{"shared-igc0", "wifi-prepare", "wifi-first", "post-rack",
	"post-ap", "final", "wifi-ipv6-uplink", "vpn-bootstrap"}

// OptsRetire keeps the one-shot out of routine deployment.
func (Profiles) OptsRetire() TaskOptions { return TaskOptions{Operational()} }

// Retire removes the pruned profiles, their readiness receipts, the t14 rejected stage and the previous journal.
func (Profiles) Retire() {
	// Never during a trial, and only while the live journal holds a kept
	// profile: router-state itself, the active profile and the rollback
	// cron are not touched.
	guard := Sh("test ! -e " + routerGonf + "/router-state/pending")
	active := Sh("grep -Eqx 'wifi-ipv6-nat66|camera-igc3|lab-igc1|lab-final' "+routerGonf+"/router-state/profile",
		DependsOn(guard))
	dirs := []string{routerGonf + "/router-rejected-t14-shared-expiry2-20261105", routerGonf + "/router-state.previous"}
	readiness := []string{}
	for _, name := range prunedProfiles {
		dirs = append(dirs, routerGonf+"/router-profiles/"+name)
		readiness = append(readiness, routerGonf+"/router-readiness/"+name)
	}
	NoDir(List(dirs...), WithPrune, DependsOn(active))
	NoFile(List(readiness...), DependsOn(active))
}
