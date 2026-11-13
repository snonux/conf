package triggerfish

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"

	. "github.com/snonux/gonf/api"
)

// localOnlyIPv6Policy keeps router autoconfiguration and temporary addressing
// off on the lab port of every local-only profile. It is two lines on
// purpose: netstart(8) turns a third word of an "inet6" line into a prefix
// length, so the former single line "inet6 -autoconf -temporary" ran
// "ifconfig igc0 inet6 -autoconf prefixlen -temporary" at boot (i44). Each
// line here reaches ifconfig verbatim. The o34 uplink stages replace both
// lines with uplinkIPv6Line (ipv6wifi.go), which netstart also passes verbatim.
const localOnlyIPv6Policy = "inet6 -autoconf\ninet6 -temporary\n"

// Profiles stages router candidates without installing or activating them.
type Profiles struct{ RequiresRoot }

// RouterProfiles carries verified migration dependencies from inventory.
type RouterProfiles struct {
	// BootCanary adds a fixed, reserved alias only to the shared profile, the
	// o34 NAT66 profile and the g24 camera-igc3 profile (see bootCanaryStage;
	// never c24's lab-igc1).
	BootCanary bool
	// RetainProvisioningAlias keeps the old 192.168.11.2 alias in every
	// candidate. This retains its address only: any active provisioning
	// clients need explicit reviewed PF permissions. j24 retired the alias:
	// the inventory leaves it false (after b24 no client shares igc0).
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

// Candidates stages inactive network, firewall and service configuration bundles.
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
	// o34 IPv6 stages (ipv6wifi.go) reuse the wifi-first wiring.
	for _, stage := range ipv6Stages() {
		profiles[stage.name] = ipv6Files(stage, data)
	}
	// g24 camera port (camera.go) builds on the o34 NAT66 profile.
	for _, stage := range cameraStages() {
		profiles[stage.name] = cameraFiles(stage, data)
	}
	// c24 lab port (lab.go) builds on the g24 camera profile.
	for _, stage := range labStages() {
		profiles[stage.name] = labFiles(stage, data)
	}
	return profiles, nil
}

// routerStages lists the six shared-wire migration stages. They have been
// historical since j24 activated lab-final, but stay renderable until the
// camera tasks (l24, n24) are done: task g74 (IQ openbsdrouter-plan
// CLEANUP.md item 10a) then prunes them, wifi-ipv6-uplink and the
// vpn-bootstrap candidate, keeping the chain wifi-ipv6-nat66 -> camera-igc3
// -> lab-igc1 -> lab-final that router-rollback builds on.
func routerStages() []routerStage {
	return []routerStage{
		{"shared-igc0", "igc0", "igc0", "igc0", true, true},
		// AP may be moved before the separate lab-switch human gate.
		{"wifi-prepare", "igc0", "igc2", "igc0", true, true},
		{"wifi-first", "igc0", "igc2", "igc0", true, true},
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
	files["hostname."+s.lab] += localOnlyIPv6Policy
	if data.BootCanary && bootCanaryStage(s.name) {
		files["hostname.igc0"] += "inet alias 10.20.0.254 255.255.255.0\n"
		files["ntpd.conf"] += "# gonf boot canary: 10.20.0.254\n"
	}
	if s.recovery {
		files["hostname.igc1"] += "inet alias 10.98.0.1 255.255.255.0\n"
	}
	return files
}

// bootCanaryStage names the profiles whose unconfirmed trial may carry the
// fixed reboot canary: the shared profile (u14) and the NAT66 profiles (o34,
// g24), whose persistence across a real reboot is part of their acceptance.
// All keep the lab zone on igc0, where the canary alias lives. c24's
// lab-igc1 is a NAT66 profile too but is excluded: its lab prefix is on igc1,
// so the igc0 canary would put 10.20.0.0/24 on two ports (router-rollback
// refuses that, and its canary_profile excludes lab-igc1 as well).
func bootCanaryStage(name string) bool {
	return name == "shared-igc0" || (nat66Stage(name) && !labPortStage(name))
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
	return profilePFWithIPv6(s, data, localOnlyIPv6Rules())
}

// localOnlyIPv6Rules is the agreed local-only native IPv6 policy of every
// migration profile: lab ULA neighbour discovery and router echo, no transit.
func localOnlyIPv6Rules() string {
	return labIPv6Rules() + "block drop quick inet6 all\n"
}

// labIPv6Rules permits only lab ULA neighbour discovery and echo to the router.
// Shared by the local-only policy and the o34 IPv6 stages, which
// must keep the lab ULA exactly as local as before.
func labIPv6Rules() string {
	return `# Native IPv6 is local-only; allow address resolution on the lab link.
pass quick on $lab_if inet6 proto icmp6 icmp6-type { neighbrsol, neighbradv }
pass in quick on $lab_if inet6 proto icmp6 from fd02:ca0b:f1cb:20::/64 to fd02:ca0b:f1cb:20::1 icmp6-type echoreq keep state
`
}

// profilePFWithIPv6 renders the shared IPv4 policy around one IPv6 rule block.
// The IPv6 block must end in a quick inet6 default deny: every IPv6 rule after
// it is unreachable, so all IPv6 permits live inside the block. The header
// names the profile without claiming it is inactive: the same bytes are the
// staged candidate and, once a trial is confirmed, the live /etc/pf.conf.
func profilePFWithIPv6(s routerStage, data RouterProfiles, ipv6 string) string {
	rules := fmt.Sprintf(`# gonf router profile %s, applied only by router-rollback. No blanket pass; wg0/wg1 are not configured here.
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
`, s.name, s.lab, s.wifi, s.camera) + ipv6
	// Each zone can arrive only on its current physical interface. In shared
	// stages this is source filtering, not isolation from same-wire attackers.
	for _, zone := range []struct{ iface, prefix string }{
		{s.lab, "10.20.0.0/24"}, {s.wifi, "10.30.0.0/24"}, {s.camera, "10.40.0.0/24"},
		{"igc1", "10.98.0.0/24"}, {"wg2", "10.99.0.0/24"},
	} {
		if zone.prefix == "10.98.0.0/24" && !s.recovery {
			continue // j24: the recovery subnet is gone, nothing to anti-spoof
		}
		rules += fmt.Sprintf("block drop in quick on ! %s inet from %s to any\n", zone.iface, zone.prefix)
	}
	if cameraZoneStage(s.name) {
		rules += cameraZoneRules()
	}
	rules += oldManagementRules(s) + `pass in on { igc0 $wifi_if } inet proto udp to { 192.168.1.2, 10.30.0.1 } port 51822 keep state
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
pass in on wg2 inet proto tcp from 10.99.0.2 to 10.20.0.138 port 2323 tag gonf_transit keep state
# n44: laptop to the Syncthing NodePort on the k3s nodes (direct sync, no relay).
pass in on wg2 inet proto tcp from 10.99.0.2 to { 10.20.0.120, 10.20.0.121, 10.20.0.122 } port 32200 tag gonf_transit keep state
pass in on wg2 inet proto tcp from 10.99.0.3 to <lab> port 443 tag gonf_transit keep state
pass in on wg2 inet proto tcp from 10.99.0.2 to 10.99.0.1 port 22 keep state
pass in on wg2 inet proto icmp from 10.99.0.2 to { 10.99.0.1, 10.20.0.0/24 } tag gonf_transit keep state
` + shellyControllerRules()
	if s.recovery {
		rules += "pass in on wg2 inet proto tcp from 10.99.0.2 to 10.98.0.0/24 port { 22, 443 } tag gonf_transit keep state\n"
		rules += "pass in on wg2 inet proto icmp from 10.99.0.2 to 10.98.0.0/24 tag gonf_transit keep state\n"
	}
	if s.name == labProfileName {
		rules += labRecoveryProofRules()
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

// shellyControllerRules permits only the existing Shelly controllers: pi0/pi1
// to both plugs, f0-f3 to the rack-fan plug, and the laptop over wg2 to both.
// Replies use state; Wi-Fi never initiates towards the lab.
//
// p34: the plugs serve native HTTPS with private-CA certificates and every
// controller (f3sctl on pi0/pi1 and the laptop, the f0-f3 boot-fan helper)
// verifies them, so only TCP443 is admitted, in every stage. TCP80 (plain
// HTTP with digest auth) was removed once all callers were accepted on HTTPS;
// the plugs still listen on 80, which the router no longer forwards.
func shellyControllerRules() string {
	return `# Shelly controllers only (native HTTPS); replies use state, no Wi-Fi initiation to lab.
pass in on $lab_if inet proto tcp from { 10.20.0.130, 10.20.0.131, 10.20.0.132, 10.20.0.133 } to 10.30.0.28 port 443 tag gonf_transit keep state
pass in on $lab_if inet proto tcp from { 10.20.0.125, 10.20.0.126 } to { 10.30.0.28, 10.30.0.29 } port 443 tag gonf_transit keep state
pass in on wg2 inet proto tcp from 10.99.0.2 to { 10.30.0.28, 10.30.0.29 } port 443 tag gonf_transit keep state
`
}

// dhcpInterfaces lists the ports dhcpd serves in a stage: the lab port once
// the lab has its own port (igc1), and Wi-Fi on igc2 from wifi-first on.
// wifi-prepare already declares the Wi-Fi subnet but leaves dhcpd off, so the
// AP keeps serving until the separate cut-over. The shared igc0 never appears.
// services.conf, the dhcpd.conf header and ACTIVATION.txt all derive from it.
func dhcpInterfaces(s routerStage) []string {
	interfaces := []string{}
	if s.lab == "igc1" {
		interfaces = append(interfaces, "igc1")
	}
	if s.wifi == "igc2" && s.name != "wifi-prepare" {
		interfaces = append(interfaces, "igc2")
	}
	return interfaces
}

// dhcpState describes dhcpd for a stage in rendered comments.
func dhcpState(s routerStage) string {
	if interfaces := dhcpInterfaces(s); len(interfaces) > 0 {
		return "dhcpd serves only " + strings.Join(interfaces, " ")
	}
	return "dhcpd stays disabled"
}

func profileDHCP(s routerStage) string {
	config := fmt.Sprintf("# gonf router profile %s: %s. Shared igc0 must NEVER run this server.\ndefault-lease-time 3600;\nmax-lease-time 7200;\n",
		s.name, dhcpState(s))
	if s.lab == "igc1" {
		config += labDHCPNetwork(s)
		config += jetKVMReservations()
	}
	if s.wifi == "igc2" {
		config += "subnet 10.30.0.0 netmask 255.255.255.0 {\n authoritative;\n range 10.30.0.100 10.30.0.239;\n option routers 10.30.0.1;\n option domain-name-servers 10.30.0.1;\n}\n"
	}
	return config
}

const labDHCPSubnet = "subnet 10.20.0.0 netmask 255.255.255.0 {\n option routers 10.20.0.1;\n option domain-name-servers 10.20.0.1;\n deny unknown-clients;\n}\n"

// labDHCPNetwork declares the lab DHCP network on igc1. OpenBSD dhcpd
// (dispatch.c discover_interfaces) matches a subnet against an interface's
// FIRST IPv4 address only and silently drops the interface when none
// matches. With the retained recovery address, igc1's address order depends
// on its history: a live activation adds 10.20.0.1 after 10.98.0.1, a boot
// follows hostname.igc1. c24 went live with dhcpd not serving igc1 for that
// reason (review B1). Both igc1 prefixes therefore share one network, the
// recovery subnet without a pool, and the server identifier is the lab
// gateway, the only igc1 address PF admits DHCP requests to.
//
// No "authoritative;" here on purpose (h24): OpenBSD dhcpd is authoritative
// by default (confpars.c root_group.authoritative = 1) and already NAKs an
// INIT-REBOOT or SELECTING request for a foreign address on igc1. A client
// still holding an ISP lease sends REBINDING requests (ciaddr set, no
// requested-address option), which dhcp.c dhcprequest() ignores whatever the
// setting, because it cannot tell them from RENEWING ones. Such a client moves
// to its reservation when the old lease expires or when it reboots.
func labDHCPNetwork(s routerStage) string {
	if !s.recovery {
		return labDHCPSubnet
	}
	return "# dhcpd binds igc1 by its first IPv4 address, either prefix; one network serves both.\n" +
		"shared-network igc1-lab {\n server-identifier 10.20.0.1;\n" +
		indent(labDHCPSubnet) +
		" subnet 10.98.0.0 netmask 255.255.255.0 {\n  deny unknown-clients;\n }\n}\n"
}

// indent prefixes every line of a block with one space.
func indent(block string) string {
	return " " + strings.ReplaceAll(strings.TrimSuffix(block, "\n"), "\n", "\n ") + "\n"
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
	return `# gonf router profile, applied only by router-rollback. System trust anchor and chroot retain OpenBSD defaults.
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
	interfaces := dhcpInterfaces(s)
	dhcp := "disabled"
	if len(interfaces) > 0 {
		dhcp = strings.Join(interfaces, " ")
	}
	return "# Declarative activation input; never source as rc.conf.local.\ndhcpd_interfaces=" + dhcp + "\nunbound=enabled\nntpd=enabled\nrad=disabled\nwg2=blocked-until-vpn-task\n"
}

// profileActivation is the review note of the six migration stages. The
// bundle stays a candidate until router-rollback activates it, so the header
// keeps saying so; per-stage facts such as DHCP come from the stage itself.
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
connected prefix. j24 retired provisioning .11.2 (RetainProvisioningAlias=false
after its dependency proof). That flag retains its address only;
unlisted active provisioning clients make this candidate unready for activation.
Inventory every such client and its required explicit PF permission first.
Recovery 10.98/24 stays on igc1 through post-rack/post-AP rollback; shared-link
hosts must not route laptop replies through its uncabled gateway.
%s
Camera updates and recorder exceptions remain closed pending measured sources/ports.
%s IPv6 forwarding is disabled and PF
blocks IPv6 transit; endpoint RA/DHCPv6 suppression remains a host requirement.
Final requires explicit address/recovery retirement acceptance. Keep all retained
host IPv4 aliases until that later gate; final files do not authorize retirement.
`, s.name, dhcpActivation(s), isolationActivation(s))
}

// isolationActivation states whether the stage's lab shares the upstream
// wire. Only the stages with the lab on igc0 are same-wire Stage A wiring;
// post-rack, post-ap and final assume the separate b24 lab cabling.
func isolationActivation(s routerStage) string {
	if s.lab == "igc0" {
		return "Same-wire Stage A is not physical isolation."
	}
	return "Lab on its own port " + s.lab + " (b24 cabling)."
}

// dhcpActivation states the stage's real DHCP role. Only Stage A
// (shared-igc0) leaves the AP's DHCP and cabling untouched; every other line
// follows dhcpInterfaces, the source of services.conf.
func dhcpActivation(s routerStage) string {
	if s.name == "shared-igc0" {
		return "Stage A preserves the existing AP .101, SSID, DHCP and cabling; it serves NO DHCP."
	}
	return "DHCP: " + dhcpState(s) + ", never on the shared igc0."
}
