package triggerfish

import (
	"fmt"
	"strings"
)

// o34 Wi-Fi internet IPv6 candidate. Design and acceptance criteria live in
// IQ topics/infra/openbsdrouter-plan/IPV6-WIFI.md. The profile is rendered only
// when RouterProfiles.WiFiIPv6Candidate is set (never by the inventory today)
// and the rollback helper/trial input do not list it, so it cannot be staged or
// activated. It assumes an ISP-side DHCPv6-PD server that has NOT been proved.
const (
	wifiIPv6ProfileName = "wifi-ipv6-candidate"
	// ipv6Uplink is the ISP-facing port in every wiring stage.
	ipv6Uplink = "igc0"
	// The current local-only line is replaced on the uplink: slaacd must learn
	// the ISP default route there. Only the "inet6 autoconf ..." form is passed
	// verbatim by netstart(8); "inet6 -autoconf -temporary" is not.
	localOnlyIPv6Line = "inet6 -autoconf -temporary\n"
	uplinkIPv6Line    = "inet6 autoconf -temporary\n"
)

// wifiIPv6Stage is the current verified wiring (wifi-first): lab and cameras
// still share igc0 with the ISP LAN, Wi-Fi is on igc2.
func wifiIPv6Stage() routerStage {
	return routerStage{wifiIPv6ProfileName, "igc0", "igc2", "igc0", true, true}
}

// wifiIPv6Files derives the candidate from the stage's IPv4/ULA files and
// changes only what IPv6 internet for Wi-Fi needs: uplink SLAAC, a Wi-Fi
// link-local address, PD client, RA server, IPv6 forwarding and PF.
func wifiIPv6Files(s routerStage, data RouterProfiles) map[string]string {
	files := profileFiles(s, data)
	uplink := "hostname." + ipv6Uplink
	if strings.Contains(files[uplink], localOnlyIPv6Line) {
		files[uplink] = strings.Replace(files[uplink], localOnlyIPv6Line, uplinkIPv6Line, 1)
	} else {
		files[uplink] += uplinkIPv6Line
	}
	// Link-local before rad(8)/dhcp6leased(8) start; the GUA comes from the lease.
	files["hostname."+s.wifi] += "inet6 eui64\n"
	files["sysctl.conf.fragment"] = "net.inet.ip.forwarding=1\nnet.inet6.ip6.forwarding=1\n"
	files["dhcp6leased.conf"] = wifiIPv6DHCP6(s)
	files["rad.conf"] = wifiIPv6RAD(s)
	files["services.conf"] = strings.Replace(profileServices(s), "rad=disabled\n",
		"rad=enabled\ndhcp6leased=enabled\nslaacd=enabled\n", 1)
	files["pf.conf"] = profilePFWithIPv6(s, data, wifiIPv6Rules())
	files["ACTIVATION.txt"] = wifiIPv6Activation(s)
	return files
}

func wifiIPv6DHCP6(s routerStage) string {
	return fmt.Sprintf(`# Inactive o34 candidate. One delegated prefix, requested on the uplink;
# its first /64 (router ::1) goes to Wi-Fi only. Lab stays ULA-only and
# cameras IPv4-only, so no other interface is listed. Append new interfaces
# at the end (dhcp6leased.conf(5)) so Wi-Fi is never renumbered by an edit.
request prefix delegation on %s for { %s/64 }
`, ipv6Uplink, s.wifi)
}

func wifiIPv6RAD(s routerStage) string {
	return fmt.Sprintf(`# Inactive o34 candidate. Advertise ONLY on Wi-Fi: never on %s (ISP LAN and
# shared lab wire) or the camera port. The prefix is discovered from the
# delegated %s address, so rad.conf(5) caps lifetimes at the lease's.
# No RDNSS: dual-stack clients keep DHCPv4 DNS %s, which returns AAAA.
interface %s
`, ipv6Uplink, s.wifi, "10.30.0.1", s.wifi)
}

// wifiIPv6Rules replaces the local-only IPv6 block. Order matters: link-local
// control traffic first, then zone/spoof blocks, then the only transit permit,
// then a logged deny of unsolicited inbound and the inet6 default deny.
// Interface-derived addresses are parenthesised so a renumbered delegation
// needs no ruleset reload (pf.conf(5)).
func wifiIPv6Rules() string {
	return labIPv6Rules() + fmt.Sprintf(`# o34 candidate: Wi-Fi internet IPv6 from a DHCPv6-PD prefix; default deny stays.
up6_if = "%s"
table <nonglobal6> const { ::/8, 100::/64, 2001:db8::/32, 2002::/16, 3fff::/20, fc00::/7, fe80::/10, fec0::/10, ff00::/8 }
# ULA (lab included) and other non-global sources never leave towards the internet.
block drop out log quick on $up6_if inet6 from <nonglobal6> to ! <nonglobal6>
pass quick on { $up6_if $wifi_if } inet6 proto icmp6 icmp6-type { neighbrsol, neighbradv }
# MLD query/report/done and MLDv2 report keep solicited-node groups working.
pass quick on { $up6_if $wifi_if } inet6 proto icmp6 from { fe80::/10, :: } to ff02::/16 icmp6-type { 130, 131, 132, 143 }
pass in quick on $up6_if inet6 proto icmp6 from fe80::/10 to { ff02::1, fe80::/10 } icmp6-type routeradv
pass out quick on $up6_if inet6 proto icmp6 from { fe80::/10, :: } to ff02::2 icmp6-type routersol
pass in quick on $wifi_if inet6 proto icmp6 from { fe80::/10, :: } to ff02::2 icmp6-type routersol
pass out quick on $wifi_if inet6 proto icmp6 from fe80::/10 to { ff02::1, fe80::/10 } icmp6-type routeradv
# Only the ISP router advertises to triggerfish; no redirects are honoured.
block drop log quick inet6 proto icmp6 icmp6-type { routeradv, redir }
# DHCPv6-PD client on the uplink only.
pass out quick on $up6_if inet6 proto udp from fe80::/10 port 546 to { ff02::1:2, fe80::/10 } port 547
pass in quick on $up6_if inet6 proto udp from fe80::/10 port 547 to fe80::/10 port 546
# Anti-spoofing in both directions for the delegated Wi-Fi prefix.
block drop in log quick on $wifi_if inet6 from ! ($wifi_if:network)
block drop in log quick on ! $wifi_if inet6 from ($wifi_if:network)
# Diagnostics: Wi-Fi may ping its own gateway address, nothing else on the router.
pass in quick on $wifi_if inet6 proto icmp6 from ($wifi_if:network) to ($wifi_if) icmp6-type echoreq keep state
# Wi-Fi gets internet only: not the router, the ISP LAN, the lab ULA or any
# other non-global destination.
block drop in log quick on $wifi_if inet6 to { (self), ($up6_if:network), <nonglobal6> }
pass in quick on $wifi_if inet6 from ($wifi_if:network) to ! <nonglobal6> tag gonf_transit6 keep state
pass out quick on $up6_if inet6 from ($wifi_if:network) to ! <nonglobal6> tagged gonf_transit6 keep state
# Router-originated global IPv6 (clock sync, packages). Never sourced from ULA.
pass out quick on $up6_if inet6 from ($up6_if) to ! <nonglobal6> keep state
# Replies and ICMPv6 errors (Packet Too Big etc.) for the states above are
# matched by state. Anything unsolicited from upstream is dropped and logged:
# this is the pflog evidence for the external-vantage probe.
block drop in log quick on $up6_if inet6 to { ($wifi_if:network), (self) }
block drop quick inet6 all
`, ipv6Uplink)
}

func wifiIPv6Activation(s routerStage) string {
	return fmt.Sprintf(`INACTIVE o34 CANDIDATE: %s (wiring of wifi-first: lab/camera igc0, Wi-Fi %s)
NOT STAGEABLE: router-rollback, RouterTrial and GONF_ROUTER_TRIAL refuse this
profile name on purpose. Design: IQ openbsdrouter-plan/IPV6-WIFI.md.
Prerequisites before any activation work:
- A delegated (DHCPv6-PD) or routed prefix for Wi-Fi has been PROVED. An RA
  /64 on the ISP LAN is not one. Without it this candidate must not be used.
- f24 IPv4-only Wi-Fi acceptance is complete.
- router-rollback extended and fixture-tested for: ip6.forwarding=1, uplink
  autoconf, delegated igc2 address, dhcp6leased lease/uuid state, rad, and
  withdrawal of learned default routes/addresses on restore and boot.
- Native checks: pfctl -nf, rad -n -f, dhcp6leased -n -vv -f.
- OpenWrt AP keeps RA/DHCPv6/NDP off and bridges IPv6 transparently.
- Independent external IPv6 vantage for the negative inbound probe.
Lab ULA stays local-only; wg0 IPv6, wg1 and wg2 are untouched.
`, s.name, s.wifi)
}
