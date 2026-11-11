package endpointfiles

import (
	"fmt"
	"strings"

	. "github.com/snonux/gonf/api"
)

// Retirement removes the endpoint migration's host artifacts (task a74, the
// user's decision of 2026-11-11): the every-minute watchdog cron, the boot
// restore hooks of the retired trial helper, and the WireGuard endpoint
// rollback watchdog and boot hooks that wireguardmeshgenerator's endpoint
// tooling installed without a gonf owner (systemd units on r0-r2; rc.d
// hooks and an /etc/crontab line on f0-f3). The apply helper itself stays
// (InstallHelper).
//
// One-shot: once every host ran its *_endpoints_retire task, a later commit
// deletes this file and its registrations. It keeps the old helper's private
// state (/var/{db,lib}/gonf-endpoint-migration journals, the orphan
// /usr/local/sbin/gonf-endpoint-rollback and /var/{db,lib}/gonf-endpoint):
// deleting those was not part of the decision.
type Retirement struct {
	RequiresRoot
	Platform string
}

// Linux unit files: gonf-endpoint-boot from the retired Prepare task, the
// gonf-endpoint-rollback* units from wireguardmeshgenerator's endpoint
// tooling. On the r-nodes k3s and wg-quick@wg0 hold RequiredBy enablement
// links to the -runtime and -boot units.
var (
	linuxUnits = List("gonf-endpoint-boot.service", "gonf-endpoint-rollback.timer",
		"gonf-endpoint-rollback.service", "gonf-endpoint-rollback-runtime.service",
		"gonf-endpoint-rollback-boot.service")
	rcdHooks = List("/etc/rc.d/gonf_endpoint_migration", "/etc/rc.conf.d/gonf_endpoint_migration",
		"/etc/rc.d/gonf_endpoint_rollback", "/etc/rc.d/gonf_endpoint_runtime")
)

// rollbackWatchdogLine is the FreeBSD system-crontab counterpart of the
// r-nodes' gonf-endpoint-rollback.timer (f0-f3).
const rollbackWatchdogLine = "* * * * * root /usr/local/bin/python3.12 /usr/local/sbin/gonf-endpoint-rollback watchdog /var/db/gonf-endpoint/pending"

// openBSDRcBlock is the exact block install-openbsd-boot inserted into /etc/rc.
const openBSDRcBlock = "# BEGIN gonf endpoint files\n/usr/local/sbin/gonf-endpoint-migration boot || exit 1\n# END gonf endpoint files\n"

// Retire removes the endpoint watchdog cron, boot hooks and orphan rollback units.
func (r Retirement) Retire() {
	NoCron("gonf-endpoint-migration")
	switch r.Platform {
	case "linux":
		retireSystemdUnits()
	case "freebsd", "netbsd":
		// The rollback/runtime scripts have no rcvar: they run at every boot
		// while present, so deleting the files is what disables them.
		NoFile(rcdHooks)
		if r.Platform == "freebsd" {
			File("/etc/crontab", WithoutLines(rollbackWatchdogLine), RootOwned)
		}
	case "openbsd":
		retireOpenBSDRc()
	default:
		Refuse("endpoint retirement", r.Platform, fmt.Errorf("unsupported platform"))
	}
}

// retireSystemdUnits disables the units before deleting them. Disable drops
// the k3s.service.requires and wg-quick@wg0.service.requires links, so after
// the one daemon-reload k3s and wg-quick never require a missing unit.
// Nothing is stopped except the timer: a stop job for a unit k3s Requires
// would stop k3s too.
func retireSystemdUnits() {
	present := `for u in "$@"; do [ -e "/etc/systemd/system/$u" ] && exit 0; done; exit 1`
	stopTimer := Command("systemctl", List("stop", "gonf-endpoint-rollback.timer"),
		OnlyIf("systemctl", List("is-active", "--quiet", "gonf-endpoint-rollback.timer")))
	disable := Command("sh", append(List("-c", `for u in "$@"; do [ ! -e "/etc/systemd/system/$u" ] || systemctl disable --quiet "$u"; done`, "disable"), linuxUnits...),
		OnlyIf("sh", append(List("-c", present, "present"), linuxUnits...)), DependsOn(stopTimer))
	var files []string
	for _, unit := range linuxUnits {
		files = append(files, "/etc/systemd/system/"+unit)
	}
	removed := NoFile(files, DependsOn(disable))
	Sh("systemctl daemon-reload", OnChange(disable, removed))
}

// retireOpenBSDRc deletes the exact three-line boot block from /etc/rc after
// a ksh syntax check of the result, and refuses a changed block.
func retireOpenBSDRc() {
	script := strings.Join([]string{
		`next=$(mktemp /etc/rc.gonf-endpoints.XXXXXXXX)`,
		`trap 'rm -f "$next"' 0`,
		`cp -p /etc/rc "$next"`,
		`[ "$(sed -n '/^# BEGIN gonf endpoint files$/,/^# END gonf endpoint files$/p' /etc/rc)" = "$1" ]`,
		`sed '/^# BEGIN gonf endpoint files$/,/^# END gonf endpoint files$/d' /etc/rc > "$next"`,
		`/bin/ksh -n "$next"`,
		// A bare "! grep" would not stop sh -e; refuse explicitly.
		`if grep -q 'gonf-endpoint-migration' "$next"; then exit 1; fi`,
		`mv "$next" /etc/rc`,
	}, "\n")
	Command("sh", List("-ec", script, "retire-rc", strings.TrimSuffix(openBSDRcBlock, "\n")),
		OnlyIf("grep", List("-q", "^# BEGIN gonf endpoint files$", "/etc/rc")))
}
