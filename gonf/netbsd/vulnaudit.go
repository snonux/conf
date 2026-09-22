package netbsd

import (
	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"

	"codeberg.org/snonux/conf/gonf/cluster"
	"codeberg.org/snonux/conf/gonf/paths"
)

// The daily vulnerability audit of pi0/pi1 (task 652,
// frontends/docs/unattended-upgrades-pi.plan.md §14): pkgsrc packages via
// the pkgsrc-security pkg-vulnerabilities list and `pkg_admin audit`, the
// base system via the NetBSD supported-release list and security
// advisories. It is a separate script and cron job so a feed outage never
// blocks unattended-upgrade-netbsd; it remediates nothing (base updates
// stay manual release upgrades, plan §3).
//
// The tasks are methods of Unattended, named VulnAudit*, so they register
// as pis_netbsd_vuln_audit_* through the existing pis_netbsd_ registration
// and aggregate without a separate RegisterMethods line in gonf/tasks.
const (
	vulnAuditScript   = "/usr/local/sbin/netbsd-vuln-audit"
	vulnAuditStateDir = "/var/db/netbsd-vuln-audit"
)

// DescVulnAuditPackages returns the description for the audit's tools.
func (Unattended) DescVulnAuditPackages() string {
	return "Install curl for netbsd-vuln-audit"
}

// VulnAuditPackages installs curl (pkgsrc), which the audit uses for its
// conditional HTTPS downloads. gzip, awk and pkg_admin are in base.
func (Unattended) VulnAuditPackages() {
	WhenHostname(ClusterHosts(), func() {
		Package("curl")
	})
}

// DescVulnAuditScript returns the description for the script deployment.
func (Unattended) DescVulnAuditScript() string {
	return "Install " + vulnAuditScript + " (0755 root:wheel)"
}

// VulnAuditScript installs the ksh audit script.
func (Unattended) VulnAuditScript() {
	WhenHostname(ClusterHosts(), func() {
		dir := EnsureDir("/usr/local/sbin",
			WithMode(0o755), WithOwner("root"), WithGroup("wheel"))
		InstallFile(vulnAuditScript,
			paths.Frontends+"/scripts/netbsd-vuln-audit.sh",
			WithMode(0o755), WithOwner("root"), WithGroup("wheel"),
			DependsOn(dir))
	})
}

// DescVulnAuditStateDir returns the description for the state directory.
func (Unattended) DescVulnAuditStateDir() string {
	return "Ensure " + vulnAuditStateDir + " (caches, baseline, status)"
}

// VulnAuditStateDir creates the directory holding the run lock, the
// download caches (pkg-vulnerabilities.gz, releases.html, advisories/), the
// per-source contact stamps, the status record, the findings baseline, the
// new/resolved batches with the 90-day new-finding history and the raw
// pkg_admin audit output. The verified pkg-vulnerabilities list itself is
// installed in PKGVULNDIR (/usr/pkg/pkgdb), where interactive
// `pkg_admin audit` and the daily /etc/security check read it.
func (Unattended) VulnAuditStateDir() {
	WhenHostname(ClusterHosts(), func() {
		EnsureDir(vulnAuditStateDir,
			WithMode(0o700), WithOwner("root"), WithGroup("wheel"))
	})
}

// DescVulnAuditCron returns the description for the daily cron job.
func (Unattended) DescVulnAuditCron() string {
	return "Root cron: netbsd-vuln-audit daily, per-host time"
}

// VulnAuditCron runs the audit once a day at the host's
// cluster.ValueVulnAuditCron {minute, hour}, after the host's unattended
// pkgs/reboot window (so it audits the updated packages) and before
// /etc/daily at 04:15. Only UNKNOWN (broken coverage) exits non-zero; the
// script prints only err/warning lines, so cron's mail carries alerts only.
func (Unattended) VulnAuditCron() {
	ForHosts(cluster.ValueVulnAuditCron, func(_ string, at [2]string) {
		Cron("netbsd-vuln-audit",
			WithCommand(vulnAuditScript),
			WithMinute(at[0]), WithHour(at[1]))
	})
}
