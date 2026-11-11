package etchosts

import (
	"github.com/snonux/conf/gonf/frontends"
)

// earthLAN are the non-rack rows (first listed for the laptop). RenderMigration
// renders them into every profile, so every hosts file, NSD and Pi-hole get
// them. The Shellys left 192.168.1.x for Wi-Fi (oldRetired); the normally
// stopped freebsd VM still has its old address configured (not part of i24),
// so it keeps freebsd.old.
var earthLAN = []lanHost{
	{name: "shelly1", ip: "192.168.1.28", migration: migrationHost{zone: wifiZone, oldRetired: true}},
	{name: "shelly2", ip: "192.168.1.29", migration: migrationHost{zone: wifiZone, oldRetired: true}},
	{name: "zen", ip: "192.168.1.13"},
	{name: "freebsd", ip: "192.168.1.139", aliases: []string{"fbsd"}, migration: migrationHost{zone: labZone, recovery: true}},
}

var earthWireGuard = []frontends.WireGuardAddress{
	{Name: "freebsd", IPv4: "192.168.2.202", IPv6: "fd42:beef:cafe:2::202"},
	{Name: "uranus", IPv4: "192.168.2.205", IPv6: "fd42:beef:cafe:2::205"},
}

// EarthHostsBlock returns the accepted fleet and unchanged mesh on the laptop.
// Its canonical router uses authenticated wg2 management; native ULA uses only
// explicit lan6 names. The upstream alias still names the WireGuard endpoint.
func EarthHostsBlock() []string { return fleetBlockWithMesh("vpn") }

// RouterHostsBlock is the same block for triggerfish itself, whose canonical
// name stays its upstream address 192.168.1.2. It must not share the laptop's
// vpn view: since 1c2a805 (2026-11-08) the router task used EarthHostsBlock and
// would have renamed the router to its wg2 address 10.99.0.1 (seen in h24).
func RouterHostsBlock() []string { return fleetBlockWithMesh("upstream") }

// fleetBlockWithMesh renders the accepted fleet in one router view plus the
// unchanged wg0 mesh names of the standalone peers and the laptop's extras.
func fleetBlockWithMesh(routerView string) []string {
	lines := append([]string{"# Accepted LAN; reserved recovery aliases retained, retired OLD aliases dropped"}, acceptedView(routerView).HostsBlock...)
	lines = append(lines, "# WireGuard mesh wg0 (wireguardmeshgenerator)")
	peers := append(frontends.StandaloneWireGuardAddresses(), earthWireGuard...)
	return append(lines, frontends.WireGuardHostLines(peers...)...)
}
