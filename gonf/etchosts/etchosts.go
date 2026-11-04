// Package etchosts is the one inventory of the homelab /etc/hosts rows the
// f3s hosts share: the LAN names (192.168.1.0/24) and the wg0 mesh names.
// The f-host recipe (freebsd.Base.Hosts) and the r-node recipe
// (rnodes.Base.Hosts) render the rows from here into their own OS-specific
// file skeleton, so a new host is one row here, not one edit per template.
//
// The wg0 rows are only name->address mappings. The wg0 configs, keys and
// peers belong to ~/git/wireguardmeshgenerator, which does not touch
// /etc/hosts.
package etchosts

import "github.com/snonux/conf/gonf/frontends"

// lanHost is one LAN row: "IP name name.lan name.lan.buetow.org".
type lanHost struct {
	name      string
	ip        string
	aliases   []string
	migration migrationHost
}

// Data is the root of the hosts templates (JSON-compatible for
// WithTemplateData): the rendered LAN and wg0 rows, in file order.
type Data struct {
	LAN       []string
	WireGuard []string
}

// lanHosts are the LAN rows in file order: hypervisors, the CARP storage
// VIP, the k3s VMs and the plain rocky VM, then the Pis (the old t450 laptop was retired on 2026-09-25).
var lanHosts = []lanHost{
	{name: "triggerfish", ip: "192.168.1.2", aliases: []string{"triggrefish.lan.buetow.org"}, migration: migrationHost{zone: routerZone, recovery: true, ula: "fd02:ca0b:f1cb:20::1"}},
	{name: "f0", ip: "192.168.1.130", migration: migrationHost{zone: labZone, recovery: true, ula: "fd02:ca0b:f1cb:20::130"}},
	{name: "f1", ip: "192.168.1.131", migration: migrationHost{zone: labZone, recovery: true, ula: "fd02:ca0b:f1cb:20::131"}},
	{name: "f2", ip: "192.168.1.132", migration: migrationHost{zone: labZone, recovery: true, ula: "fd02:ca0b:f1cb:20::132"}},
	{name: "f3", ip: "192.168.1.133", migration: migrationHost{zone: labZone, recovery: true, ula: "fd02:ca0b:f1cb:20::133"}},
	{name: "f3s-storage-ha", ip: "192.168.1.138", migration: migrationHost{zone: labZone}},
	{name: "r0", ip: "192.168.1.120", migration: migrationHost{zone: labZone, recovery: true, ula: "fd02:ca0b:f1cb:20::120"}},
	{name: "r1", ip: "192.168.1.121", migration: migrationHost{zone: labZone, recovery: true, ula: "fd02:ca0b:f1cb:20::121"}},
	{name: "r2", ip: "192.168.1.122", migration: migrationHost{zone: labZone, recovery: true, ula: "fd02:ca0b:f1cb:20::122"}},
	{name: "rocky", ip: "192.168.1.123", migration: migrationHost{zone: labZone, recovery: true}},
	{name: "pi0", ip: "192.168.1.125", migration: migrationHost{zone: labZone, recovery: true}},
	{name: "pi1", ip: "192.168.1.126", migration: migrationHost{zone: labZone, recovery: true}},
	{name: "pi2", ip: "192.168.1.127", migration: migrationHost{zone: labZone, recovery: true}},
	{name: "pi3", ip: "192.168.1.128", migration: migrationHost{zone: labZone, recovery: true}},
}

// TemplateData returns fresh LAN and wg0 rows for a hosts template.
func TemplateData() Data {
	return Data{LAN: LANLines(), WireGuard: WireGuardLines()}
}

// LANLines returns the LAN rows, "IP name name.lan name.lan.buetow.org".
func LANLines() []string {
	lines := make([]string, 0, len(lanHosts))
	for _, h := range lanHosts {
		line := h.ip + " " + h.name + " " + h.name + ".lan " + h.name + ".lan.buetow.org"
		for _, alias := range h.aliases {
			line += " " + alias
		}
		lines = append(lines, line)
	}
	return lines
}

// WireGuardLines returns the wg0 rows, all IPv4 then all IPv6, for the
// shared frontends peers plus the standalone ones (f3). f3 is a mesh member
// the f3s hosts resolve, but it stays out of frontends.WireGuardAddresses so
// the frontends' own /etc/hosts does not list it; its row lives in
// frontends.StandaloneWireGuardAddresses, which the frontends also ping.
func WireGuardLines() []string {
	return frontends.WireGuardHostLines(frontends.StandaloneWireGuardAddresses()...)
}
