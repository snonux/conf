package netbsd

import (
	. "github.com/snonux/gonf/api"
)

// ShellyEndpoints retires what the Shelly endpoint adoption (task 724) and
// the f3sctl HTTPS rollout (p34, f54) left on pi0/pi1 (task e74). The former
// prepare/activate tasks are gone: Controller.Config owns
// /usr/local/etc/f3sctl.json, with assets/shelly-endpoints.json as its
// recognised legacy input. RetireState is a one-shot task, deleted in a later
// commit once both Pis are clean.
//
// /var/db/gonf-endpoint-migration belongs to the endpoint helper that
// Controller.Config still uses and stays.
type ShellyEndpoints struct{ RequiresRoot }

// OptsRetireState keeps the one-shot removal out of the pis_netbsd aggregate.
func (ShellyEndpoints) OptsRetireState() TaskOptions { return TaskOptions{Operational()} }

// RetireState removes the Shelly endpoint candidate and the f3sctl rollback copies.
//
// The rollback copies (f3sctl 0.11.3 and 0.11.4rc1 binaries plus their
// configs) are obsolete: f3sctl 0.11.4 from pkgrepo is the installed release,
// and older packages remain on pkgrepo for a rollback.
func (ShellyEndpoints) RetireState() {
	NoFile("/usr/local/etc/f3sctl.json.triggerfish-shelly-candidate")
	NoDir(List(
		"/var/db/gonf-shelly-migration",
		"/var/db/gonf-p34-shelly",
		"/var/db/gonf-f54-f3sctl",
	), WithPrune)
}
