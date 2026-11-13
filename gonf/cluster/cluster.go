// Package cluster registers the SSH inventory (hosts and clusters) for the conf
// repository.
package cluster

import (
	"github.com/snonux/conf/gonf/dtail"
	"github.com/snonux/conf/gonf/freebsd"
	"github.com/snonux/conf/gonf/frontends"
	"github.com/snonux/conf/gonf/garage"
	"github.com/snonux/conf/gonf/goprecords"
	"github.com/snonux/conf/gonf/netbsd"
	"github.com/snonux/conf/gonf/openbsd"
	"github.com/snonux/conf/gonf/rnodes"
	"github.com/snonux/conf/gonf/rocky"
	"github.com/snonux/conf/gonf/triggerfish"
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
//	NameFreeBSDGuest → freebsd_guest_* (normally stopped; push by name; also in freebsd, guarded)
const (
	NameFrontends    = "frontends"
	NameNetBSDPis    = "netbsd-pis"
	NameRockyPis     = "rocky-pis"
	NamePis          = "pis"
	NameRockyK3s     = "rocky-k3s"
	NameRockyAll     = "rocky-all"
	NameRockyDtail   = "rocky-dtail"
	NameFreeBSDGuest = "freebsd-guest"
	NameFreeBSD      = "freebsd-hosts"
	NameGarage       = "garage"
	NameTriggerfish  = "triggerfish"
)

// Per-host recipe data is attached with WithData, one small struct type per
// recipe (openbsd.UnattendedSchedule, frontends.Server, ...), defined in the
// recipe package that reads it with EachHost. Every member of a task's
// cluster must carry the type the task reads, otherwise the record fails
// before any SSH connection. EachHost checks every member, even when a push
// targets only one host.

// Register registers the OpenBSD frontend hosts, the four Raspberry Pis, the
// Rocky k3s nodes (r0–r2), standalone rocky, the FreeBSD hypervisors (f0–f3), and the clusters
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
	RegisterWithRouterTrial(triggerfish.RouterTrial{})
}

// RegisterWithRouterTrial registers the inventory with explicit nonsecret router
// trial input. Ordinary registration keeps the fail-closed zero default.
func RegisterWithRouterTrial(trial triggerfish.RouterTrial) {
	registerFrontends()
	router := Host("triggerfish", lan("paul"), WithPrivilege(PrivilegeDoas), WithPlatform("openbsd/amd64"),
		// j24 retired the provisioning alias 192.168.11.2 (no clients on igc0
		// after b24), the f0 -> AP TCP22 migration flow (the AP is managed from
		// Wi-Fi, where it sits) and the shared-wire JetKVM canary, which needed
		// that alias. Every profile renders without them.
		WithData(triggerfish.RouterProfiles{BootCanary: trial.BootCanary}),
		WithData(trial))
	Cluster(NameTriggerfish, router)
	pi2, pi3 := registerPis()
	registerRockyK3s(pi2, pi3)
	registerFreeBSD()
	// Guest stays out of hypervisor aggregates and vm_list (normally stopped).
	// Use its trusted SSH alias; plain-IP inventory must not bypass host keys.
	guest := Host("freebsd", lan("paul"), WithPrivilege(PrivilegeDoas), WithPlatform("freebsd/amd64"),
		WithSSHHost("freebsd.lan"))
	Cluster(NameFreeBSDGuest, guest)
}

// lan bundles the SSH settings every LAN host (piN, rN, fN) shares. The port
// must be 22 explicitly: ~/.ssh/config maps Host *.buetow.org to Port 2 (the
// OpenBSD frontends), so an ssh without -p to piN.lan.buetow.org /
// rN.lan.buetow.org / fN.lan.buetow.org would time out on port 2. See
// docs/archive/frontends/docs/unattended-upgrades-pi.plan.md §2.
func lan(user string) HostOption {
	return HostDefaults(WithSSHUser(user), WithSSHDomain("lan.buetow.org"), WithSSHPort(22))
}

// registerFrontends registers the two OpenBSD frontend hosts and the
// frontends cluster. They are reached as rex on port 2.
func registerFrontends() {
	frontend := HostDefaults(
		WithSSHUser("rex"),
		WithSSHDomain("buetow.org"),
		WithSSHPort(2),
		WithPrivilege(PrivilegeDoas),
		WithPlatform("openbsd/amd64"),
	)
	blowfish := Host("blowfish", frontend,
		WithData(dtail.ServerHost{BindAddress: "192.168.2.110", Platform: "openbsd"}),
		WithData(openbsd.UnattendedSchedule{BaseHour: "6", PkgsHour: "6", AuditHour: "7"}),
		WithData(frontends.MustServer("blowfish")),
	)
	fishfinger := Host("fishfinger", frontend,
		WithData(dtail.ServerHost{BindAddress: "192.168.2.111", Platform: "openbsd"}),
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
		WithPlatform("netbsd/arm64"),
	)
	pi0 = Host("pi0", netbsdPi,
		WithData(netbsd.LANAddress{Interface: "mue0", IPv4: "10.20.0.125", ULA: "fd02:ca0b:f1cb:20::125"}),
		WithData(dtail.ServerHost{BindAddress: "10.20.0.125", Platform: "netbsd"}),
		// Hours are local time (Europe/Sofia since 2026-09-25). The pkgs
		// window must fall while the k3s cluster is up: the custom repo
		// pkgrepo.f3s.buetow.org runs on it, and at the old 02:xx window the
		// cluster is powered off every night, so dtail/f3sctl etc. were never
		// updated ("not operational — skipping custom-repo packages"). pi1's
		// 22:xx window is before the nightly power-off. pi0 and pi1 keep
		// separate windows so the static site always has one live backend.
		WithData(netbsd.UnattendedSchedule{PkgsHour: "14", RebootHour: "14"}),
		// After the 14:10 pkgs / 14:50 reboot window (each with up to
		// 20 min jitter).
		WithData(netbsd.VulnAuditTime{Minute: "40", Hour: "15"}),
		WithData(goprecords.ClientHost{Host: "pi0"}),
		// Hourly Shelly certificate probe; pi1 runs half an hour apart.
		WithData(netbsd.ShellyTLSProbeTime{Minute: "23"}),
	)
	pi1 = Host("pi1", netbsdPi,
		WithData(netbsd.LANAddress{Interface: "mue0", IPv4: "10.20.0.126", ULA: "fd02:ca0b:f1cb:20::126"}),
		WithData(dtail.ServerHost{BindAddress: "10.20.0.126", Platform: "netbsd"}),
		WithData(netbsd.UnattendedSchedule{PkgsHour: "22", RebootHour: "22"}),
		// After the 22:10 pkgs / 22:50 reboot window.
		WithData(netbsd.VulnAuditTime{Minute: "40", Hour: "23"}),
		WithData(goprecords.ClientHost{Host: "pi1"}),
		WithData(netbsd.ShellyTLSProbeTime{Minute: "53"}),
	)
	return pi0, pi1
}

// registerRockyPiHosts registers pi2 and pi3, the Rocky/sudo Pis.
func registerRockyPiHosts() (pi2, pi3 HostRef) {
	rockyPi := HostDefaults(lan("paul"),
		WithPrivilege(PrivilegeSudo),
		WithPlatform("linux/arm64"),
	)
	pi2 = Host("pi2", rockyPi,
		WithData(dtail.ServerHost{BindAddress: "10.20.0.127", Platform: "linux"}),
		WithData(rocky.UnattendedCalendar{OnCalendar: "*-*-* *:05:00"}),
		WithData(rocky.KernelAuditCalendar{OnCalendar: "*-*-* 06:15:00"}),
		WithData(goprecords.ClientHost{Host: "pi2"}),
	)
	pi3 = Host("pi3", rockyPi,
		WithData(dtail.ServerHost{BindAddress: "10.20.0.128", Platform: "linux"}),
		WithData(rocky.UnattendedCalendar{OnCalendar: "*-*-* *:35:00"}),
		WithData(rocky.KernelAuditCalendar{OnCalendar: "*-*-* 06:45:00"}),
		WithData(goprecords.ClientHost{Host: "pi3"}),
	)
	return pi2, pi3
}

// registerRockyK3s registers the Rocky k3s hosts (r0–r2) and the rocky-k3s /
// rocky-all clusters. Each r-node carries its wg0 address as rnodes.K3sNode
// (k3s node-ip and advertise-address, rnodes_k3s_config). pi2 and pi3 (from
// registerPis) fold into rocky-all, which is every Rocky unattended host
// (Pis + k3s + the standalone VM). Dtail remains limited to its existing
// Pis/k3s role; adding unattended coverage must not add a server to rocky.
// r0-r2 and the standalone VM all carry rocky.BhyveGuest: all four are
// vm-bhyve guests ("rocky" on f0-f3), and a vm stop during the guest's boot
// must still power it off (rocky.PowerButton, task j74; r0-r2 since n74).
func registerRockyK3s(pi2, pi3 HostRef) {
	rnode := HostDefaults(lan("root"),
		WithPrivilege(PrivilegeSudo),
		WithPlatform("linux/amd64"),
	)
	r0 := Host("r0", rnode,
		WithData(dtail.ServerHost{BindAddress: "10.20.0.120", Platform: "linux"}),
		WithData(rnodes.K3sNode{IP: "192.168.2.120"}),
		WithData(rocky.UnattendedCalendar{OnCalendar: "*-*-* *:05:00"}),
		WithData(rocky.BhyveGuest{}),
	)
	r1 := Host("r1", rnode,
		WithData(dtail.ServerHost{BindAddress: "10.20.0.121", Platform: "linux"}),
		WithData(rnodes.K3sNode{IP: "192.168.2.121"}),
		WithData(rocky.UnattendedCalendar{OnCalendar: "*-*-* *:25:00"}),
		WithData(rocky.BhyveGuest{}),
	)
	r2 := Host("r2", rnode,
		WithData(dtail.ServerHost{BindAddress: "10.20.0.122", Platform: "linux"}),
		WithData(rnodes.K3sNode{IP: "192.168.2.122"}),
		WithData(rocky.UnattendedCalendar{OnCalendar: "*-*-* *:45:00"}),
		WithData(rocky.BhyveGuest{}),
	)
	// Root access is documented for the standalone VM; validate sudo and
	// bootstrap gonf when it becomes reachable before approving a deploy.
	standalone := Host("rocky", rnode,
		WithData(rocky.UnattendedCalendar{OnCalendar: "*-*-* *:55:00"}),
		WithData(rocky.BhyveGuest{}),
	)
	Cluster(NameRockyK3s, r0, r1, r2).Parallel(3)
	Cluster(NameRockyDtail, pi2, pi3, r0, r1, r2)
	Cluster(NameRockyAll, pi2, pi3, r0, r1, r2, standalone)
}

// registerFreeBSD registers the FreeBSD hypervisor hosts (f0–f3) and their
// freebsd-hosts / garage clusters. f0–f2 are also Garage nodes (each carries
// the garage.Node the Garage recipe reads); f3 stays out of the Garage
// cluster. f3 never auto-reboots: unattended-upgrade-freebsd hardcodes that.
// The zrepl jobs and ZFS key datasets (fNZrepl, fNKeys) live in
// freebsd_storage.go.
func registerFreeBSD() {
	// f0 has the UPS on USB and serves its status on its LAN address; f1-f3
	// poll it there (by IP, so apcupsd never depends on name resolution).
	const upsServer = "10.20.0.130:3551"
	// Baseline keys that differ between the f-hosts, as found live on
	// 2026-09-25 (task dk2). f3 is not a k3s host and lacks the 300 s
	// rcshutdown_timeout, clear_tmp_enable and cryptodev_load; left as is.
	f01Baseline := freebsd.BaselineHost{ClearTmp: true, ShutdownTimeout: "300", Cryptodev: true}
	f2Baseline := freebsd.BaselineHost{ShutdownTimeout: "300", Cryptodev: true}
	f3Baseline := freebsd.BaselineHost{}
	fhost := HostDefaults(lan("paul"),
		WithPrivilege(PrivilegeDoas),
		WithPlatform("freebsd/amd64"),
	)
	f0 := Host("f0", fhost,
		WithData(dtail.ServerHost{BindAddress: "10.20.0.130", Platform: "freebsd"}),
		WithData(freebsd.UnattendedSchedule{Minute: "5"}),
		WithData(freebsd.PeriodicSchedule{Hour: "13"}),
		WithData(f0Zrepl), WithData(f0Keys),
		WithData(freebsd.NodeExporterHost{ListenAddress: "192.168.2.130:9100"}),
		WithData(freebsd.GoprecordsClient{Host: "f0"}),
		WithData(f01Baseline),
		// CARP MASTER by default (advskew 0). Interface: the VIP's rc.conf
		// line follows the host's addresses: freebsd.LabBridge since the
		// task 764 bridge move was confirmed on f0 and f1 (2026-11-12).
		//
		// GateDeadline 60 (task s74; f1 keeps the 180 s default): after a
		// cold start of both members f0 gives up first and takes the VIP
		// (~70 s after its gate starts instead of ~195 s), and f1, still
		// held, hears f0 and comes up BACKUP. Measured in
		// /var/log/gonf-carp-gate.log: guest tap joined 9-11 s after the
		// waiter started, and with the peer MASTER the advert was heard
		// 18-21 s after it (f0 2026-11-12 13:01Z with f1 MASTER: 21 s;
		// f1 12:46Z/12:53Z: 19 s/18 s). 60 s leaves ~3x that, i.e. four
		// 10 s-stable + 4 s-listen rounds where the first one sufficed.
		// Residual risk: if f0 reboots alone while f1 is MASTER and re0
		// still does not forward at 60 s, f0 goes MASTER too; once frames
		// pass, the member with the higher effective advskew (normally f1:
		// 100 vs 0, unless a demotion raised f0's) steps down on the other's
		// first advert and, as both send from the virtual MAC on vm-public,
		// the switch re-learns the port within 1 s. A brief dual master, no
		// lasting split brain.
		WithData(freebsd.CarpNode{Interface: freebsd.LabBridge, GateDeadline: 60}),
		WithData(k3sBhyve),
		WithData(freebsd.UPSServer{NISIP: "10.20.0.130"}),
		// In-tree re(4) until f1 has run a week on the vendor driver without
		// a re0 stall (task v74). The _2 build has no TX watchdog, so
		// FreebsdNicWatchdogReset cannot see one: judge by f1 staying
		// reachable and CARP BACKUP. f0's reboot is a controlled CARP
		// failover. See f3's RealtekNIC.
		WithData(freebsd.RealtekNIC{}),
		WithData(garage.Node{RPCPublicAddr: "10.20.0.130:3901", AdminAPIBindAddr: "192.168.2.130:3903"}),
	)
	f1 := Host("f1", fhost,
		WithData(dtail.ServerHost{BindAddress: "10.20.0.131", Platform: "freebsd"}),
		WithData(freebsd.UnattendedSchedule{Minute: "25"}),
		WithData(freebsd.PeriodicSchedule{Hour: "14"}),
		WithData(f1Zrepl), WithData(f1Keys),
		WithData(freebsd.NodeExporterHost{ListenAddress: "192.168.2.131:9100"}),
		WithData(freebsd.GoprecordsClient{Host: "f1"}),
		WithData(f01Baseline),
		// CARP standby, loses to f0; VIP on vm-public since 764 (2026-11-12).
		// No GateDeadline: f1 keeps the 180 s default, so on a cold start
		// it hears f0 (60 s) and never promotes while f0 is coming up.
		WithData(freebsd.CarpNode{AdvSkew: 100, Interface: freebsd.LabBridge}),
		WithData(k3sBhyve),
		// Back on the in-tree re(4) since 2026-11-13 (x74). See f3's
		// RealtekNIC.
		WithData(freebsd.RealtekNIC{}),
		WithData(freebsd.UPSClient{Server: upsServer, NISIP: "127.0.0.1"}),
		WithData(garage.Node{RPCPublicAddr: "10.20.0.131:3901", AdminAPIBindAddr: "192.168.2.131:3903"}),
	)
	f2 := Host("f2", fhost,
		WithData(dtail.ServerHost{BindAddress: "10.20.0.132", Platform: "freebsd"}),
		WithData(freebsd.UnattendedSchedule{Minute: "45"}),
		WithData(freebsd.PeriodicSchedule{Hour: "15"}),
		WithData(f2Zrepl), WithData(f2Keys),
		WithData(freebsd.NodeExporterHost{ListenAddress: "192.168.2.132:9100"}),
		WithData(freebsd.GoprecordsClient{Host: "f2"}),
		WithData(f2Baseline),
		WithData(k3sBhyve),
		// Realtek vendor driver since 2026-11-13 (t74, after f3 and f1).
		// See f3's RealtekNIC.
		WithData(freebsd.RealtekNIC{VendorDriver: true}),
		WithData(freebsd.UPSClient{Server: upsServer, NISIP: "127.0.0.1"}),
		WithData(garage.Node{RPCPublicAddr: "10.20.0.132:3901", AdminAPIBindAddr: "192.168.2.132:3903"}),
	)
	f3 := Host("f3", fhost,
		WithData(dtail.ServerHost{BindAddress: "10.20.0.133", Platform: "freebsd"}),
		WithData(freebsd.UnattendedSchedule{Minute: "15"}),
		WithData(freebsd.PeriodicSchedule{Hour: "16"}),
		WithData(f3Zrepl), WithData(f3Keys),
		WithData(freebsd.NodeExporterHost{ListenAddress: "192.168.2.133:9100"}),
		WithData(freebsd.GoprecordsClient{Host: "f3"}),
		WithData(f3Baseline),
		WithData(f3Bhyve),
		// Back on the in-tree re(4) since 2026-11-13 (task x74): the user
		// rejected the Realtek vendor driver (realtek-re-kmod, t74) because
		// it has to be rebuilt and checked at every FreeBSD update. f1 and
		// f2 follow one at a time; then the vendor-driver code goes.
		WithData(freebsd.RealtekNIC{}),
		WithData(freebsd.UPSClient{Server: upsServer, NISIP: "127.0.0.1"}),
	)
	Cluster(NameFreeBSD, f0, f1, f2, f3)
	Cluster(NameGarage, f0, f1, f2).Parallel(1)
}
