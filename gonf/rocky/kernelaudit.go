package rocky

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// KernelAudit deploys the daily Raspberry Pi kernel CVE audit (task 752) to
// the Rocky Pis. pi2/pi3 boot the SIG AltArch raspberrypi2-kernel4 package,
// which Rocky's updateinfo does not cover, so the audit maps the running
// kernel's upstream version against the OSV Linux-kernel CVE export instead
// (docs/archive/frontends/docs/unattended-upgrades-pi.plan.md §13). It is separate from
// Unattended so the regular dnf update/audit path stays untouched, and it is
// registered with OnCluster(cluster.NameRockyPis): the r-nodes run the
// ordinary Rocky kernel, which dnf updateinfo does cover.
type KernelAudit struct {
	RequiresRoot
}

// KernelAuditCalendar is a Rocky Pi's daily audit schedule (host data, see
// gonf/cluster), a systemd OnCalendar= expression.
type KernelAuditCalendar struct {
	OnCalendar string
}

// DescPackages returns the description for the audit's tool dependencies.
func (KernelAudit) DescPackages() string {
	return "Install ksh, unzip, jq and curl for rocky-kernel-audit"
}

// Packages installs the script's interpreter and feed tooling: ksh runs it
// (a fresh host without rocky_packages would otherwise fail 203/EXEC), curl
// downloads the OSV export, unzip streams its records, jq evaluates the
// version ranges. ksh is also declared by Unattended.Packages; the duplicate
// declaration is idempotent.
func (KernelAudit) Packages() {
	Packages("ksh", "unzip", "jq", "curl")
}

// DescScript returns the description for the audit script deployment.
func (KernelAudit) DescScript() string {
	return "Install /usr/local/sbin/rocky-kernel-audit (0755 root:root)"
}

// OptsScript records the audit's tools before the script.
func (KernelAudit) OptsScript() TaskOptions {
	return TaskOptions{Needs("packages")}
}

// Script installs the ksh audit script (after its directory, which gonf
// orders as the parent).
func (KernelAudit) Script() {
	EnsureDir("/usr/local/sbin", Perm(0o755, Root))
	InstallFile("/usr/local/sbin/rocky-kernel-audit",
		paths.FrontendAsset("scripts/rocky-kernel-audit.sh"),
		Perm(0o755, Root))
}

// DescStateDir returns the description for the audit state directory.
func (KernelAudit) DescStateDir() string {
	return "Ensure /var/lib/rocky-kernel-audit (feed cache + status)"
}

// StateDir creates the directory holding the run lock, the cached feed, the
// status record, the affected-CVE baseline (affected-cves) with its record
// count (baseline-cve-records), a pending record-count drop
// (drop-candidate, accepted after 3 consecutive runs), the new-CVE list plus
// its dated 90-day history (one line per CVE, first alert date) and the
// removed-CVE list. Operator overrides: deleting
// baseline-cve-records accepts a lower feed record count at once; deleting
// affected-cves re-baselines (every affected CVE is reported as new once).
func (KernelAudit) StateDir() {
	EnsureDir("/var/lib/rocky-kernel-audit", Perm(0o700, Root))
}

// DescUnits returns the description for the audit timer.
func (KernelAudit) DescUnits() string {
	return "Install rocky-kernel-audit SystemdTimer (daily, per-host calendar)"
}

// OptsUnits records the script and the state directory before the timer.
func (KernelAudit) OptsUnits() TaskOptions {
	return TaskOptions{Needs("script", "state_dir")}
}

// Units installs the daily oneshot+timer pair. Persistent catches a run
// missed while the Pi was down. Only UNKNOWN (broken coverage) exits
// non-zero, so a failed service means the audit itself needs attention;
// VULNERABLE is reported in the journal at notice/warning priority (new
// CVEs, status changes) and in the status record. The Pis have no MTA.
func (KernelAudit) Units() {
	EachHost(func(c KernelAuditCalendar) {
		SystemdTimer("rocky-kernel-audit",
			WithCommand("/usr/local/sbin/rocky-kernel-audit"),
			WithOnCalendar(c.OnCalendar),
			WithPersistent,
			WithDescription("Daily Raspberry Pi kernel CVE audit"),
			WithServiceDescription("Raspberry Pi kernel CVE audit (OSV Linux feed)"),
			WithAfter("network-online.target"),
			WithWants("network-online.target"),
		)
	})
}
