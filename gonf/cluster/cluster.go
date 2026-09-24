// Package cluster registers the SSH inventory (hosts and clusters) for the conf
// repository.
package cluster

import (
	"codeberg.org/snonux/conf/gonf/freebsd"
	"codeberg.org/snonux/conf/gonf/frontends"
	"codeberg.org/snonux/conf/gonf/garage"
	"codeberg.org/snonux/conf/gonf/netbsd"
	"codeberg.org/snonux/conf/gonf/openbsd"
	"codeberg.org/snonux/conf/gonf/rocky"
	. "github.com/snonux/gonf/api"
)

// Cluster names — the source of truth for which hosts a task package targets.
// RegisterMethods(..., OnCluster(Name…)) binds the tasks to the cluster, so
// recipes use EachHost (or ClusterHosts()) without naming it again, and
// guards every task to the cluster's hosts.
//
//	cluster          → Aggregate / deploy
//	NameFrontends    → frontends / frontends_*
//	NameNetBSDPis    → pis_netbsd / pis_netbsd_*
//	NameRockyAll     → rocky / rocky_*
//	NameRockyPis     → rocky_kernel_audit / rocky_kernel_audit_* (also in rocky)
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

// Per-host recipe data is attached with WithData, one small struct type per
// recipe (openbsd.UnattendedSchedule, frontends.Server, ...), defined in the
// recipe package that reads it with EachHost. Every member of a task's
// cluster must carry the type the task reads, otherwise the record fails
// before any SSH connection. EachHost checks every member, even when a push
// targets only one host.

// Register registers the OpenBSD frontend hosts, the four Raspberry Pis, the
// Rocky k3s nodes (r0–r2), the FreeBSD hypervisors (f0–f3), and the clusters
// that group them. Privileged tasks apply via doas (OpenBSD / NetBSD /
// FreeBSD) or sudo (Rocky).
//
// Inventory names (pi0…pi3, r0…r2, f0…f3) are substrings of the live OS
// hostnames so OnCluster and WhenHostname("piN") / WhenHostname("rN") /
// WhenHostname("fN") match via hostname_contains. Keep that invariant if
// renaming hosts.
//
// r-nodes are reached on the LAN as root@rN.lan.buetow.org (amd64 Rocky VMs).
// Login is already root; PrivilegeSudo still wraps elevated chunks as
// sudo -n so gonf push works with RequiresRoot tasks.
//
// f-hosts are paul@fN.lan.buetow.org with doas (never root SSH).
//
// Register itself only sequences the host/cluster groups below, in a fixed
// order, so the host and cluster registration order stays stable.
func Register() {
	registerFrontends()
	pi2, pi3 := registerPis()
	registerRockyK3s(pi2, pi3)
	registerFreeBSD()
}

// lan bundles the SSH settings every LAN host (piN, rN, fN) shares. The port
// must be 22 explicitly: ~/.ssh/config maps Host *.buetow.org to Port 2 (the
// OpenBSD frontends), so an ssh without -p to piN.lan.buetow.org /
// rN.lan.buetow.org / fN.lan.buetow.org would time out on port 2. See
// docs/archive/frontends/docs/unattended-upgrades-pi.plan.md §2.
func lan(user string) HostOption {
	return HostDefaults(WithSSHUser(user), WithSSHPort(22))
}

// registerFrontends registers the two OpenBSD frontend hosts and the
// frontends cluster. They are reached as rex on port 2.
func registerFrontends() {
	frontend := HostDefaults(
		WithSSHUser("rex"),
		WithSSHPort(2),
		WithPrivilege(PrivilegeDoas),
		WithGOOS("openbsd"),
		WithGOARCH("amd64"),
	)
	blowfish := Host("blowfish", frontend,
		WithSSHHost("blowfish.buetow.org"),
		WithData(openbsd.UnattendedSchedule{BaseHour: "6", PkgsHour: "6", AuditHour: "7"}),
		WithData(frontends.MustServer("blowfish")),
	)
	fishfinger := Host("fishfinger", frontend,
		WithSSHHost("fishfinger.buetow.org"),
		WithData(openbsd.UnattendedSchedule{BaseHour: "22", PkgsHour: "22", AuditHour: "23"}),
		WithData(frontends.MustServer("fishfinger")),
	)
	Cluster(NameFrontends, blowfish, fishfinger).Parallel(5)
}

// registerPis registers the four Raspberry Pi hosts and their OS-pair and
// umbrella clusters. It returns pi2 and pi3 so registerRockyK3s can fold
// them into rocky-all (pi0/pi1 stay local — nothing downstream needs them).
func registerPis() (pi2, pi3 HostRef) {
	pi0, pi1 := registerNetBSDPiHosts()
	pi2, pi3 = registerRockyPiHosts()
	// OS-pair clusters are the default deploy targets for OS-specific work
	// (scripts, timers, package managers). The umbrella "pis" cluster mixes
	// NetBSD/doas and Rocky/sudo — only use it for truly shared, OS-gated
	// tasks (e.g. a ping smoke test with WhenHostname / WhenLinux).
	Cluster(NameNetBSDPis, pi0, pi1)
	Cluster(NameRockyPis, pi2, pi3)
	Cluster(NamePis, pi0, pi1, pi2, pi3)
	return pi2, pi3
}

// registerNetBSDPiHosts registers pi0 and pi1, the NetBSD/doas Pis.
func registerNetBSDPiHosts() (pi0, pi1 HostRef) {
	netbsdPi := HostDefaults(lan("paul"),
		WithPrivilege(PrivilegeDoas),
		WithGOOS("netbsd"),
		WithGOARCH("arm64"),
	)
	pi0 = Host("pi0", netbsdPi,
		WithSSHHost("pi0.lan.buetow.org"),
		WithData(netbsd.UnattendedSchedule{PkgsHour: "2", RebootHour: "2"}),
		// After the 02:10 pkgs / 02:50 reboot window (each with up to
		// 20 min jitter), before /etc/daily at 04:15.
		WithData(netbsd.VulnAuditTime{Minute: "40", Hour: "3"}),
	)
	pi1 = Host("pi1", netbsdPi,
		WithSSHHost("pi1.lan.buetow.org"),
		WithData(netbsd.UnattendedSchedule{PkgsHour: "22", RebootHour: "22"}),
		// After the 22:10 pkgs / 22:50 reboot window.
		WithData(netbsd.VulnAuditTime{Minute: "40", Hour: "23"}),
	)
	return pi0, pi1
}

// registerRockyPiHosts registers pi2 and pi3, the Rocky/sudo Pis.
func registerRockyPiHosts() (pi2, pi3 HostRef) {
	rockyPi := HostDefaults(lan("paul"),
		WithPrivilege(PrivilegeSudo),
		WithGOOS("linux"),
		WithGOARCH("arm64"),
	)
	pi2 = Host("pi2", rockyPi,
		WithSSHHost("pi2.lan.buetow.org"),
		WithData(rocky.UnattendedCalendar{OnCalendar: "*-*-* *:05:00"}),
		WithData(rocky.KernelAuditCalendar{OnCalendar: "*-*-* 06:15:00"}),
	)
	pi3 = Host("pi3", rockyPi,
		WithSSHHost("pi3.lan.buetow.org"),
		WithData(rocky.UnattendedCalendar{OnCalendar: "*-*-* *:35:00"}),
		WithData(rocky.KernelAuditCalendar{OnCalendar: "*-*-* 06:45:00"}),
	)
	return pi2, pi3
}

// registerRockyK3s registers the Rocky k3s hosts (r0–r2) and the rocky-k3s /
// rocky-all clusters. pi2 and pi3 (from registerPis) fold into rocky-all,
// which is every Rocky unattended host (Pis + k3s).
func registerRockyK3s(pi2, pi3 HostRef) {
	rnode := HostDefaults(lan("root"),
		WithPrivilege(PrivilegeSudo),
		WithGOOS("linux"),
		WithGOARCH("amd64"),
	)
	r0 := Host("r0", rnode,
		WithSSHHost("r0.lan.buetow.org"),
		WithData(rocky.UnattendedCalendar{OnCalendar: "*-*-* *:05:00"}),
	)
	r1 := Host("r1", rnode,
		WithSSHHost("r1.lan.buetow.org"),
		WithData(rocky.UnattendedCalendar{OnCalendar: "*-*-* *:25:00"}),
	)
	r2 := Host("r2", rnode,
		WithSSHHost("r2.lan.buetow.org"),
		WithData(rocky.UnattendedCalendar{OnCalendar: "*-*-* *:45:00"}),
	)
	// rocky-k3s = r0/r1/r2 only. rocky-all = every Rocky unattended host
	// (Pis + k3s). Prefer OS-pair / role clusters for deploys.
	Cluster(NameRockyK3s, r0, r1, r2).Parallel(3)
	Cluster(NameRockyAll, pi2, pi3, r0, r1, r2)
}

// registerFreeBSD registers the FreeBSD hypervisor hosts (f0–f3) and their
// freebsd-hosts / garage clusters. f0–f2 are also Garage nodes (each carries
// the garage.Node the Garage recipe reads); f3 stays out of the Garage
// cluster. f3 never auto-reboots: unattended-upgrade-freebsd hardcodes that.
func registerFreeBSD() {
	fhost := HostDefaults(lan("paul"),
		WithPrivilege(PrivilegeDoas),
		WithGOOS("freebsd"),
		WithGOARCH("amd64"),
	)
	f0 := Host("f0", fhost,
		WithSSHHost("f0.lan.buetow.org"),
		WithData(freebsd.UnattendedSchedule{Minute: "5"}),
		WithData(garage.Node{RPCPublicAddr: "192.168.1.130:3901"}),
	)
	f1 := Host("f1", fhost,
		WithSSHHost("f1.lan.buetow.org"),
		WithData(freebsd.UnattendedSchedule{Minute: "25"}),
		WithData(garage.Node{RPCPublicAddr: "192.168.1.131:3901"}),
	)
	f2 := Host("f2", fhost,
		WithSSHHost("f2.lan.buetow.org"),
		WithData(freebsd.UnattendedSchedule{Minute: "45"}),
		WithData(garage.Node{RPCPublicAddr: "192.168.1.132:3901"}),
	)
	f3 := Host("f3", fhost,
		WithSSHHost("f3.lan.buetow.org"),
		WithData(freebsd.UnattendedSchedule{Minute: "15"}),
	)
	Cluster(NameFreeBSD, f0, f1, f2, f3)
	Cluster(NameGarage, f0, f1, f2).Parallel(1)
}
