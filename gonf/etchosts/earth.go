package etchosts

import (
	"github.com/snonux/conf/gonf/frontends"
)

var earthLAN = []lanHost{
	{name: "shelly1", ip: "192.168.1.28"},
	{name: "shelly2", ip: "192.168.1.29"},
	{name: "zen", ip: "192.168.1.13"},
	{name: "freebsd", ip: "192.168.1.139", aliases: []string{"fbsd"}},
}

var earthWireGuard = []frontends.WireGuardAddress{
	{Name: "freebsd", IPv4: "192.168.2.202", IPv6: "fd42:beef:cafe:2::202"},
	{Name: "uranus", IPv4: "192.168.2.205", IPv6: "fd42:beef:cafe:2::205"},
}

// EarthHostsBlock returns the laptop's fleet block, sharing server inventory
// while keeping its additional laptop-only names out of the server templates.
func EarthHostsBlock() []string {
	lines := append([]string{"# LAN (192.168.1.0/24)"}, LANLines()...)
	for _, h := range earthLAN {
		line := h.ip + " " + h.name + " " + h.name + ".lan " + h.name + ".lan.buetow.org"
		for _, alias := range h.aliases {
			line += " " + alias
		}
		lines = append(lines, line)
	}
	lines = append(lines, "# WireGuard mesh wg0 (wireguardmeshgenerator)")
	peers := append(frontends.StandaloneWireGuardAddresses(), earthWireGuard...)
	return append(lines, frontends.WireGuardHostLines(peers...)...)
}
