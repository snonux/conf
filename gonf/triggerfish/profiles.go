package triggerfish

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"

	. "github.com/snonux/gonf/api"
)

// Profiles stages router candidates without installing or activating them.
type Profiles struct{ RequiresRoot }

// RouterProfiles carries verified migration dependencies from inventory.
type RouterProfiles struct {
	// BootCanary adds a fixed, reserved alias only to the shared profile.
	BootCanary bool
	// RetainProvisioningAlias keeps the existing 192.168.11.2 alias in every
	// candidate until verified retirement. This retains its address only: any
	// active provisioning clients need explicit reviewed PF permissions.
	RetainProvisioningAlias bool
	// TemporaryFlows are reviewed routed dependencies during mixed-address stages.
	TemporaryFlows []MigrationFlow
	// JetKVMManagement admits only independently reserved shared-wire endpoints.
	// Populate after IQ allocation and native UDP-range inspection, not discovery.
	JetKVMManagement []JetKVMManagementEndpoint
}

// JetKVMManagementEndpoint permits laptop management and native ephemeral UDP.
// Each address needs its own verified device UDP range; no inventory is activated
// by the zero-value RouterProfiles, and these permits never follow a cable move.
type JetKVMManagementEndpoint struct {
	Address        string
	UDPMin, UDPMax uint16
}

// MigrationFlow permits one source, destination and transport port while staging.
// Use literal host addresses: subnet-wide exceptions are deliberately unsupported.
type MigrationFlow struct {
	Source, Destination, Protocol string
	Port                          uint16
}

type routerStage struct {
	name, lab, wifi, camera string
	recovery, temporary     bool
}

// OptsCandidates excludes profile preparation from routine aggregate deployment.
func (Profiles) OptsCandidates() TaskOptions { return TaskOptions{Operational()} }

// Candidates stages four inactive network, firewall and service configuration bundles.
func (Profiles) Candidates() {
	EachHost(func(data RouterProfiles) {
		candidates, err := RenderProfiles(data)
		if err != nil {
			Refuse("File", "/etc/gonf/router-profiles", err)
			return
		}
		EnsureDir("/etc/gonf/router-profiles", RootPrivate)
		for name, files := range candidates {
			dir := "/etc/gonf/router-profiles/" + name
			EnsureDir(dir, RootPrivate)
			for name, content := range files {
				File(dir+"/"+name, WithContent(content), RootPrivate)
			}
		}
	})
}

// RenderProfiles returns inactive candidates for offline review and native validation.
// It performs no I/O and never reads keys or modifies router state.
func RenderProfiles(data RouterProfiles) (map[string]map[string]string, error) {
	if err := validateMigrationFlows(data.TemporaryFlows); err != nil {
		return nil, err
	}
	if err := validateJetKVMManagement(data); err != nil {
		return nil, err
	}
	profiles := make(map[string]map[string]string)
	for _, stage := range routerStages() {
		profiles[stage.name] = profileFiles(stage, data)
	}
	return profiles, nil
}

func routerStages() []routerStage {
	return []routerStage{
		{"shared-igc0", "igc0", "igc0", "igc0", true, true},
		{"post-rack", "igc1", "igc0", "igc0", true, true},
		{"post-ap", "igc1", "igc2", "igc3", true, true},
		{"final", "igc1", "igc2", "igc3", false, false},
	}
}

func profileFiles(s routerStage, data RouterProfiles) map[string]string {
	files := map[string]string{
		"hostname.igc0": "inet 192.168.1.2 255.255.255.0\n",
		"hostname.igc1": "up\n", "hostname.igc2": "up\n", "hostname.igc3": "up\n",
		"mygate":                   "192.168.1.1\n",
		"sysctl.conf.fragment":     "net.inet.ip.forwarding=1\nnet.inet6.ip6.forwarding=0\n",
		"ntpd.conf":                "listen on 10.20.0.1\nlisten on 10.30.0.1\nservers pool.ntp.org\n",
		"pf.conf":                  profilePF(s, data),
		"dhcpd.conf":               profileDHCP(s),
		"jetkvm-reservations.conf": jetKVMReservations(),
		"unbound.conf":             profileDNS(),
		"services.conf":            profileServices(s),
		"ACTIVATION.txt":           profileActivation(s),
		// Deliberately unusable until the separate VPN task supplies real keys/peers.
		"hostname.wg2.template": "# INACTIVE: VPN task must supply private key and authenticated /32 peers.\n# inet 10.99.0.1 255.255.255.0\n# wgport 51822\n# Laptop AllowedIPs 10.99.0.2/32; phone 10.99.0.3/32.\n",
	}
	if data.RetainProvisioningAlias {
		files["hostname.igc0"] += "inet alias 192.168.11.2 255.255.255.0\n"
	}
	for _, zone := range []struct{ iface, address string }{
		{s.lab, "10.20.0.1"}, {s.wifi, "10.30.0.1"}, {s.camera, "10.40.0.1"},
	} {
		files["hostname."+zone.iface] += "inet alias " + zone.address + " 255.255.255.0\n"
	}
	files["hostname."+s.lab] += "inet6 alias fd02:ca0b:f1cb:20::1 64\n"
	files["hostname."+s.lab] += "inet6 -autoconf -temporary\n"
	if data.BootCanary && s.name == "shared-igc0" {
		files["hostname.igc0"] += "inet alias 10.20.0.254 255.255.255.0\n"
		files["ntpd.conf"] += "# gonf boot canary: 10.20.0.254\n"
	}
	if s.recovery {
		files["hostname.igc1"] += "inet alias 10.98.0.1 255.255.255.0\n"
	}
	return files
}

func validateMigrationFlows(flows []MigrationFlow) error {
	for _, flow := range flows {
		for _, address := range []string{flow.Source, flow.Destination} {
			ip, err := netip.ParseAddr(address)
			if err != nil || !ip.Is4() || !ip.IsPrivate() {
				return fmt.Errorf("migration flow requires private literal IPv4 hosts: %q", address)
			}
		}
		if !slices.Contains([]string{"tcp", "udp"}, flow.Protocol) || flow.Port == 0 {
			return fmt.Errorf("invalid migration transport %q port %d", flow.Protocol, flow.Port)
		}
	}
	return nil
}

func validateJetKVMManagement(data RouterProfiles) error {
	if len(data.JetKVMManagement) > 0 && !data.RetainProvisioningAlias {
		return fmt.Errorf("JetKVM management requires retained shared provisioning alias")
	}
	seen := make(map[string]bool)
	for _, endpoint := range data.JetKVMManagement {
		if !slices.Contains([]string{"192.168.11.140", "192.168.11.141", "192.168.11.142", "192.168.11.143"}, endpoint.Address) {
			return fmt.Errorf("unreviewed JetKVM management address %q", endpoint.Address)
		}
		if seen[endpoint.Address] || endpoint.UDPMin == 0 || endpoint.UDPMax < endpoint.UDPMin {
			return fmt.Errorf("duplicate JetKVM address or invalid native UDP range")
		}
		seen[endpoint.Address] = true
	}
	return nil
}

func profilePF(s routerStage, data RouterProfiles) string {
	rules := fmt.Sprintf(`# Inactive candidate: %s. No blanket pass; wg0/wg1 are not configured here.
lab_if = "%s"
wifi_if = "%s"
cam_if = "%s"
set skip on lo
set state-policy floating
# Non-public destinations cannot use the public-internet exception.
table <nonpublic> const { 0.0.0.0/8, 10.0.0.0/8, 100.64.0.0/10, 127.0.0.0/8, 169.254.0.0/16, 172.16.0.0/12, 192.0.0.0/24, 192.0.2.0/24, 192.168.0.0/16, 198.18.0.0/15, 198.51.100.0/24, 203.0.113.0/24, 224.0.0.0/4, 240.0.0.0/4 }
table <lab> const { 10.20.0.0/24 }
table <wifi> const { 10.30.0.0/24 }
table <outdoor> const { 10.40.0.11, 10.40.0.12, 10.40.0.13, 10.40.0.14 }
# Intentionally empty; recorder source/ports require separate acceptance.
table <recorders> persist
# Empty and no permit/NAT rule: firmware updates remain closed.
table <camera_updates> persist
match out on igc0 inet from { 10.20.0.0/24, 10.30.0.0/24 } to ! <nonpublic> nat-to 192.168.1.2
block return log all
# Native IPv6 is local-only; allow address resolution on the lab link.
pass quick on $lab_if inet6 proto icmp6 icmp6-type { neighbrsol, neighbradv }
pass in quick on $lab_if inet6 proto icmp6 from fd02:ca0b:f1cb:20::/64 to fd02:ca0b:f1cb:20::1 icmp6-type echoreq keep state
block drop quick inet6 all
`, s.name, s.lab, s.wifi, s.camera)
	// Each zone can arrive only on its current physical interface. In shared
	// stages this is source filtering, not isolation from same-wire attackers.
	for _, zone := range []struct{ iface, prefix string }{
		{s.lab, "10.20.0.0/24"}, {s.wifi, "10.30.0.0/24"}, {s.camera, "10.40.0.0/24"},
		{"igc1", "10.98.0.0/24"}, {"wg2", "10.99.0.0/24"},
	} {
		rules += fmt.Sprintf("block drop in quick on ! %s inet from %s to any\n", zone.iface, zone.prefix)
	}
	rules += `# Preserve fresh old-address router management from the existing LAN.
pass in on igc0 inet proto tcp from 192.168.1.0/24 to 192.168.1.2 port 22 keep state
pass in on igc0 inet proto icmp from 192.168.1.0/24 to 192.168.1.2 keep state
pass in on { igc0 $wifi_if } inet proto udp to { 192.168.1.2, 10.30.0.1 } port 51822 keep state
# Router-originated public IPv4 services, including DNS and clock sync.
pass out on igc0 inet from 192.168.1.2 to ! <nonpublic> keep state
# A forwarded packet is filtered again on egress. Only ingress-authorized
# transit carries this internal tag; an inbound state alone is insufficient.
pass out inet tagged gonf_transit keep state
pass in on $lab_if inet from <lab> to ! <nonpublic> tag gonf_transit keep state
pass in on $wifi_if inet from <wifi> to ! <nonpublic> tag gonf_transit keep state
# Local DNS, clock sync and management. No private upstream resolver dependency.
pass in on $lab_if inet proto { tcp udp } from <lab> to 10.20.0.1 port 53 keep state
pass in on $wifi_if inet proto { tcp udp } from <wifi> to 10.30.0.1 port 53 keep state
pass in on $lab_if inet proto udp from <lab> to 10.20.0.1 port 123 keep state
pass in on $wifi_if inet proto udp from <wifi> to 10.30.0.1 port 123 keep state
pass in on $lab_if inet proto tcp from <lab> to 10.20.0.1 port 22 keep state
pass in on $lab_if inet proto icmp from <lab> to 10.20.0.1 keep state
# Distinct authenticated peers; no NAT between wg2 and the lab.
# Pi-hole DNS contract: both peers, both DNS transports, only the resolver pair.
pass in on wg2 inet proto { tcp udp } from { 10.99.0.2, 10.99.0.3 } to { 10.20.0.127, 10.20.0.128 } port 53 tag gonf_transit keep state
pass in on wg2 inet proto tcp from 10.99.0.2 to <lab> port { 22, 80, 443 } tag gonf_transit keep state
pass in on wg2 inet proto tcp from 10.99.0.3 to <lab> port 443 tag gonf_transit keep state
pass in on wg2 inet proto tcp from 10.99.0.2 to 10.99.0.1 port 22 keep state
pass in on wg2 inet proto icmp from 10.99.0.2 to { 10.99.0.1, 10.20.0.0/24 } tag gonf_transit keep state
# Shelly controllers only; replies use state, no Wi-Fi initiation to lab.
pass in on $lab_if inet proto tcp from { 10.20.0.130, 10.20.0.131, 10.20.0.132, 10.20.0.133 } to 10.30.0.28 port 80 tag gonf_transit keep state
pass in on $lab_if inet proto tcp from { 10.20.0.125, 10.20.0.126 } to { 10.30.0.28, 10.30.0.29 } port 80 tag gonf_transit keep state
pass in on wg2 inet proto tcp from 10.99.0.2 to { 10.30.0.28, 10.30.0.29 } port 80 tag gonf_transit keep state
`
	if s.recovery {
		rules += "pass in on wg2 inet proto tcp from 10.99.0.2 to 10.98.0.0/24 port { 22, 443 } tag gonf_transit keep state\n"
		rules += "pass in on wg2 inet proto icmp from 10.99.0.2 to 10.98.0.0/24 tag gonf_transit keep state\n"
	}
	if s.name == "shared-igc0" {
		for _, endpoint := range data.JetKVMManagement {
			// Exact reserved destinations only. Media uses device-native ephemeral
			// UDP; the existing gonf_transit tag admits its routed egress/replies.
			rules += fmt.Sprintf("pass in on wg2 inet proto tcp from 10.99.0.2 to %s port { 22, 80 } tag gonf_transit keep state\n", endpoint.Address)
			rules += fmt.Sprintf("pass in on wg2 inet proto udp from 10.99.0.2 to %s port %d:%d tag gonf_transit keep state\n", endpoint.Address, endpoint.UDPMin, endpoint.UDPMax)
		}
	}
	if s.lab == "igc1" {
		rules += "pass in on igc1 inet proto udp from any port 68 to { 255.255.255.255, 10.20.0.1 } port 67 keep state\npass out on igc1 inet proto udp from 10.20.0.1 port 67 to any port 68 keep state\n"
	}
	if s.wifi == "igc2" {
		rules += "pass in on igc2 inet proto udp from any port 68 to { 255.255.255.255, 10.30.0.1 } port 67 keep state\npass out on igc2 inet proto udp from 10.30.0.1 port 67 to any port 68 keep state\n"
	}
	if s.temporary {
		for _, flow := range data.TemporaryFlows {
			// Explicit source and target only; no generic private-network pass.
			rules += fmt.Sprintf("pass in on { igc0 $lab_if } inet proto %s from %s to %s port %d tag gonf_transit keep state\n", flow.Protocol, flow.Source, flow.Destination, flow.Port)
		}
	}
	return rules
}

func profileDHCP(s routerStage) string {
	config := "# Inactive candidate. Shared igc0 must NEVER run this server.\ndefault-lease-time 3600;\nmax-lease-time 7200;\n"
	if s.lab == "igc1" {
		config += "subnet 10.20.0.0 netmask 255.255.255.0 {\n option routers 10.20.0.1;\n option domain-name-servers 10.20.0.1;\n deny unknown-clients;\n}\n"
		config += jetKVMReservations()
	}
	if s.wifi == "igc2" {
		config += "subnet 10.30.0.0 netmask 255.255.255.0 {\n range 10.30.0.100 10.30.0.239;\n option routers 10.30.0.1;\n option domain-name-servers 10.30.0.1;\n}\n"
	}
	return config
}

func jetKVMReservations() string {
	return `# Verified device IDs; do not infer f-host attachments.
host jetkvm-46793b0d4c8d45d { hardware ethernet 30:52:53:03:57:38; fixed-address 10.20.0.151; }
host jetkvm-e10a0883debf7d4a { hardware ethernet 30:52:53:08:97:97; fixed-address 10.20.0.158; }
host jetkvm-f72978d7f87aa969 { hardware ethernet 30:52:53:06:8b:21; fixed-address 10.20.0.191; }
host jetkvm-cee6ab4df2f88754 { hardware ethernet 30:52:53:04:a4:4b; fixed-address 10.20.0.198; }
`
}

func profileDNS() string {
	return `# Inactive candidate. System trust anchor and chroot retain OpenBSD defaults.
server:
 interface: 127.0.0.1
 interface: 10.20.0.1
 interface: 10.30.0.1
 access-control: 127.0.0.0/8 allow
 access-control: 10.20.0.0/24 allow
 access-control: 10.30.0.0/24 allow
 access-control: 0.0.0.0/0 refuse
 do-ip6: no
 hide-identity: yes
 hide-version: yes
forward-zone:
 name: "."
 forward-addr: 9.9.9.9
 forward-addr: 1.1.1.1
`
}

func profileServices(s routerStage) string {
	interfaces := []string{}
	if s.lab == "igc1" {
		interfaces = append(interfaces, "igc1")
	}
	if s.wifi == "igc2" {
		interfaces = append(interfaces, "igc2")
	}
	dhcp := "disabled"
	if len(interfaces) > 0 {
		dhcp = strings.Join(interfaces, " ")
	}
	return "# Declarative activation input; never source as rc.conf.local.\ndhcpd_interfaces=" + dhcp + "\nunbound=enabled\nntpd=enabled\nrad=disabled\nwg2=blocked-until-vpn-task\n"
}

func profileActivation(s routerStage) string {
	return fmt.Sprintf(`INACTIVE CANDIDATE: %s
This bundle is configuration input for the separately gated rollback/activation task l14.
Do not copy files or run netstart directly. No activation command is provided here.
Required: native pfctl -nf and dhcpd -n -c (when enabled), unbound-checkconf,
verified wg2 keys/peers, independent timed + persistent + boot rollback, and
fresh old/new management tests. Preserve /etc/sysctl.conf unrelated keys.
Remove moved IPv4 AND ULA addresses/connected routes from the old interface
BEFORE adding them to the target. Netstart alone does not remove old aliases.
Disable obsolete physical IPv6 autoconfiguration and remove learned default
routes/global addresses while retaining intended link-local/ULA and wg0 state.
Keep the existing upstream .1.2 and gateway .1.1 throughout. Never duplicate a
connected prefix. Retire provisioning .11.2 only after separate dependency proof
and setting RetainProvisioningAlias=false. That flag retains its address only;
unlisted active provisioning clients make this candidate unready for activation.
Inventory every such client and its required explicit PF permission first.
Recovery 10.98/24 stays on igc1 through post-rack/post-AP rollback; shared-link
hosts must not route laptop replies through its uncabled gateway.
Stage A preserves the existing AP .101, SSID, DHCP and cabling; it serves NO DHCP.
Camera updates and recorder exceptions remain closed pending measured sources/ports.
Same-wire Stage A is not physical isolation. IPv6 forwarding is disabled and PF
blocks IPv6 transit; endpoint RA/DHCPv6 suppression remains a host requirement.
Final requires explicit address/recovery retirement acceptance. Keep all retained
host IPv4 aliases until that later gate; final files do not authorize retirement.
`, s.name)
}
