package etchosts

import (
	"github.com/snonux/conf/gonf/frontends"
)

// earthLAN are the non-rack rows (first listed for the laptop). hostRows
// renders them into every view, so every hosts file, NSD and Pi-hole get
// them. The Shellys left 192.168.1.x for Wi-Fi (oldRetired). The normally
// stopped freebsd VM retired its old address on 2026-11-11 (task 374, after
// i24), but freebsd.old is still rendered: setting oldRetired here needs a
// push to every hosts file, Pi-hole and NSD that carries it.
var earthLAN = []lanHost{
	{name: "shelly1", ip: "192.168.1.28", zone: wifiZone, oldRetired: true},
	{name: "shelly2", ip: "192.168.1.29", zone: wifiZone, oldRetired: true},
	{name: "zen", ip: "192.168.1.13"},
	{name: "freebsd", ip: "192.168.1.139", aliases: []string{"fbsd"}, zone: labZone},
}

var earthWireGuard = []frontends.WireGuardAddress{
	{Name: "freebsd", IPv4: "192.168.2.202", IPv6: "fd42:beef:cafe:2::202"},
	{Name: "uranus", IPv4: "192.168.2.205", IPv6: "fd42:beef:cafe:2::205"},
}

// EarthHostsBlock returns the fleet and unchanged mesh on the laptop.
// Its canonical router uses authenticated wg2 management; native ULA uses only
// explicit lan6 names. The upstream alias still names the WireGuard endpoint.
func EarthHostsBlock() []string { return fleetBlockWithMesh(vpnView) }

// RouterHostsBlock is the same block for triggerfish itself, whose canonical
// name stays its upstream address 192.168.1.2. It must not share the laptop's
// vpn view: since 1c2a805 (2026-11-08) the router task used EarthHostsBlock and
// would have renamed the router to its wg2 address 10.99.0.1 (seen in h24).
func RouterHostsBlock() []string { return fleetBlockWithMesh(upstreamView) }

// FrontendHostsLines is RouterHostsBlock without its comment lines: the
// fleet in the upstream view plus the mesh, as generated into
// frontends/etc/hosts-fleet.inc for frontends.Maintenance.Hosts.
func FrontendHostsLines() []string {
	return append(hostRows(upstreamView), meshLines()...)
}

// fleetBlockWithMesh renders the fleet in one router view plus the
// unchanged wg0 mesh names of the standalone peers and the laptop's extras.
// The comment lines are part of the published hosts block on earth and the
// router; changing them rewrites those files.
func fleetBlockWithMesh(view routerView) []string {
	lines := append([]string{"# Accepted LAN; retired OLD and recovery aliases dropped"}, hostRows(view)...)
	lines = append(lines, "# WireGuard mesh wg0 (wireguardmeshgenerator)")
	return append(lines, meshLines()...)
}

// meshLines are the wg0 rows of the shared frontends peers, the standalone
// peers (f3) and the laptop's extra peers.
func meshLines() []string {
	peers := append(frontends.StandaloneWireGuardAddresses(), earthWireGuard...)
	return frontends.WireGuardHostLines(peers...)
}
