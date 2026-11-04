package etchosts

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/snonux/conf/gonf/frontends"
	"github.com/snonux/conf/gonf/migrationaddress"
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
var migrationDevices = []lanHost{
	{name: "openwrt", ip: "192.168.1.101", migration: migrationHost{zone: apZone, target: "10.30.0.2"}},
	{name: "jetkvm-46793b0d4c8d45d", ip: "192.168.1.248", migration: migrationHost{zone: labZone, target: "10.20.0.151"}},
	{name: "jetkvm-e10a0883debf7d4a", ip: "192.168.1.241", migration: migrationHost{zone: labZone, target: "10.20.0.158"}},
	{name: "jetkvm-f72978d7f87aa969", ip: "192.168.1.244", migration: migrationHost{zone: labZone, target: "10.20.0.191"}},
	{name: "jetkvm-cee6ab4df2f88754", ip: "192.168.1.245", migration: migrationHost{zone: labZone, target: "10.20.0.198"}},
}

// RenderMigration prepares an offline candidate. staging keeps canonical old
// addresses; target promotes lab/Wi-Fi endpoints but holds AP management old;
// ap-target additionally promotes the AP, for use only after d24 acceptance.
// routerView is upstream, lab or wifi; it controls only the canonical router
// name. Nothing here installs a file, registers a task or claims acceptance.
func RenderMigration(profile, routerView string) (MigrationCandidate, error) {
	if profile != "staging" && profile != "target" && profile != "ap-target" {
		return MigrationCandidate{}, fmt.Errorf("unknown migration profile %q", profile)
	}
	if routerView != "upstream" && routerView != "lab" && routerView != "wifi" {
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
	if profile == "staging" || (h.migration.zone == apZone && profile != "ap-target") {
		canonical = h.ip
	}
	rows := []string{canonical + " " + strings.Join(names, " "), migrationAlias(h.ip, h.name, "old"), migrationAlias(target, h.name, "new")}
	if h.migration.recovery {
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
		default:
			return h.ip, nil
		}
	}
	target, err := migrationaddress.Lab(h.ip)
	if err != nil {
		return "", fmt.Errorf("%s: %w", h.name, err)
	}
	if h.migration.zone == wifiZone {
		target = strings.Replace(target, "10.20.", "10.30.", 1)
	}
	return target, nil
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
