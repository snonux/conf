package netbsd

import (
	. "github.com/snonux/gonf/api"

	"codeberg.org/snonux/conf/gonf/paths"
)

// The daily vulnerability audit of pi0/pi1 (task 652,
// docs/archive/frontends/docs/unattended-upgrades-pi.plan.md §14): pkgsrc packages via
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

// VulnAuditTime is a Pi's daily audit time (host data, see gonf/cluster).
type VulnAuditTime struct {
	Minute string
	Hour   string
}

// DescVulnAuditPackages returns the description for the audit's tools.
func (Unattended) DescVulnAuditPackages() string {
	return "Install curl for netbsd-vuln-audit"
}

// VulnAuditPackages installs curl (pkgsrc), which the audit uses for its
// conditional HTTPS downloads. gzip, awk, pkg_admin and ntpq (the
// clock gate) are in base.
func (Unattended) VulnAuditPackages() {
	Package("curl")
}

// DescVulnAuditScript returns the description for the script deployment.
func (Unattended) DescVulnAuditScript() string {
	return "Install " + vulnAuditScript + " (0755 root:wheel)"
}

// OptsVulnAuditScript records curl before the script that needs it.
func (Unattended) OptsVulnAuditScript() TaskOptions {
	return TaskOptions{Needs("vuln_audit_packages")}
}

// VulnAuditScript installs the ksh audit script (after its directory, which
// gonf orders as the parent).
func (Unattended) VulnAuditScript() {
	EnsureDir("/usr/local/sbin", Perm(0o755, Root))
	InstallFile(vulnAuditScript,
		paths.FrontendAsset("scripts/netbsd-vuln-audit.sh"),
		Perm(0o755, Root))
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
	EnsureDir(vulnAuditStateDir, Perm(0o700, Root))
}

// DescVulnAuditCron returns the description for the daily cron job.
func (Unattended) DescVulnAuditCron() string {
	return "Root cron: netbsd-vuln-audit daily, per-host time"
}

// OptsVulnAuditCron records the script and its state directory before the
// job.
func (Unattended) OptsVulnAuditCron() TaskOptions {
	return TaskOptions{Needs("vuln_audit_script", "vuln_audit_state_dir")}
}

// VulnAuditCron runs the audit once a day at the host's VulnAuditTime,
// after the host's unattended pkgs/reboot window (so it audits the updated
// packages) and before /etc/daily at 04:15. The script also takes unattended-upgrade-netbsd's
// lock around pkg_admin audit, in case a window overruns. Only UNKNOWN
// (broken coverage, including an unsynchronised clock) exits non-zero; the
// script prints only err/warning lines, so cron's mail carries alerts only.
func (Unattended) VulnAuditCron() {
	EachHost(func(at VulnAuditTime) {
		Cron("netbsd-vuln-audit",
			WithCommand(vulnAuditScript),
			WithMinute(at.Minute), WithHour(at.Hour))
	})
}
