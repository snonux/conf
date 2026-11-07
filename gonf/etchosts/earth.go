package etchosts

import (
	"github.com/snonux/conf/gonf/frontends"
)

var earthLAN = []lanHost{
	{name: "shelly1", ip: "192.168.1.28", migration: migrationHost{zone: wifiZone}},
	{name: "shelly2", ip: "192.168.1.29", migration: migrationHost{zone: wifiZone}},
	{name: "zen", ip: "192.168.1.13"},
	{name: "freebsd", ip: "192.168.1.139", aliases: []string{"fbsd"}, migration: migrationHost{zone: labZone, recovery: true}},
}

var earthWireGuard = []frontends.WireGuardAddress{
	{Name: "freebsd", IPv4: "192.168.2.202", IPv6: "fd42:beef:cafe:2::202"},
	{Name: "uranus", IPv4: "192.168.2.205", IPv6: "fd42:beef:cafe:2::205"},
}

// EarthHostsBlock returns the accepted fleet and unchanged mesh on the laptop.
// Its canonical router stays upstream; native ULA uses only explicit lan6 names.
func EarthHostsBlock() []string {
	lines := append([]string{"# Accepted LAN; OLD and reserved recovery aliases retained"}, acceptedView("upstream").HostsBlock...)
	lines = append(lines, "# WireGuard mesh wg0 (wireguardmeshgenerator)")
	peers := append(frontends.StandaloneWireGuardAddresses(), earthWireGuard...)
	return append(lines, frontends.WireGuardHostLines(peers...)...)
}
