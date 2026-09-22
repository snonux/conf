package debian

import (
	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"

	"codeberg.org/snonux/conf/gonf/cluster"
	"codeberg.org/snonux/conf/gonf/paths"
)

// aptUnattendedConf is /etc/apt/apt.conf.d/52unattended-upgrade-gonf. It
// sorts after Debian's 20auto-upgrades and 50unattended-upgrades, so its
// values win; #clear drops the stock origin list instead of merging into it.
// The origins are Debian security, Debian point releases (stable updates,
// Debian's own default) and Docker CE, the Debian counterpart of the full
// "dnf upgrade" on Rocky. apt's periodic jobs stay idle: the
// unattended-upgrade-debian timer is the only driver, so the partner gate
// and the per-host schedule cannot be bypassed by apt-daily-upgrade.timer.
// Rebooting is the wrapper's job (partner-gated), never unattended-upgrade's.
const aptUnattendedConf = `// Managed by gonf (conf: gonf/debian/unattended.go). Do not edit here.
// /usr/local/sbin/unattended-upgrade-debian (its systemd timer) runs
// apt-get update and unattended-upgrade; apt's own periodic jobs stay idle.
APT::Periodic::Update-Package-Lists "0";
APT::Periodic::Download-Upgradeable-Packages "0";
APT::Periodic::Unattended-Upgrade "0";
APT::Periodic::AutocleanInterval "7";

#clear Unattended-Upgrade::Origins-Pattern;
Unattended-Upgrade::Origins-Pattern {
	"origin=Debian,codename=${distro_codename}-security,label=Debian-Security";
	"origin=Debian,codename=${distro_codename},label=Debian";
	"origin=Docker,label=Docker CE";
};
Unattended-Upgrade::Automatic-Reboot "false";
Unattended-Upgrade::OnlyOnACPower "false";
Unattended-Upgrade::Skip-Updates-On-Metered-Connections "false";
Unattended-Upgrade::Remove-Unused-Kernel-Packages "true";
`

// needrestartConf keeps needrestart's automatic restart pass (run by the
// wrapper after unattended-upgrade) away from the wrapper's own running
// oneshot unit, which it would otherwise SIGTERM before the day is stamped.
// needrestart's stock override_rc (dbus, systemd-logind, docker, gettys, ...)
// stays in force; the wrapper reboots for what those leave behind.
const needrestartConf = `# Managed by gonf (conf: gonf/debian/unattended.go). Do not edit here.
# Never restart the running unattended-upgrade-debian oneshot.
$nrconf{override_rc}{qr(^unattended-upgrade-debian\.service$)} = 0;
`

// Unattended carries the unattended-upgrade deployment for the Debian Pis
// (frontends/scripts/unattended-upgrade-debian.sh, the Debian counterpart of
// rocky.Unattended). Register with WithCluster(cluster.NameDebianPis).
// Per-host OnCalendar lives on each Host via
// WithValue(cluster.ValueUnattendedOnCalendar, …), the same key and values
// as on Rocky, so the pi2/pi3 offsets stay as they are.
//
// There is no GonfLink task as on Rocky: Debian's sudo secure_path already
// contains /usr/local/bin, where the bootstrap installs gonf.
type Unattended struct {
	RequiresRoot
}

// DescPackages returns the description for required packages.
func (Unattended) DescPackages() string {
	return "Install unattended-upgrades, needrestart and logrotate for unattended-upgrade-debian"
}

// Packages installs unattended-upgrades (the apt-origin-aware upgrader, and
// the kernel postinst hook writing /run/reboot-required), needrestart (the
// service restarts and the kernel/library reboot signals) and logrotate.
func (Unattended) Packages() {
	onDebian(func() { aptPackages(List("unattended-upgrades", "needrestart", "logrotate")) })
}

// DescAptConfig returns the description for the apt and needrestart config.
func (Unattended) DescAptConfig() string {
	return "Install the apt unattended-upgrade origins and the needrestart override"
}

// AptConfig installs the unattended-upgrade origins/periodic settings and
// the needrestart override, each validated before the live file changes:
// apt-config parses the candidate, perl compiles the needrestart snippet.
func (Unattended) AptConfig() {
	onDebian(func() {
		pkgs := aptPackages(List("unattended-upgrades", "needrestart", "logrotate"))
		File("/etc/apt/apt.conf.d/52unattended-upgrade-gonf",
			WithContent(aptUnattendedConf),
			WithValidation("apt-config", List("-c", CandidatePath, "dump")),
			WithMode(0o644), WithOwner("root"), WithGroup("root"))
		dir := rootDir("/etc/needrestart/conf.d")
		File("/etc/needrestart/conf.d/50-unattended-upgrade-gonf.conf",
			WithContent(needrestartConf),
			WithValidation("perl", List("-c", CandidatePath)),
			WithMode(0o644), WithOwner("root"), WithGroup("root"),
			DependsOn(pkgs, dir))
	})
}

// DescScript returns the description for the wrapper deployment.
func (Unattended) DescScript() string {
	return "Install /usr/local/sbin/unattended-upgrade-debian (0755 root:root; needs debian_pis_packages)"
}

// Script installs the bash wrapper. The script itself refuses to run unless
// /etc/os-release says ID=debian (a second, run-time OS guard).
func (Unattended) Script() {
	onDebian(func() {
		dir := rootDir("/usr/local/sbin")
		InstallFile("/usr/local/sbin/unattended-upgrade-debian",
			paths.Frontends+"/scripts/unattended-upgrade-debian.sh",
			WithMode(0o755), WithOwner("root"), WithGroup("root"),
			DependsOn(dir))
	})
}

// DescStampDir returns the description for the stamp directory.
func (Unattended) DescStampDir() string {
	return "Ensure /var/lib/unattended-upgrade stamp directory"
}

// StampDir creates the persistent stamp directory (day, reboot, kernel and
// clock-latch stamps; see the script header).
func (Unattended) StampDir() {
	onDebian(func() {
		EnsureDir("/var/lib/unattended-upgrade",
			WithMode(0o700), WithOwner("root"), WithGroup("root"))
	})
}

// DescUnits returns the description for systemd timer install.
func (Unattended) DescUnits() string {
	return "Install unattended-upgrade-debian SystemdTimer (oneshot + per-host calendar; needs debian_pis_script, _stamp_dir)"
}

// Units installs the oneshot+timer pair and enables the timer, with the
// per-host OnCalendar from the inventory. ForHosts adds the hostname guard,
// WhenPathExists the Debian guard (onDebian cannot be used: the calendar
// differs per host).
func (Unattended) Units() {
	ForHosts(cluster.ValueUnattendedOnCalendar, func(_ string, calendar string) {
		WhenPathExists(debianMarker, func() {
			SystemdTimer("unattended-upgrade-debian",
				WithCommand("/usr/local/sbin/unattended-upgrade-debian daily"),
				WithOnCalendar(calendar),
				WithOnBootSec("10min"),
				WithPersistent,
				WithDescription("Hourly unattended-upgrade check (updates once per day)"),
				WithServiceDescription("Unattended upgrade (Debian daily mode)"),
				WithAfter("network-online.target"),
				WithWants("network-online.target"),
			)
		})
	})
}

// DescLogrotate returns the description for logrotate.
func (Unattended) DescLogrotate() string {
	return "Install /etc/logrotate.d/unattended-upgrade"
}

// Logrotate installs the logrotate snippet shared with Rocky. Debian's own
// /etc/logrotate.d/unattended-upgrades (plural) rotates the upgrader's logs
// under /var/log/unattended-upgrades/ and does not clash with it.
func (Unattended) Logrotate() {
	src := paths.Frontends + "/systemd/unattended-upgrade.logrotate"
	onDebian(func() {
		InstallFile("/etc/logrotate.d/unattended-upgrade",
			src,
			WithMode(0o644), WithOwner("root"), WithGroup("root"))
	})
}
