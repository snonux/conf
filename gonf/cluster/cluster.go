// Package fleet registers the SSH inventory (hosts and fleets) for the conf
// repository.
package fleet

import . "github.com/snonux/gonf/api"

// Fleet names — the source of truth for which hosts a task package targets.
// RegisterMethods(..., WithFleet(Name…)) so recipes use FleetHosts() /
// MustHostValue without naming the fleet again.
//
//	fleet            → Aggregate / deploy
//	NameFrontends    → frontends / frontends_*
//	NameNetBSDPis    → pis_netbsd / pis_netbsd_*
//	NameRockyAll     → rocky / rocky_*
const (
	NameFrontends = "frontends"
	NameNetBSDPis = "netbsd-pis"
	NameRockyPis  = "rocky-pis"
	NamePis       = "pis"
	NameRockyK3s  = "rocky-k3s"
	NameRockyAll  = "rocky-all"
)

// Per-host recipe value keys (WithValue). Every fleet member that a recipe
// reads must have the key set — MustHostValue Fatals otherwise.
const (
	ValueUnattendedCron       = "unattended.cron"        // frontend [3]string hours; netbsd [2]string hours
	ValueUnattendedOnCalendar = "unattended.on_calendar" // rocky OnCalendar= string
)

// Register registers the OpenBSD frontend hosts, the four Raspberry Pis, the
// Rocky k3s nodes (r0–r2), and the fleets that group them. Privileged tasks
// apply via doas (OpenBSD / NetBSD) or sudo (Rocky).
//
// Pi and r-node SSH must use port 22 explicitly: ~/.ssh/config maps Host
// *.buetow.org to Port 2 (OpenBSD frontends), so an unqualified ssh to
// piN.lan.buetow.org / rN.lan.buetow.org times out on port 2. See
// frontends/docs/unattended-upgrades-pi.plan.md §2.
//
// Inventory names (pi0…pi3, r0…r2) are substrings of the live OS hostnames
// (piN.lan.buetow.org / rN.lan.buetow.org) so WhenHostname("piN") /
// WhenHostname("rN") match via hostname_contains. Keep that invariant if
// renaming hosts.
//
// r-nodes are reached on the LAN as root@rN.lan.buetow.org (amd64 Rocky VMs),
// same Port-22 rule as the Pis. Login is already root; PrivilegeSudo still
// wraps elevated chunks as sudo -n so gonf push works with RequiresRoot tasks.
func Register() {
	blowfish := Host("blowfish",
		WithSSHUser("rex"),
		WithSSHHost("blowfish.buetow.org"),
		WithSSHPort(2),
		WithPrivilege(PrivilegeDoas),
		WithGOOS("openbsd"),
		WithGOARCH("amd64"),
		WithValue(ValueUnattendedCron, [3]string{"6", "6", "7"}),
	)
	fishfinger := Host("fishfinger",
		WithSSHUser("rex"),
		WithSSHHost("fishfinger.buetow.org"),
		WithSSHPort(2),
		WithPrivilege(PrivilegeDoas),
		WithGOOS("openbsd"),
		WithGOARCH("amd64"),
		WithValue(ValueUnattendedCron, [3]string{"22", "22", "23"}),
	)
	Fleet(NameFrontends, blowfish, fishfinger)

	pi0 := Host("pi0",
		WithSSHUser("paul"),
		WithSSHHost("pi0.lan.buetow.org"),
		WithSSHPort(22),
		WithPrivilege(PrivilegeDoas),
		WithGOOS("netbsd"),
		WithGOARCH("arm64"),
		WithValue(ValueUnattendedCron, [2]string{"2", "2"}),
	)
	pi1 := Host("pi1",
		WithSSHUser("paul"),
		WithSSHHost("pi1.lan.buetow.org"),
		WithSSHPort(22),
		WithPrivilege(PrivilegeDoas),
		WithGOOS("netbsd"),
		WithGOARCH("arm64"),
		WithValue(ValueUnattendedCron, [2]string{"22", "22"}),
	)
	pi2 := Host("pi2",
		WithSSHUser("paul"),
		WithSSHHost("pi2.lan.buetow.org"),
		WithSSHPort(22),
		WithPrivilege(PrivilegeSudo),
		WithGOOS("linux"),
		WithGOARCH("arm64"),
		WithValue(ValueUnattendedOnCalendar, "*-*-* *:05:00"),
	)
	pi3 := Host("pi3",
		WithSSHUser("paul"),
		WithSSHHost("pi3.lan.buetow.org"),
		WithSSHPort(22),
		WithPrivilege(PrivilegeSudo),
		WithGOOS("linux"),
		WithGOARCH("arm64"),
		WithValue(ValueUnattendedOnCalendar, "*-*-* *:35:00"),
	)
	// OS-pair fleets are the default deploy targets for OS-specific work
	// (scripts, timers, package managers). The umbrella "pis" fleet mixes
	// NetBSD/doas and Rocky/sudo — only use it for truly shared, OS-gated
	// tasks (e.g. a ping smoke test with WhenHostname / WhenLinux).
	Fleet(NameNetBSDPis, pi0, pi1)
	Fleet(NameRockyPis, pi2, pi3)
	Fleet(NamePis, pi0, pi1, pi2, pi3)

	r0 := Host("r0",
		WithSSHUser("root"),
		WithSSHHost("r0.lan.buetow.org"),
		WithSSHPort(22),
		WithPrivilege(PrivilegeSudo),
		WithGOOS("linux"),
		WithGOARCH("amd64"),
		WithValue(ValueUnattendedOnCalendar, "*-*-* *:05:00"),
	)
	r1 := Host("r1",
		WithSSHUser("root"),
		WithSSHHost("r1.lan.buetow.org"),
		WithSSHPort(22),
		WithPrivilege(PrivilegeSudo),
		WithGOOS("linux"),
		WithGOARCH("amd64"),
		WithValue(ValueUnattendedOnCalendar, "*-*-* *:25:00"),
	)
	r2 := Host("r2",
		WithSSHUser("root"),
		WithSSHHost("r2.lan.buetow.org"),
		WithSSHPort(22),
		WithPrivilege(PrivilegeSudo),
		WithGOOS("linux"),
		WithGOARCH("amd64"),
		WithValue(ValueUnattendedOnCalendar, "*-*-* *:45:00"),
	)
	// rocky-k3s = r0/r1/r2 only. rocky-all = every Rocky unattended host
	// (Pis + k3s). Prefer OS-pair / cluster fleets for deploys.
	Fleet(NameRockyK3s, r0, r1, r2)
	Fleet(NameRockyAll, pi2, pi3, r0, r1, r2)
}
