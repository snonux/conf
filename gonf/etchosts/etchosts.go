// Package etchosts is the one inventory of the homelab LAN names the f3s
// hosts share, and the one renderer of what is published from it: the fleet
// /etc/hosts rows (f-, r- and rocky hosts, Pis, router, frontends, earth),
// the Pi-hole dnsmasq host records (pi2/pi3) and the authoritative NSD
// records of the frontends (generated into frontends/, see generate.go).
//
// Each LAN row renders its canonical lab name, a <host>.new alias, an
// explicit <host>.lan6 name for its native ULA and, while the endpoint still
// holds its former 192.168.1.x address, a <host>.old alias. The .old aliases
// of the hosts that left 192.168.1.x in i24 are gone (task 274); router, AP
// and the stopped freebsd VM keep theirs. The freebsd VM itself dropped
// 192.168.1.139 on 2026-11-11 (task 374); its freebsd.old row is stale but
// unused. It goes once that row is marked oldRetired and every hosts file,
// Pi-hole and NSD is pushed (not done yet). The .recovery aliases were
// retired in j24. The staging/target/ap-target previews of the router
// migration and their hosts-migration command were deleted in task f74
// (see conf history); the live fleet view is the only one left.
// check-offline.py checks that the generated frontends files are current
// and asserts the main facts of that view.
//
// The f-host recipe (freebsd.Base.Hosts) and the r-node recipe
// (rnodes.Base.Hosts) render the rows from here into their own OS-specific
// file skeleton, so a new host is one row here, not one edit per template.
//
// The wg0 rows are only name->address mappings. The wg0 configs, keys and
// peers belong to ~/git/wireguardmeshgenerator, which does not touch
// /etc/hosts.
package etchosts

import "github.com/snonux/conf/gonf/frontends"

// lanZone is the network a LAN row's endpoint lives in; it selects how the
// canonical address is derived from the row's former address.
type lanZone uint8

const (
	// noZone rows (zen) keep ip as their only, canonical address.
	noZone lanZone = iota
	// labZone endpoints moved to the same suffix in 10.20.0.0/24.
	labZone
	// wifiZone endpoints (the Shellys) moved to the same suffix in 10.30.0.0/24.
	wifiZone
	// routerZone is triggerfish: its canonical address depends on the
	// routerView, and it carries upstream/lab/wifi aliases.
	routerZone
	// apZone keeps its canonical name on ip; target is only its .new alias.
	apZone
)

// lanHost is one LAN row: "IP name name.lan name.lan.buetow.org" plus its
// aliases. ip is the host's former 192.168.1.x address (zen's current one):
// it is the suffix source of the lab and Wi-Fi addresses and the .old alias
// while that address is still held.
type lanHost struct {
	name    string
	ip      string
	aliases []string
	zone    lanZone
	// target overrides the derived address when suffix preservation is wrong
	// (AP, JetKVM leases).
	target string
	// ula is the native lab ULA, published only as <host>.lan6.
	ula string
	// oldRetired marks an endpoint that no longer holds ip (i24, 2026-11-11;
	// Shelly and JetKVM moved off the ISP LAN), so it gets no .old alias.
	oldRetired bool
}

// Data is the root of the hosts templates (JSON-compatible for
// WithTemplateData): the rendered LAN and wg0 rows, in file order.
type Data struct {
	LAN       []string
	WireGuard []string
}

// lanHosts are the LAN rows in file order: router, hypervisors, the CARP
// storage VIP, the k3s VMs and the plain rocky VM, then the Pis (the old t450
// laptop was retired on 2026-09-25). earthLAN and lanDevices follow them.
var lanHosts = []lanHost{
	{name: "triggerfish", ip: "192.168.1.2", aliases: []string{"triggrefish.lan.buetow.org"}, zone: routerZone, ula: "fd02:ca0b:f1cb:20::1"},
	{name: "f0", ip: "192.168.1.130", zone: labZone, ula: "fd02:ca0b:f1cb:20::130", oldRetired: true},
	{name: "f1", ip: "192.168.1.131", zone: labZone, ula: "fd02:ca0b:f1cb:20::131", oldRetired: true},
	{name: "f2", ip: "192.168.1.132", zone: labZone, ula: "fd02:ca0b:f1cb:20::132", oldRetired: true},
	{name: "f3", ip: "192.168.1.133", zone: labZone, ula: "fd02:ca0b:f1cb:20::133", oldRetired: true},
	{name: "f3s-storage-ha", ip: "192.168.1.138", zone: labZone, oldRetired: true},
	{name: "r0", ip: "192.168.1.120", zone: labZone, ula: "fd02:ca0b:f1cb:20::120", oldRetired: true},
	{name: "r1", ip: "192.168.1.121", zone: labZone, ula: "fd02:ca0b:f1cb:20::121", oldRetired: true},
	{name: "r2", ip: "192.168.1.122", zone: labZone, ula: "fd02:ca0b:f1cb:20::122", oldRetired: true},
	{name: "rocky", ip: "192.168.1.123", zone: labZone, oldRetired: true},
	{name: "pi0", ip: "192.168.1.125", zone: labZone, ula: "fd02:ca0b:f1cb:20::125", oldRetired: true},
	{name: "pi1", ip: "192.168.1.126", zone: labZone, ula: "fd02:ca0b:f1cb:20::126", oldRetired: true},
	{name: "pi2", ip: "192.168.1.127", zone: labZone, ula: "fd02:ca0b:f1cb:20::127", oldRetired: true},
	{name: "pi3", ip: "192.168.1.128", zone: labZone, ula: "fd02:ca0b:f1cb:20::128", oldRetired: true},
}

// TemplateData returns fresh LAN and wg0 rows for a hosts template.
func TemplateData() Data {
	return Data{LAN: LANLines(), WireGuard: WireGuardLines()}
}

// LANLines returns the fleet rows as lab clients see them (the router under
// its lab address 10.20.0.1).
func LANLines() []string {
	return hostRows(labView)
}

// WireGuardLines returns the wg0 rows, all IPv4 then all IPv6, for the
// shared frontends peers plus the standalone ones (f3). f3 is a mesh member
// the f3s hosts resolve, but it stays out of frontends.WireGuardAddresses so
// the frontends' own /etc/hosts does not list it; its row lives in
// frontends.StandaloneWireGuardAddresses, which the frontends also ping.
func WireGuardLines() []string {
	return frontends.WireGuardHostLines(frontends.StandaloneWireGuardAddresses()...)
}
