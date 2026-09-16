// Package fleet registers the SSH inventory (hosts and fleets) for the conf
// repository.
package fleet

import . "github.com/snonux/gonf/api"

// Register registers the OpenBSD frontend hosts, the four Raspberry Pis, the
// Rocky k3s nodes (r0–r2), and the fleets that group them. Privileged tasks
// apply via doas (OpenBSD / NetBSD) or sudo (Rocky).
//
// Pi SSH must use port 22 explicitly: ~/.ssh/config maps Host *.buetow.org to
// Port 2 (OpenBSD frontends), so an unqualified ssh to piN.lan.buetow.org
// times out on port 2. See frontends/docs/unattended-upgrades-pi.plan.md §2.
//
// Inventory names (pi0…pi3, r0…r2) are substrings of the live OS hostnames
// (piN.lan.buetow.org / rN.lan.buetow.org) so WhenHostname("piN") /
// WhenHostname("rN") match via hostname_contains. Keep that invariant if
// renaming hosts.
//
// r-nodes are reached over WireGuard as root@rN.wg0 (amd64 Rocky VMs). Login
// is already root; PrivilegeSudo still wraps elevated chunks as sudo -n so
// gonf push works with RequiresRoot tasks.
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

	pi0 := Host("pi0",
		WithSSHUser("paul"),
		WithSSHHost("pi0.lan.buetow.org"),
		WithSSHPort(22),
		WithPrivilege(PrivilegeDoas),
	)
	pi1 := Host("pi1",
		WithSSHUser("paul"),
		WithSSHHost("pi1.lan.buetow.org"),
		WithSSHPort(22),
		WithPrivilege(PrivilegeDoas),
	)
	pi2 := Host("pi2",
		WithSSHUser("paul"),
		WithSSHHost("pi2.lan.buetow.org"),
		WithSSHPort(22),
		WithPrivilege(PrivilegeSudo),
	)
	pi3 := Host("pi3",
		WithSSHUser("paul"),
		WithSSHHost("pi3.lan.buetow.org"),
		WithSSHPort(22),
		WithPrivilege(PrivilegeSudo),
	)
	// OS-pair fleets are the default deploy targets for OS-specific work
	// (scripts, timers, package managers). The umbrella "pis" fleet mixes
	// NetBSD/doas and Rocky/sudo — only use it for truly shared, OS-gated
	// tasks (e.g. a ping smoke test with WhenHostname / WhenLinux).
	Fleet("netbsd-pis", pi0, pi1)
	Fleet("rocky-pis", pi2, pi3)
	Fleet("pis", pi0, pi1, pi2, pi3)

	r0 := Host("r0",
		WithSSHUser("root"),
		WithSSHHost("r0.wg0"),
		WithSSHPort(22),
		WithPrivilege(PrivilegeSudo),
	)
	r1 := Host("r1",
		WithSSHUser("root"),
		WithSSHHost("r1.wg0"),
		WithSSHPort(22),
		WithPrivilege(PrivilegeSudo),
	)
	r2 := Host("r2",
		WithSSHUser("root"),
		WithSSHHost("r2.wg0"),
		WithSSHPort(22),
		WithPrivilege(PrivilegeSudo),
	)
	// rocky-k3s = r0/r1/r2 only. rocky-all = every Rocky unattended host
	// (Pis + k3s). Prefer OS-pair / cluster fleets for deploys.
	Fleet("rocky-k3s", r0, r1, r2)
	Fleet("rocky-all", pi2, pi3, r0, r1, r2)
}
