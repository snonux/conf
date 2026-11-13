package etchosts

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/snonux/conf/gonf/frontends"
)

type migrationZone uint8

const (
	labZone migrationZone = iota + 1
	wifiZone
	routerZone
	apZone
)

// migrationHost extends the existing address inventory without changing its
// live renderers. target is explicit only when suffix preservation is wrong.
type migrationHost struct {
	zone     migrationZone
	target   string
	recovery bool
	ula      string
	hold     bool // Canonical DHCP device address waits for its measured lease.
	// oldRetired marks an endpoint that no longer holds its old 192.168.1.x
	// address (i24, 2026-11-11; Shelly and JetKVM moved off the ISP LAN), so
	// the accepted profile renders no <host>.old alias for it. ip stays the
	// suffix source for the derived target and recovery addresses.
	oldRetired bool
}

// MigrationCandidate contains inactive fragments, never complete /etc/hosts
// replacements. Preserve each destination's loopback, registry and local rows.
type MigrationCandidate struct {
	Profile          string
	RouterView       string
	HostsBlock       []string
	WireGuardBlock   []string
	PiHoleDNSMasq    []string
	AuthoritativeLAN []string
}

// These previously unrepresented devices have one identity record here. The
// JetKVM old addresses are authenticated 2026-11-04 leases, not target suffixes.
// Device-ID names deliberately live outside the service wildcard namespace.
// h24 (2026-11-10): each JetKVM was rebooted, took its reserved lease from
// triggerfish on igc1 and answered with its device ID on the target, so the
// canonical names are no longer held at the ISP lease. 274 (2026-11-11)
// dropped their .old aliases: a former ISP lease may now belong to anyone.
var migrationDevices = []lanHost{
	{name: "openwrt", ip: "192.168.1.101", migration: migrationHost{zone: apZone, target: "10.30.0.2"}},
	{name: "jetkvm-46793b0d4c8d45d", ip: "192.168.1.248", migration: migrationHost{zone: labZone, target: "10.20.0.151", oldRetired: true}},
	{name: "jetkvm-e10a0883debf7d4a", ip: "192.168.1.241", migration: migrationHost{zone: labZone, target: "10.20.0.158", oldRetired: true}},
	{name: "jetkvm-f72978d7f87aa969", ip: "192.168.1.244", migration: migrationHost{zone: labZone, target: "10.20.0.191", oldRetired: true}},
	{name: "jetkvm-cee6ab4df2f88754", ip: "192.168.1.245", migration: migrationHost{zone: labZone, target: "10.20.0.198", oldRetired: true}},
}

// RenderMigration prepares a mapping view. staging keeps canonical old
// addresses; target promotes lab/Wi-Fi endpoints but holds AP management old;
// ap-target additionally promotes the AP, for use only after d24 acceptance.
// accepted promotes verified endpoints while holding AP management and any
// device still marked hold (none since the h24 JetKVM leases), and omits the
// .old alias of every endpoint marked oldRetired (task 274). The historical
// staging/target/ap-target previews keep all .old aliases.
// routerView is upstream, lab, wifi or vpn; it controls only the canonical router
// name. Nothing here installs a file, registers a task or claims acceptance.
func RenderMigration(profile, routerView string) (MigrationCandidate, error) {
	if profile != "staging" && profile != "target" && profile != "ap-target" && profile != "accepted" {
		return MigrationCandidate{}, fmt.Errorf("unknown migration profile %q", profile)
	}
	if routerView != "upstream" && routerView != "lab" && routerView != "wifi" && routerView != "vpn" {
		return MigrationCandidate{}, fmt.Errorf("unknown router view %q", routerView)
	}
	if profile == "staging" && routerView != "upstream" {
		return MigrationCandidate{}, fmt.Errorf("staging must preserve upstream router/controller management")
	}
	candidate := MigrationCandidate{Profile: profile, RouterView: routerView}
	hosts := append(append(append([]lanHost{}, lanHosts...), earthLAN...), migrationDevices...)
	for _, host := range hosts {
		rows, err := migrationRows(host, profile, routerView)
		if err != nil {
			return MigrationCandidate{}, err
		}
		candidate.HostsBlock = append(candidate.HostsBlock, rows...)
	}
	if err := validateMigrationRows(candidate.HostsBlock); err != nil {
		return MigrationCandidate{}, err
	}
	peers := append(frontends.StandaloneWireGuardAddresses(), earthWireGuard...)
	candidate.WireGuardBlock = frontends.WireGuardHostLines(peers...)
	candidate.PiHoleDNSMasq, candidate.AuthoritativeLAN = migrationDNS(candidate.HostsBlock)
	vip := ""
	for _, row := range candidate.HostsBlock {
		fields := strings.Fields(row)
		if fields[1] == "f3s-storage-ha" {
			vip = fields[0]
		}
	}
	if vip == "" {
		return MigrationCandidate{}, fmt.Errorf("storage VIP missing from shared inventory")
	}
	candidate.PiHoleDNSMasq = append(candidate.PiHoleDNSMasq, "address=/.f3s.lan.buetow.org/"+vip)
	candidate.AuthoritativeLAN = append(candidate.AuthoritativeLAN, "*.f3s.lan.buetow.org. 60 IN A "+vip)
	return candidate, nil
}

func migrationRows(h lanHost, profile, routerView string) ([]string, error) {
	old, err := netip.ParseAddr(h.ip)
	if err != nil || !old.Is4() {
		return nil, fmt.Errorf("invalid old IPv4 for %s: %q", h.name, h.ip)
	}
	names := []string{h.name, h.name + ".lan", h.name + ".lan.buetow.org"}
	names = append(names, h.aliases...)
	if h.migration.zone == 0 {
		return []string{h.ip + " " + strings.Join(names, " ")}, nil
	}
	target, err := migrationTarget(h, routerView)
	if err != nil {
		return nil, err
	}
	canonical := target
	if profile == "staging" || (h.migration.zone == apZone && profile != "ap-target") ||
		(profile == "accepted" && h.migration.hold) {
		canonical = h.ip
	}
	rows := []string{canonical + " " + strings.Join(names, " ")}
	if profile != "accepted" || !h.migration.oldRetired {
		rows = append(rows, migrationAlias(h.ip, h.name, "old"))
	}
	rows = append(rows, migrationAlias(target, h.name, "new"))
	// j24 retired the igc1 recovery subnet 10.98.0.0/24 (hosts and router),
	// so the accepted view has no .recovery names; the historical staging
	// and target previews keep them.
	if h.migration.recovery && profile != "accepted" {
		suffix := old.As4()[3]
		if h.migration.zone == routerZone {
			suffix = 1
		}
		rows = append(rows, migrationAlias(fmt.Sprintf("10.98.0.%d", suffix), h.name, "recovery"))
	}
	if h.migration.ula != "" {
		ip, err := netip.ParseAddr(h.migration.ula)
		if err != nil || !netip.MustParsePrefix("fd02:ca0b:f1cb:20::/64").Contains(ip) {
			return nil, fmt.Errorf("invalid native ULA for %s", h.name)
		}
		rows = append(rows, h.migration.ula+" "+h.name+".lan6 "+h.name+".lan6.buetow.org")
	}
	if h.migration.zone == routerZone {
		rows = append(rows, migrationAlias("192.168.1.2", h.name, "upstream"), migrationAlias("10.20.0.1", h.name, "lab"), migrationAlias("10.30.0.1", h.name, "wifi"))
	}
	return rows, nil
}

func migrationTarget(h lanHost, view string) (string, error) {
	if h.migration.target != "" {
		ip, err := netip.ParseAddr(h.migration.target)
		if err != nil || !ip.Is4() {
			return "", fmt.Errorf("invalid migration target for %s", h.name)
		}
		return ip.String(), nil
	}
	if h.migration.zone == routerZone {
		switch view {
		case "lab":
			return "10.20.0.1", nil
		case "wifi":
			return "10.30.0.1", nil
		case "vpn":
			return "10.99.0.1", nil
		default:
			return h.ip, nil
		}
	}
	target, err := labAddress(h.ip)
	if err != nil {
		return "", fmt.Errorf("%s: %w", h.name, err)
	}
	if h.migration.zone == wifiZone {
		target = strings.Replace(target, "10.20.", "10.30.", 1)
	}
	return target, nil
}

// labAddress maps an inventory host's old 192.168.1.x LAN address to the
// same suffix in the lab subnet 10.20.0.0/24. migrationTarget uses it for
// every profile; the accepted one is the live fleet view. It is the former
// migrationaddress.Lab, whose other callers (the service candidates, task
// 974, and the storage candidates, task 874) are gone with that package.
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

func migrationAlias(ip, name, suffix string) string {
	return ip + " " + name + "." + suffix + ".lan " + name + "." + suffix + ".lan.buetow.org"
}

func validateMigrationRows(rows []string) error {
	names := make(map[string]string)
	for _, row := range rows {
		fields := strings.Fields(row)
		if len(fields) < 2 {
			return fmt.Errorf("invalid candidate hosts row %q", row)
		}
		if _, err := netip.ParseAddr(fields[0]); err != nil {
			return fmt.Errorf("invalid candidate address: %w", err)
		}
		for _, name := range fields[1:] {
			if previous, exists := names[name]; exists {
				return fmt.Errorf("duplicate candidate name %s (%s, %s)", name, previous, fields[0])
			}
			names[name] = fields[0]
		}
	}
	return nil
}

func migrationDNS(rows []string) ([]string, []string) {
	dnsmasq := []string{"# INACTIVE candidate; merge only after endpoint acceptance", "local-ttl=60"}
	zone := []string{"; INACTIVE buetow.org LAN fragment; merge, never replace the public zone"}
	for _, row := range rows {
		fields := strings.Fields(row)
		ip := netip.MustParseAddr(fields[0])
		rrType := "A"
		if ip.Is6() {
			rrType = "AAAA"
		}
		for _, name := range fields[1:] {
			// DNS serves explicit LAN FQDNs; short aliases stay in /etc/hosts.
			if !strings.HasSuffix(name, ".buetow.org") {
				continue
			}
			dnsmasq = append(dnsmasq, "host-record="+name+","+ip.String()+",60")
			zone = append(zone, name+". 60 IN "+rrType+" "+ip.String())
		}
	}
	return dnsmasq, zone
}
