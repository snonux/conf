package etchosts

import (
	"fmt"
	"net/netip"
	"strings"
)

// routerView selects the canonical address of the router names
// (triggerfish, triggerfish.lan, triggerfish.lan.buetow.org): the router's
// zone aliases (triggerfish.upstream/.lab/.wifi.lan) always name their
// explicit addresses. Only the views a published file uses exist.
type routerView uint8

const (
	// upstreamView (192.168.1.2) is for the router itself, the frontends and
	// the authoritative NSD records.
	upstreamView routerView = iota + 1
	// labView (10.20.0.1) is for the lab hosts and the Pi-hole records.
	labView
	// vpnView (10.99.0.1, wg2) is for the laptop's management access.
	vpnView
)

// lanDevices are the devices with one identity record here and no hosts
// recipe of their own. The JetKVM targets are their reserved leases on
// triggerfish (h24, 2026-11-10: each rebooted and answered with its device
// ID there); 274 (2026-11-11) dropped their .old aliases, since a former ISP
// lease may now belong to anyone. Device-ID names deliberately live outside
// the service wildcard namespace. The OpenWrt AP's only management address
// is 10.30.0.2 on the Wi-Fi segment since i74 (2026-11-13) removed
// 192.168.1.101 from its br-lan, so its canonical name follows target and
// openwrt.old is gone.
var lanDevices = []lanHost{
	{name: "openwrt", ip: "192.168.1.101", zone: wifiZone, target: "10.30.0.2", oldRetired: true},
	{name: "jetkvm-46793b0d4c8d45d", ip: "192.168.1.248", zone: labZone, target: "10.20.0.151", oldRetired: true},
	{name: "jetkvm-e10a0883debf7d4a", ip: "192.168.1.241", zone: labZone, target: "10.20.0.158", oldRetired: true},
	{name: "jetkvm-f72978d7f87aa969", ip: "192.168.1.244", zone: labZone, target: "10.20.0.191", oldRetired: true},
	{name: "jetkvm-cee6ab4df2f88754", ip: "192.168.1.245", zone: labZone, target: "10.20.0.198", oldRetired: true},
}

// nativeULA is the lab's native IPv6 prefix; every lan6 name must be in it.
var nativeULA = netip.MustParsePrefix("fd02:ca0b:f1cb:20::/64")

// hostRows renders the hosts rows of every inventory entry (lanHosts,
// earthLAN, lanDevices, in that order) for one router view. The inventory is
// compiled source, never remote or user input: an invalid or duplicate row
// panics, so the recording fails before any file is published.
func hostRows(view routerView) []string {
	var rows []string
	for _, host := range append(append(append([]lanHost{}, lanHosts...), earthLAN...), lanDevices...) {
		entry, err := lanRows(host, view)
		if err != nil {
			panic(err)
		}
		rows = append(rows, entry...)
	}
	if err := validateRows(rows); err != nil {
		panic(err)
	}
	return rows
}

// lanRows renders one inventory entry: the canonical row, the .old alias
// while the old address is held, the .new alias, the explicit lan6 name and,
// for the router, its zone aliases.
func lanRows(h lanHost, view routerView) ([]string, error) {
	if err := checkOldIPv4(h); err != nil {
		return nil, err
	}
	names := []string{h.name, h.name + ".lan", h.name + ".lan.buetow.org"}
	names = append(names, h.aliases...)
	if h.zone == noZone {
		return []string{h.ip + " " + strings.Join(names, " ")}, nil
	}
	target, err := zoneAddress(h, view)
	if err != nil {
		return nil, err
	}
	rows := []string{target + " " + strings.Join(names, " ")}
	if !h.oldRetired {
		rows = append(rows, alias(h.ip, h.name, "old"))
	}
	rows = append(rows, alias(target, h.name, "new"))
	if h.ula != "" {
		ip, err := netip.ParseAddr(h.ula)
		if err != nil || !nativeULA.Contains(ip) {
			return nil, fmt.Errorf("invalid native ULA for %s", h.name)
		}
		rows = append(rows, h.ula+" "+h.name+".lan6 "+h.name+".lan6.buetow.org")
	}
	if h.zone == routerZone {
		rows = append(rows, alias("192.168.1.2", h.name, "upstream"), alias("10.20.0.1", h.name, "lab"), alias("10.30.0.1", h.name, "wifi"))
	}
	return rows, nil
}

// checkOldIPv4 refuses a row whose ip is not an IPv4 address.
func checkOldIPv4(h lanHost) error {
	if ip, err := netip.ParseAddr(h.ip); err != nil || !ip.Is4() {
		return fmt.Errorf("invalid old IPv4 for %s: %q", h.name, h.ip)
	}
	return nil
}

// zoneAddress is the endpoint's address in its zone: an explicit target, the
// router's address in view, or the old suffix in the lab (or Wi-Fi) subnet.
func zoneAddress(h lanHost, view routerView) (string, error) {
	if h.target != "" {
		ip, err := netip.ParseAddr(h.target)
		if err != nil || !ip.Is4() {
			return "", fmt.Errorf("invalid target for %s", h.name)
		}
		return ip.String(), nil
	}
	if h.zone == routerZone {
		switch view {
		case labView:
			return "10.20.0.1", nil
		case vpnView:
			return "10.99.0.1", nil
		default:
			return h.ip, nil
		}
	}
	target, err := labAddress(h.ip)
	if err != nil {
		return "", fmt.Errorf("%s: %w", h.name, err)
	}
	if h.zone == wifiZone {
		target = strings.Replace(target, "10.20.", "10.30.", 1)
	}
	return target, nil
}

// labAddress maps an inventory host's old 192.168.1.x LAN address to the
// same suffix in the lab subnet 10.20.0.0/24 (the former
// migrationaddress.Lab, moved here in task 974).
func labAddress(address string) (string, error) {
	ip, err := netip.ParseAddr(address)
	if err != nil || !netip.MustParsePrefix("192.168.1.0/24").Contains(ip) {
		return "", fmt.Errorf("lab mapping requires an old 192.168.1.0/24 address: %q", address)
	}
	octets := ip.As4()
	if octets[3] == 0 || octets[3] == 255 {
		return "", fmt.Errorf("lab mapping requires a host address: %q", address)
	}
	return fmt.Sprintf("10.20.0.%d", octets[3]), nil
}

func alias(ip, name, suffix string) string {
	return ip + " " + name + "." + suffix + ".lan " + name + "." + suffix + ".lan.buetow.org"
}

// validateRows refuses a malformed row or a name listed twice.
func validateRows(rows []string) error {
	names := make(map[string]string)
	for _, row := range rows {
		fields := strings.Fields(row)
		if len(fields) < 2 {
			return fmt.Errorf("invalid hosts row %q", row)
		}
		if _, err := netip.ParseAddr(fields[0]); err != nil {
			return fmt.Errorf("invalid hosts address: %w", err)
		}
		for _, name := range fields[1:] {
			if previous, exists := names[name]; exists {
				return fmt.Errorf("duplicate hosts name %s (%s, %s)", name, previous, fields[0])
			}
			names[name] = fields[0]
		}
	}
	return nil
}

// PiHoleHostRecords returns the dnsmasq lines of pi2/pi3's
// 99-f3s-lan-hosts.conf: a 60-second local TTL and one host-record per
// .buetow.org name in the lab view. The *.f3s.lan wildcard is not here; it
// has its own file and cache gate (pihole.Deployment.Compose).
func PiHoleHostRecords() []string {
	records := []string{"local-ttl=60"}
	eachFQDN(labView, func(name string, ip netip.Addr) {
		records = append(records, "host-record="+name+","+ip.String()+",60")
	})
	return records
}

// NSDRecords returns the authoritative buetow.org LAN records in the
// upstream view, generated into the frontends' zone fragment. No SOA, NS or
// wildcard: the publisher merges them into the complete zone.
func NSDRecords() []string {
	var records []string
	eachFQDN(upstreamView, func(name string, ip netip.Addr) {
		rrType := "A"
		if ip.Is6() {
			rrType = "AAAA"
		}
		records = append(records, name+". 60 IN "+rrType+" "+ip.String())
	})
	return records
}

// eachFQDN calls fn for every .buetow.org name of the view's hosts rows, in
// row order. DNS serves the explicit LAN FQDNs; short aliases stay in
// /etc/hosts.
func eachFQDN(view routerView, fn func(name string, ip netip.Addr)) {
	for _, row := range hostRows(view) {
		fields := strings.Fields(row)
		ip := netip.MustParseAddr(fields[0])
		for _, name := range fields[1:] {
			if strings.HasSuffix(name, ".buetow.org") {
				fn(name, ip)
			}
		}
	}
}
