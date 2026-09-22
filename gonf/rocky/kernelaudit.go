package rocky

import (
	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"

	"codeberg.org/snonux/conf/gonf/cluster"
	"codeberg.org/snonux/conf/gonf/paths"
)

// KernelAudit deploys the daily Raspberry Pi kernel CVE audit (task 752) to
// the Rocky Pis. pi2/pi3 boot the SIG AltArch raspberrypi2-kernel4 package,
// which Rocky's updateinfo does not cover, so the audit maps the running
// kernel's upstream version against the OSV Linux-kernel CVE export instead
// (frontends/docs/unattended-upgrades-pi.plan.md §13). It is separate from
// Unattended so the regular dnf update/audit path stays untouched, and it is
// registered with WithCluster(cluster.NameRockyPis): the r-nodes run the
// ordinary Rocky kernel, which dnf updateinfo does cover.
type KernelAudit struct {
	RequiresRoot
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
	WhenHostname(ClusterHosts(), func() {
		Package("ksh")
		Package("unzip")
		Package("jq")
		Package("curl")
	})
}

// DescScript returns the description for the audit script deployment.
func (KernelAudit) DescScript() string {
	return "Install /usr/local/sbin/rocky-kernel-audit (0755 root:root)"
}

// Script installs the ksh audit script.
func (KernelAudit) Script() {
	WhenHostname(ClusterHosts(), func() {
		dir := EnsureDir("/usr/local/sbin",
			WithMode(0o755), WithOwner("root"), WithGroup("root"))
		InstallFile("/usr/local/sbin/rocky-kernel-audit",
			paths.Frontends+"/scripts/rocky-kernel-audit.sh",
			WithMode(0o755), WithOwner("root"), WithGroup("root"),
			DependsOn(dir))
	})
}

// DescStateDir returns the description for the audit state directory.
func (KernelAudit) DescStateDir() string {
	return "Ensure /var/lib/rocky-kernel-audit (feed cache + status)"
}

// StateDir creates the directory holding the run lock, the cached feed, the
// status record, the affected-CVE baseline (with its record count) and the
// new-CVE list plus its dated 90-day history.
func (KernelAudit) StateDir() {
	WhenHostname(ClusterHosts(), func() {
		EnsureDir("/var/lib/rocky-kernel-audit",
			WithMode(0o700), WithOwner("root"), WithGroup("root"))
	})
}

// DescUnits returns the description for the audit timer.
func (KernelAudit) DescUnits() string {
	return "Install rocky-kernel-audit SystemdTimer (daily, per-host calendar)"
}

// Units installs the daily oneshot+timer pair. Persistent catches a run
// missed while the Pi was down. Only UNKNOWN (broken coverage) exits
// non-zero, so a failed service means the audit itself needs attention;
// VULNERABLE is reported in the journal at notice/warning priority (new
// CVEs, status changes) and in the status record. The Pis have no MTA.
func (KernelAudit) Units() {
	for _, host := range ClusterHosts() {
		calendar := MustHostValue[string](host, cluster.ValueKernelAuditOnCalendar)
		WhenHostname(host, func() {
			SystemdTimer("rocky-kernel-audit",
				WithCommand("/usr/local/sbin/rocky-kernel-audit"),
				WithOnCalendar(calendar),
				WithPersistent,
				WithDescription("Daily Raspberry Pi kernel CVE audit"),
				WithServiceDescription("Raspberry Pi kernel CVE audit (OSV Linux feed)"),
				WithAfter("network-online.target"),
				WithWants("network-online.target"),
			)
		})
	}
}
