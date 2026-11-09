package triggerfish

import (
	"fmt"
	"strings"
)

// o34 Wi-Fi internet IPv6 through NAT66. Design and acceptance criteria live
// in IQ topics/infra/openbsdrouter-plan/IPV6-WIFI.md. The Vivacom CPE answers
// DHCPv6 IA_PD with NoPrefixAvail, so there is no routed prefix for Wi-Fi:
// triggerfish takes its own SLAAC address from the CPE LAN /64 and Wi-Fi gets
// a ULA /64 translated to that address. Unsolicited inbound IPv6 has nothing
// to reach: the ULA is unroutable from the internet and PF has no inbound
// permit on the uplink.
//
// Two stages share the current verified wiring (wifi-first: lab and cameras
// still share igc0 with the ISP LAN, Wi-Fi is on igc2) so each can run as its
// own guarded trial: the uplink first, then Wi-Fi on top of it.
const (
	ipv6UplinkProfileName = "wifi-ipv6-uplink"
	ipv6NAT66ProfileName  = "wifi-ipv6-nat66"
	// ipv6Uplink is the ISP-facing port in every wiring stage.
	ipv6Uplink = "igc0"
	// The local-only policy line is replaced on the uplink: slaacd must learn
	// the ISP default route there. netstart(8) passes "inet6 autoconf ..."
	// verbatim; it mangles "inet6 -autoconf -temporary" into a prefixlen
	// argument, so the uplink stages also stop carrying that defect (i44
	// tracks it for the other profiles). -temporary keeps one stable RFC 7217
	// address, which is the NAT66 egress address.
	localOnlyIPv6Line = "inet6 -autoconf -temporary\n"
	uplinkIPv6Line    = "inet6 autoconf -temporary\n"
	// Wi-Fi ULA: subnet 30 of the lab's fd02:ca0b:f1cb::/48, matching 10.30.
	wifiULAPrefix = "fd02:ca0b:f1cb:30::/64"
	wifiULARouter = "fd02:ca0b:f1cb:30::1"
	// nat66EgressTable holds the uplink's usable SLAAC address. PF cannot use
	// nat-to (igc0) here: a dynamic interface pool rotates over every
	// non-link-local address, including the lab ULA on the same port.
	// triggerfish.NAT66.Egress keeps the table current.
	nat66EgressTable = "nat66_egress"
)

// ipv6Stages lists the o34 stages. Both use the wifi-first wiring.
func ipv6Stages() []routerStage {
	return []routerStage{
		{ipv6UplinkProfileName, "igc0", "igc2", "igc0", true, true},
		{ipv6NAT66ProfileName, "igc0", "igc2", "igc0", true, true},
	}
}

// ipv6Files derives an o34 stage from the stage's IPv4/ULA files. The uplink
// stage changes only igc0 SLAAC and the IPv6 PF block. The NAT66 stage adds
// the Wi-Fi ULA, rad(8), IPv6 forwarding and translation. IPv4 output stays
// byte-identical to wifi-first apart from the profile name in PF.
func ipv6Files(s routerStage, data RouterProfiles) map[string]string {
	files := profileFiles(s, data)
	uplink := "hostname." + ipv6Uplink
	files[uplink] = strings.Replace(files[uplink], localOnlyIPv6Line, uplinkIPv6Line, 1)
	files["ACTIVATION.txt"] = ipv6Activation(s)
	if s.name == ipv6UplinkProfileName {
		files["pf.conf"] = profilePFWithIPv6(s, data, labIPv6Rules()+uplinkIPv6Rules()+ipv6Tail())
		return files
	}
	// rad(8) needs a link-local source; assigning the ULA creates one.
	files["hostname."+s.wifi] += "inet6 alias " + wifiULARouter + " 64\n"
	files["sysctl.conf.fragment"] = "net.inet.ip.forwarding=1\nnet.inet6.ip6.forwarding=1\n"
	files["rad.conf"] = nat66RAD(s)
	files["services.conf"] = strings.Replace(profileServices(s), "rad=disabled\n", "rad=enabled\n", 1)
	files["pf.conf"] = profilePFWithIPv6(s, data, labIPv6Rules()+uplinkIPv6Rules()+wifiNAT66Rules()+ipv6Tail())
	return files
}

func nat66RAD(s routerStage) string {
	return fmt.Sprintf(`# o34 NAT66: advertise ONLY on Wi-Fi, never on %s (ISP LAN and shared lab
# wire) or the camera port. The prefix is the Wi-Fi ULA configured on %s.
# No RDNSS: dual-stack clients keep DHCPv4 DNS 10.30.0.1, which returns AAAA.
interface %s
`, ipv6Uplink, s.wifi, s.wifi)
}

// uplinkIPv6Rules lets triggerfish itself use IPv6 through the CPE: SLAAC on
// the uplink, router-originated egress from global addresses only. The lab
// rules before it already pass neighbour discovery on the lab port, which is
// the uplink in the wifi-first wiring; the uplink rule repeats it so a later
// rewiring keeps ND on the uplink.
func uplinkIPv6Rules() string {
	return fmt.Sprintf(`# o34: router IPv6 via the CPE LAN /64 (SLAAC). Stateful, no inbound permit.
up6_if = "%s"
table <nonglobal6> const { ::/8, 100::/64, 2001:db8::/32, 2002::/16, 3fff::/20, fc00::/7, fe80::/10, fec0::/10, ff00::/8 }
# ULA (lab included) and other non-global sources never leave towards the internet.
block drop out log quick on $up6_if inet6 from <nonglobal6> to ! <nonglobal6>
# The lab ULA never transits; logged so a test route shows the drop in pflog.
block drop in log quick on $lab_if inet6 from fd02:ca0b:f1cb:20::/64 to ! <nonglobal6>
pass quick on $up6_if inet6 proto icmp6 icmp6-type { neighbrsol, neighbradv }
# MLD query/report/done and MLDv2 report keep solicited-node groups working.
pass quick on $up6_if inet6 proto icmp6 from { fe80::/10, :: } to ff02::/16 icmp6-type { 130, 131, 132, 143 }
pass in quick on $up6_if inet6 proto icmp6 from fe80::/10 to { ff02::1, fe80::/10 } icmp6-type routeradv
pass out quick on $up6_if inet6 proto icmp6 from { fe80::/10, :: } to ff02::2 icmp6-type routersol
`, ipv6Uplink)
}

// wifiNAT66Rules gives Wi-Fi outbound IPv6 only. The match rule translates
// before the filter rules below see the packet, so the non-global egress block
// never catches translated Wi-Fi traffic, and an empty egress table fails
// closed (PF drops what it cannot translate).
func wifiNAT66Rules() string {
	return fmt.Sprintf(`# o34 NAT66: Wi-Fi ULA to the uplink's SLAAC address, outbound only.
wifi6 = "%s"
table <%s> persist
match out on $up6_if inet6 from $wifi6 to ! <nonglobal6> nat-to <%s>
pass quick on $wifi_if inet6 proto icmp6 icmp6-type { neighbrsol, neighbradv }
pass quick on $wifi_if inet6 proto icmp6 from { fe80::/10, :: } to ff02::/16 icmp6-type { 130, 131, 132, 143 }
pass in quick on $wifi_if inet6 proto icmp6 from { fe80::/10, :: } to ff02::2 icmp6-type routersol
pass out quick on $wifi_if inet6 proto icmp6 from fe80::/10 to { ff02::1, fe80::/10 } icmp6-type routeradv
# Only the ISP router advertises to triggerfish, only triggerfish to Wi-Fi.
block drop log quick inet6 proto icmp6 icmp6-type { routeradv, redir }
# Anti-spoofing in both directions for the Wi-Fi ULA.
block drop in log quick on $wifi_if inet6 from ! $wifi6
block drop in log quick on ! $wifi_if inet6 from $wifi6
# Diagnostics: Wi-Fi may ping its gateway address, nothing else on the router.
pass in quick on $wifi_if inet6 proto icmp6 from $wifi6 to %s icmp6-type echoreq keep state
# Wi-Fi gets internet only: not the router, the ISP LAN, the lab ULA or any
# other non-global destination.
block drop in log quick on $wifi_if inet6 to { (self), ($up6_if:network), <nonglobal6> }
pass in quick on $wifi_if inet6 from $wifi6 to ! <nonglobal6> tag gonf_transit6 keep state
pass out quick on $up6_if inet6 to ! <nonglobal6> tagged gonf_transit6 keep state
`, wifiULAPrefix, nat66EgressTable, nat66EgressTable, wifiULARouter)
}

// ipv6Tail closes every o34 IPv6 block. Replies and ICMPv6 errors (Packet Too
// Big etc.) for the states above match by state. Anything unsolicited from a
// global source on the uplink is dropped and logged: this is the pflog
// evidence for the external-vantage probe. The inet6 default deny stays last.
func ipv6Tail() string {
	return `# Router-originated global IPv6 (clock sync, packages). Never sourced from ULA.
pass out quick on $up6_if inet6 from ($up6_if) to ! <nonglobal6> keep state
block drop in log quick on $up6_if inet6 from ! <nonglobal6>
block drop quick inet6 all
`
}

func ipv6Activation(s routerStage) string {
	stage := "uplink only: router SLAAC address and IPv6 default via the CPE, forwarding off, Wi-Fi unchanged"
	if s.name == ipv6NAT66ProfileName {
		stage = "uplink plus Wi-Fi ULA " + wifiULAPrefix + ", rad on " + s.wifi + " only, IPv6 forwarding, NAT66 to <" + nat66EgressTable + ">"
	}
	return fmt.Sprintf(`O34 CANDIDATE: %s (wiring of wifi-first: lab/camera igc0, Wi-Fi %s)
Stage: %s.
Design: IQ openbsdrouter-plan/IPV6-WIFI.md. Activate only through
router-rollback as a timed trial, uplink stage first, then NAT66 on top of it.
NAT66 requires triggerfish_nat66_egress installed; without it the egress table
stays empty and translated traffic is dropped (fails closed).
No inbound IPv6 permit exists on the uplink. Lab ULA stays local-only; wg0
IPv6, wg1 and wg2 are untouched. OpenWrt keeps RA/DHCPv6/NDP off.
`, s.name, s.wifi, stage)
}
