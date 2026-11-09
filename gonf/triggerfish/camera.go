package triggerfish

import "fmt"

// g24: the outdoor camera zone gets its own router port before any camera
// exists. Plan and acceptance live in IQ topics/infra/openbsdrouter-plan
// (PLAN.md §8 and §13, MIGRATION.md Stage C). The profile is the live
// wifi-ipv6-nat66 profile (Wi-Fi NAT66 and the p34 Shelly TCP443 permits
// included) with 10.40.0.1/24 and its connected route moved from the shared
// igc0 to igc3, which stays uncabled until the human camera gate (m24).
// Measured recorder access and temporary update permits are later tasks
// (l24/n24); this profile opens neither.
const (
	cameraProfileName = "camera-igc3"
	// cameraPort is the dedicated, still uncabled outdoor camera port.
	cameraPort = "igc3"
)

// cameraStages lists the g24 stage: the wifi-ipv6-nat66 wiring (lab and the
// ISP LAN still share igc0, Wi-Fi on igc2, recovery on igc1) with the camera
// zone on its own port.
func cameraStages() []routerStage {
	return []routerStage{
		{cameraProfileName, "igc0", "igc2", cameraPort, true, true},
	}
}

// cameraFiles renders the g24 stage on top of the NAT66 files. The address
// move comes from the stage wiring (profileFiles), the explicit zone deny from
// profilePFWithIPv6 (cameraZoneStage); only the activation notes differ here.
func cameraFiles(s routerStage, data RouterProfiles) map[string]string {
	files := ipv6Files(s, data)
	files["ACTIVATION.txt"] = cameraActivation(s)
	return files
}

// cameraZoneStage names the profiles that render the explicit camera-zone
// deny. post-ap and final also put the cameras on igc3, but they stay
// byte-identical until their own stage revisits the camera policy.
func cameraZoneStage(name string) bool {
	return name == cameraProfileName
}

// cameraZoneRules denies the camera port in both directions, logged. The
// catch-all "block return log all" already denies it; the quick rules keep
// the zone closed against non-quick permits without an interface, such as
// "pass out inet tagged gonf_transit", and give pflog a distinct rule. A
// later measured permit therefore has to be a quick rule placed before these
// lines (or replace them). <outdoor> is the static allocation (no DHCP);
// <recorders> and <camera_updates> stay empty and unreferenced, so no NAT,
// internet, router service, lab or Wi-Fi path reaches the zone.
func cameraZoneRules() string {
	return `# g24 camera zone on $cam_if (uncabled): default deny both ways, logged.
# Static allocation <outdoor>, no DHCP, NAT, internet, router service or IPv6.
# Recorder and update permits belong to the later measured camera gate.
block drop in log quick on $cam_if
block drop out log quick on $cam_if
`
}

func cameraActivation(s routerStage) string {
	return fmt.Sprintf(`G24 CANDIDATE: %s (wifi-ipv6-nat66 plus the camera zone on its own port)
Wiring: lab %s (still shared with the ISP LAN), Wi-Fi %s, recovery igc1,
cameras %s, UNCABLED until the separate human camera gate.
Moves 10.40.0.1/24 and its connected route from igc0 to %s, never on both.
Camera zone: static allocation <outdoor> 10.40.0.11-14 without DHCP, explicit
logged default deny in both directions, no NAT, internet, router service or
IPv6 (activation removes any IPv6 from %s). <recorders> and <camera_updates>
stay empty; measured recorder access and temporary update permits are later
tasks. Everything else is the wifi-ipv6-nat66 profile (Wi-Fi NAT66, Shelly
TCP443). Activate only through router-rollback as a timed trial.
Untested until cameras exist: real camera traffic and its pflog drops.
`, s.name, s.lab, s.wifi, s.camera, s.camera, s.camera)
}
