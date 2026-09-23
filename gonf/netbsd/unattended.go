// Package netbsd declares NetBSD pi0/pi1 host tasks for the conf repository —
// unattended upgrades per frontends/docs/unattended-upgrades-pi.plan.md.
package netbsd

import (
	"path/filepath"

	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"

	"codeberg.org/snonux/conf/gonf/cluster"
	"codeberg.org/snonux/conf/gonf/paths"
)

// unattendedServicesAsset is the operator-edited rc.d restart list for
// pi0/pi1: a plain, native file (assets/unattended-upgrade-services), not a
// template. Service name is wireguard (not wireguard-go) — matches
// /etc/rc.d/wireguard on the hosts.
func unattendedServicesAsset() string {
	return filepath.Join(paths.Conf, "gonf", "netbsd", "assets", "unattended-upgrade-services")
}

// unattendedNewsyslogLine rotates /var/log/unattended-upgrade.log (NetBSD
// newsyslog.conf format — same columns as the stock authlog/cron lines).
const unattendedNewsyslogLine = "/var/log/unattended-upgrade.log\troot:wheel\t600  5    1024 *    Z"

// Unattended carries the unattended-upgrade deployment for pi0/pi1.
// Register with WithCluster(cluster.NameNetBSDPis). Per-host cron hours live on
// each Host via WithValue(cluster.ValueUnattendedCron, …).
type Unattended struct {
	RequiresRoot
}

// DescScript returns the description for the wrapper deployment.
func (Unattended) DescScript() string {
	return "Install /usr/local/sbin/unattended-upgrade-netbsd (0755 root:wheel)"
}

// Script installs the NetBSD ksh wrapper.
func (Unattended) Script() {
	WhenHostname(ClusterHosts(), func() {
		dir := EnsureDir("/usr/local/sbin",
			WithMode(0o755), WithOwner("root"), WithGroup("wheel"))
		InstallFile("/usr/local/sbin/unattended-upgrade-netbsd",
			paths.Frontends+"/scripts/unattended-upgrade-netbsd.sh",
			WithMode(0o755), WithOwner("root"), WithGroup("wheel"),
			DependsOn(dir))
	})
}

// DescServices returns the description for the restart list.
func (Unattended) DescServices() string {
	return "Install /etc/unattended-upgrade-services (0644 root:wheel)"
}

// Services installs the rc.d restart list.
func (Unattended) Services() {
	WhenHostname(ClusterHosts(), func() {
		InstallFile("/etc/unattended-upgrade-services", unattendedServicesAsset(),
			WithMode(0o644), WithOwner("root"), WithGroup("wheel"))
	})
}

// DescCron returns the description for the per-host cron schedule.
func (Unattended) DescCron() string {
	return "Root cron: unattended-upgrade-netbsd pkgs/reboot, per-host schedule (needs pis_netbsd_script, pis_netbsd_services)"
}

// Cron installs pkgs + reboot cron jobs with per-host windows.
func (Unattended) Cron() {
	ForHosts(cluster.ValueUnattendedCron, func(_ string, w [2]string) { unattendedCronJobs(w) })
}

func unattendedCronJobs(w [2]string) {
	Cron("unattended-upgrade-netbsd-pkgs",
		WithCommand("/usr/local/sbin/unattended-upgrade-netbsd pkgs"),
		WithMinute("10"), WithHour(w[0]))
	Cron("unattended-upgrade-netbsd-reboot",
		WithCommand("/usr/local/sbin/unattended-upgrade-netbsd reboot"),
		WithMinute("50"), WithHour(w[1]))
}

// DescNewsyslog returns the description for the rotation line.
func (Unattended) DescNewsyslog() string {
	return "Append unattended-upgrade log rotation to /etc/newsyslog.conf"
}

// Newsyslog appends the rotation line.
func (Unattended) Newsyslog() {
	WhenHostname(ClusterHosts(), func() {
		File("/etc/newsyslog.conf", WithLine(unattendedNewsyslogLine), WithMode(0o644))
	})
}
