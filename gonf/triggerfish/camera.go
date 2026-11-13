package triggerfish

import (
	"fmt"
	"strings"
)

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
// c24's labFiles (lab.go) starts from these files as well.
func cameraFiles(s routerStage, data RouterProfiles) map[string]string {
	files := ipv6Files(s, data)
	files["ACTIVATION.txt"] = cameraActivation(s)
	return files
}

// cameraZoneStage names the profiles that render the explicit camera-zone
// deny: camera-igc3 and the lab-port profiles built on it (c24's lab-igc1,
// j24's lab-final). router-rollback's camera_zone_profile lists the same
// names.
func cameraZoneStage(name string) bool {
	return name == cameraProfileName || labPortStage(name)
}

// cameraZoneRules denies the camera port in both directions, logged. The
// catch-all "block return log all" already denies it; the quick rules keep
// the zone closed against non-quick permits without an interface, such as
// "pass out inet tagged gonf_transit", and give pflog a distinct rule. A
// later measured permit therefore has to be a quick rule placed before these
// lines (or replace them), with an if-bound state like every lab-final rule
// (l24, ifBoundStates). <outdoor> is the static allocation (no DHCP);
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

// ifBoundStage names the profiles whose PF states are bound to the interface
// they were created on (l24): j24's lab-final, the profile that stays live
// when the cameras are cabled. The older profiles keep their accepted bytes.
// router-rollback's ifbound_profile lists the same names.
func ifBoundStage(name string) bool { return name == labFinalProfileName }

// ifBoundStates gives every pass rule of a rendered pf.conf an if-bound
// state (g24 review N2). With "set state-policy floating" PF looks a packet
// up in the state table before evaluating any rule, so a device on the
// camera port could inject packets matching a state created on another port,
// such as the replies to the router's own DNS and NTP queries on igc0 or a
// lab flow, and the quick camera deny would never see them. An if-bound
// state only matches packets on its own interface; every routed flow already
// has a pass rule on each side (the gonf_transit tag), so nothing that works
// today needs a floating state. The global policy stays floating on purpose:
// router-rollback's arm snapshot restores "set state-policy floating" plus
// the loaded rules (pfctl -sr), and the per-rule option travels with them.
// "keep state" gains the option; a pass without a state keyword gets an
// explicit "keep state (if-bound)" (PF's default is keep state). Rules with
// state options or "no state" would need their own handling: none exist, and
// router-rollback's if_bound_states refuses any stateful pass without it.
func ifBoundStates(pf string) string {
	lines := strings.Split(pf, "\n")
	for i, line := range lines {
		switch {
		case !strings.HasPrefix(line, "pass "):
		case strings.HasSuffix(line, " keep state"):
			lines[i] = line + " (if-bound)"
		case !strings.Contains(line, " state"):
			lines[i] = line + " keep state (if-bound)"
		}
	}
	pf = strings.Join(lines, "\n")
	return strings.Replace(pf, "set state-policy floating\n", "set state-policy floating\n"+
		"# l24: every pass rule keeps if-bound states, so a packet arriving on another\n"+
		"# port (a device on the camera port) never matches them; see ifBoundStates.\n", 1)
}
