// Package cluster registers the SSH inventory (hosts and clusters) for the conf
// repository.
package cluster

import (
	"codeberg.org/snonux/conf/gonf/frontends"
	. "github.com/snonux/gonf/api"
)

// Cluster names — the source of truth for which hosts a task package targets.
// RegisterMethods(..., WithCluster(Name…)) so recipes use ClusterHosts() /
// MustHostValue without naming the cluster again.
//
//	cluster          → Aggregate / deploy
//	NameFrontends    → frontends / frontends_*
//	NameNetBSDPis    → pis_netbsd / pis_netbsd_*
//	NameRockyAll     → rocky / rocky_*
//	NameFreeBSD      → freebsd / freebsd_*
const (
	NameFrontends = "frontends"
	NameNetBSDPis = "netbsd-pis"
	NameRockyPis  = "rocky-pis"
	NamePis       = "pis"
	NameRockyK3s  = "rocky-k3s"
	NameRockyAll  = "rocky-all"
	NameFreeBSD   = "freebsd-hosts"
	NameGarage    = "garage"
)

// Per-host recipe value keys (WithValue). Every cluster member that a recipe
// reads must have the key set — MustHostValue Fatals otherwise.
const (
	ValueUnattendedCron        = "unattended.cron"         // frontend [3]string hours; netbsd [2]string hours
	ValueUnattendedOnCalendar  = "unattended.on_calendar"  // rocky OnCalendar= string
	ValueUnattendedCronMinute  = "unattended.cron_minute"  // freebsd hourly minute string
	ValueUnattendedAllowReboot = "unattended.allow_reboot" // freebsd bool; false on f3
	ValueFrontendServer        = "frontends.server"        // frontends.Server address and role data
	ValueGarageRPCPublicAddr   = "garage.rpc_public_addr"  // Garage node RPC address, including port
)

// Register registers the OpenBSD frontend hosts, the four Raspberry Pis, the
// Rocky k3s nodes (r0–r2), the FreeBSD hypervisors (f0–f3), and the clusters
// that group them. Privileged tasks apply via doas (OpenBSD / NetBSD /
// FreeBSD) or sudo (Rocky).
//
// Pi, r-node, and f-host SSH must use port 22 explicitly: ~/.ssh/config maps
// Host *.buetow.org to Port 2 (OpenBSD frontends), so an unqualified ssh to
// piN.lan.buetow.org / rN.lan.buetow.org / fN.lan.buetow.org times out on
// port 2. See frontends/docs/unattended-upgrades-pi.plan.md §2.
//
// Inventory names (pi0…pi3, r0…r2, f0…f3) are substrings of the live OS
// hostnames so WhenHostname("piN") / WhenHostname("rN") / WhenHostname("fN")
// match via hostname_contains. Keep that invariant if renaming hosts.
//
// r-nodes are reached on the LAN as root@rN.lan.buetow.org (amd64 Rocky VMs),
// same Port-22 rule as the Pis. Login is already root; PrivilegeSudo still
// wraps elevated chunks as sudo -n so gonf push works with RequiresRoot tasks.
//
// f-hosts are paul@fN.lan.buetow.org with doas (never root SSH).
func Register() {
	blowfish := Host("blowfish",
		WithSSHUser("rex"),
		WithSSHHost("blowfish.buetow.org"),
		WithSSHPort(2),
		WithPrivilege(PrivilegeDoas),
		WithGOOS("openbsd"),
		WithGOARCH("amd64"),
		WithValue(ValueUnattendedCron, [3]string{"6", "6", "7"}),
		WithValue(ValueFrontendServer, frontends.MustServer("blowfish")),
	)
	fishfinger := Host("fishfinger",
		WithSSHUser("rex"),
		WithSSHHost("fishfinger.buetow.org"),
		WithSSHPort(2),
		WithPrivilege(PrivilegeDoas),
		WithGOOS("openbsd"),
		WithGOARCH("amd64"),
		WithValue(ValueUnattendedCron, [3]string{"22", "22", "23"}),
		WithValue(ValueFrontendServer, frontends.MustServer("fishfinger")),
	)
	Cluster(NameFrontends, blowfish, fishfinger).Parallel(5)

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
	// OS-pair clusters are the default deploy targets for OS-specific work
	// (scripts, timers, package managers). The umbrella "pis" cluster mixes
	// NetBSD/doas and Rocky/sudo — only use it for truly shared, OS-gated
	// tasks (e.g. a ping smoke test with WhenHostname / WhenLinux).
	Cluster(NameNetBSDPis, pi0, pi1)
	Cluster(NameRockyPis, pi2, pi3)
	Cluster(NamePis, pi0, pi1, pi2, pi3)

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
	// (Pis + k3s). Prefer OS-pair / role clusters for deploys.
	Cluster(NameRockyK3s, r0, r1, r2).Parallel(3)
	Cluster(NameRockyAll, pi2, pi3, r0, r1, r2)

	f0 := Host("f0",
		WithSSHUser("paul"),
		WithSSHHost("f0.lan.buetow.org"),
		WithSSHPort(22),
		WithPrivilege(PrivilegeDoas),
		WithGOOS("freebsd"),
		WithGOARCH("amd64"),
		WithValue(ValueUnattendedCronMinute, "5"),
		WithValue(ValueUnattendedAllowReboot, true),
		WithValue(ValueGarageRPCPublicAddr, "192.168.1.130:3901"),
	)
	f1 := Host("f1",
		WithSSHUser("paul"),
		WithSSHHost("f1.lan.buetow.org"),
		WithSSHPort(22),
		WithPrivilege(PrivilegeDoas),
		WithGOOS("freebsd"),
		WithGOARCH("amd64"),
		WithValue(ValueUnattendedCronMinute, "25"),
		WithValue(ValueUnattendedAllowReboot, true),
		WithValue(ValueGarageRPCPublicAddr, "192.168.1.131:3901"),
	)
	f2 := Host("f2",
		WithSSHUser("paul"),
		WithSSHHost("f2.lan.buetow.org"),
		WithSSHPort(22),
		WithPrivilege(PrivilegeDoas),
		WithGOOS("freebsd"),
		WithGOARCH("amd64"),
		WithValue(ValueUnattendedCronMinute, "45"),
		WithValue(ValueUnattendedAllowReboot, true),
		WithValue(ValueGarageRPCPublicAddr, "192.168.1.132:3901"),
	)
	f3 := Host("f3",
		WithSSHUser("paul"),
		WithSSHHost("f3.lan.buetow.org"),
		WithSSHPort(22),
		WithPrivilege(PrivilegeDoas),
		WithGOOS("freebsd"),
		WithGOARCH("amd64"),
		WithValue(ValueUnattendedCronMinute, "15"),
		WithValue(ValueUnattendedAllowReboot, false),
	)
	// freebsd-hosts = all FreeBSD hypervisors. f3 never auto-reboots
	// (allow_reboot=false); script also hardcodes that for defense in depth.
	Cluster(NameFreeBSD, f0, f1, f2, f3)
	Cluster(NameGarage, f0, f1, f2).Parallel(1)
}
