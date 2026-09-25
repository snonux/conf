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
	name string
	ip   string
}

// Data is the root of the hosts templates (JSON-compatible for
// WithTemplateData): the rendered LAN and wg0 rows, in file order.
type Data struct {
	LAN       []string
	WireGuard []string
}

// lanHosts are the LAN rows in file order: hypervisors, the CARP storage
// VIP, the k3s VMs and the plain rocky VM, the Pis, then the t450 laptop.
var lanHosts = []lanHost{
	{name: "f0", ip: "192.168.1.130"},
	{name: "f1", ip: "192.168.1.131"},
	{name: "f2", ip: "192.168.1.132"},
	{name: "f3", ip: "192.168.1.133"},
	{name: "f3s-storage-ha", ip: "192.168.1.138"},
	{name: "r0", ip: "192.168.1.120"},
	{name: "r1", ip: "192.168.1.121"},
	{name: "r2", ip: "192.168.1.122"},
	{name: "rocky", ip: "192.168.1.123"},
	{name: "pi0", ip: "192.168.1.125"},
	{name: "pi1", ip: "192.168.1.126"},
	{name: "pi2", ip: "192.168.1.127"},
	{name: "pi3", ip: "192.168.1.128"},
	{name: "t450", ip: "192.168.1.27"},
}

// wireGuardExtras are wg0 peers the f3s hosts resolve beyond the shared
// frontends inventory. f3 is a mesh member (wireguardmeshgenerator.yaml) but
// is kept out of frontends.WireGuardAddresses on purpose: that list also
// drives the frontends' Gogios ping checks.
var wireGuardExtras = []frontends.WireGuardAddress{
	{Name: "f3", IPv4: "192.168.2.133", IPv6: "fd42:beef:cafe:2::133"},
}

// TemplateData returns fresh LAN and wg0 rows for a hosts template.
func TemplateData() Data {
	return Data{LAN: LANLines(), WireGuard: WireGuardLines()}
}

// LANLines returns the LAN rows, "IP name name.lan name.lan.buetow.org".
func LANLines() []string {
	lines := make([]string, 0, len(lanHosts))
	for _, h := range lanHosts {
		lines = append(lines, h.ip+" "+h.name+" "+h.name+".lan "+h.name+".lan.buetow.org")
	}
	return lines
}

// WireGuardLines returns the wg0 rows, all IPv4 then all IPv6, for the
// shared frontends peers plus wireGuardExtras.
func WireGuardLines() []string {
	return frontends.WireGuardHostLines(wireGuardExtras...)
}
