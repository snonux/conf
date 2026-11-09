package triggerfish

import (
	"errors"
	"fmt"
	"net/netip"
)

// c24: the lab gets its own router port once the human b24 cable move has
// put the ISP CPE directly on igc0 and the lab switch only on igc1. Plan and
// acceptance live in IQ topics/infra/openbsdrouter-plan (MIGRATION.md
// Stage B, CUTOVER-B24-C24.md). The profile is the live camera-igc3 profile
// (Wi-Fi NAT66 on igc2, p34 Shelly TCP443, n44 Syncthing, the camera zone on
// igc3) with 10.20.0.1/24, the lab ULA and their connected routes moved from
// igc0 to igc1, next to the retained recovery 10.98.0.1. igc0 keeps the
// upstream 192.168.1.2, the retained provisioning alias, the SLAAC uplink
// and the NAT66 egress address. The old post-rack/post-ap stages must not be
// used instead: they put Wi-Fi back on igc0 or drop NAT66.
const (
	labProfileName = "lab-igc1"
	// labPort is the lab switch port after b24.
	labPort = "igc1"
	// labOldManagement is lab-igc1's readiness value for the old management
	// path. After b24 only the ISP CPE shares igc0, so no host can make a
	// fresh old-LAN login to 192.168.1.2 any more; the address stays the
	// upstream, NAT and wg2 endpoint address.
	labOldManagement = "isp-uplink-only"
)

// labStages lists the c24 stage: lab igc1, Wi-Fi igc2, cameras igc3, all
// three zones on their final ports, recovery retained on igc1.
func labStages() []routerStage {
	return []routerStage{
		{labProfileName, labPort, "igc2", cameraPort, true, true},
	}
}

// labFiles renders c24 on top of the camera-igc3 files. The address and
// route move comes from the stage wiring (profileFiles; ipv6Files keeps the
// lab's local-only IPv6 policy on igc1 and adds the uplink SLAAC line to
// igc0), the lab DHCP on igc1 from dhcpInterfaces and profileDHCP, the
// explicit camera deny from cameraZoneStage. Only the notes differ here.
func labFiles(s routerStage, data RouterProfiles) map[string]string {
	files := cameraFiles(s, data)
	files["ACTIVATION.txt"] = labActivation(s)
	return files
}

// oldManagementReceipt is the old-management line of a profile's readiness
// receipt; router-rollback's old_management prints the same values.
func oldManagementReceipt(profile string) string {
	if profile == labProfileName {
		return labOldManagement
	}
	return "fresh-authenticated"
}

// validateLabProof requires lab-igc1's "new" and "recovery" receipts to come
// from a lab host: 10.20.0.0/24 to 10.20.0.1 and 10.98.0.0/24 to 10.98.0.1.
// PF admits both source prefixes only on igc1, so these sessions prove the
// moved lab port itself; the laptop's wg2 sessions to those addresses never
// touch igc1. router-rollback's proof checks the same.
func validateLabProof(t RouterTrial, client netip.Addr) error {
	if t.Profile != labProfileName {
		return nil
	}
	prefix, server := map[string]string{
		"new": "10.20.0.0/24", "recovery": "10.98.0.0/24",
	}[t.ProofRole], map[string]string{
		"new": "10.20.0.1", "recovery": "10.98.0.1",
	}[t.ProofRole]
	if prefix == "" {
		return nil
	}
	if !netip.MustParsePrefix(prefix).Contains(client) || client == netip.MustParseAddr(server) {
		return errors.New("lab-igc1 new and recovery receipts must come from a lab host")
	}
	return nil
}

// oldManagementComment heads the old-address management rules. After b24
// (lab-igc1) only the ISP router shares igc0, so only it could use them; every
// other stage keeps its byte-identical wording.
func oldManagementComment(s routerStage) string {
	if s.name == labProfileName {
		return "# Old-address router management: after b24 only the ISP router shares igc0."
	}
	return "# Preserve fresh old-address router management from the existing LAN."
}

// labRecoveryProofRules lets a lab host's recovery address reach the router's
// igc1 recovery address on SSH, so lab-igc1's recovery receipt crosses igc1
// (the laptop's wg2 path to 10.98.0.1 does not). The same hosts already reach
// 10.20.0.1:22; anti-spoofing admits 10.98.0.0/24 only on igc1.
func labRecoveryProofRules() string {
	return "# c24: host-sourced recovery receipt over igc1.\n" +
		"pass in on igc1 inet proto tcp from 10.98.0.0/24 to 10.98.0.1 port 22 keep state\n"
}

func labActivation(s routerStage) string {
	return fmt.Sprintf(`C24 CANDIDATE: %s (camera-igc3 plus the lab on its own port)
Wiring after the human b24 cable move ONLY: ISP CPE directly on igc0, lab
switch on %s (lab gear only), Wi-Fi AP on %s, cameras %s (uncabled).
Staging is fine at any time; do NOT ACTIVATE before the user confirms b24:
the user set that order, the readiness receipt records the b24 cabling and
old-management=isp-uplink-only, and before b24 dhcpd on igc1 would share
the wire with the ISP router's DHCP.
Moves 10.20.0.1/24, fd02:ca0b:f1cb:20::1/64 and their connected routes from
igc0 to %s, never on both; recovery 10.98.0.1 stays on igc1. igc0 keeps
192.168.1.2 (upstream, NAT, wg2 endpoint, k34 source), the retained
provisioning alias, SLAAC and the NAT66 egress address.
%s with the JetKVM MAC reservations.
Lab IPv4 to the internet via NAT on igc0; the lab ULA stays local-only.
Wi-Fi NAT66, Shelly TCP443 pairs, n44 Syncthing, wg2 and the camera zone are
the camera-igc3 policy. Confirmation needs fresh new and recovery receipts
from a lab host (10.20.0.x to 10.20.0.1, 10.98.0.x to 10.98.0.1, which PF
admits only on igc1) and the VPN receipt; no old-LAN host can reach
192.168.1.2 after b24.
A timed or boot restore returns camera-igc3: the lab then has no gateway
until a fixed retry or the user's separate reverse cable action; recovery
stays on igc1 and wg2 on the router. No boot canary (it lives on igc0).
Activate only through router-rollback as a timed trial.
`, s.name, s.lab, s.wifi, s.camera, s.lab, "DHCP: "+dhcpState(s))
}
