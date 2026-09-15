// Package fleet registers the SSH inventory (hosts and fleets) for the conf
// repository.
package fleet

import . "github.com/snonux/gonf/api"

// Register registers the OpenBSD frontend hosts and the frontends fleet.
// Privileged tasks apply via doas on these hosts.
func Register() {
	blowfish := Host("blowfish",
		WithSSHUser("rex"),
		WithSSHHost("blowfish.buetow.org"),
		WithSSHPort(2),
		WithPrivilege(PrivilegeDoas),
	)
	fishfinger := Host("fishfinger",
		WithSSHUser("rex"),
		WithSSHHost("fishfinger.buetow.org"),
		WithSSHPort(2),
		WithPrivilege(PrivilegeDoas),
	)
	Fleet("frontends", blowfish, fishfinger)
}
