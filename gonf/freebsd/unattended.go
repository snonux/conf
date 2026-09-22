// Package freebsd declares FreeBSD f0–f3 host tasks for the conf repository —
// unattended package upgrades per
// frontends/docs/unattended-upgrades-freebsd.plan.md.
package freebsd

import (
	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"

	"codeberg.org/snonux/conf/gonf/cluster"
	"codeberg.org/snonux/conf/gonf/paths"
)

// unattendedServicesContent is the rc.d restart list for f0–f3 after pkg
// upgrades. Never list vm / networking here — guest stop belongs to the
// reboot path (vm stopall then reboot).
const unattendedServicesContent = `# Daemons to restart after unattended package updates (one per line).
# Names must match /etc/rc.d/<name> or /usr/local/etc/rc.d/<name>.
# Do NOT list vm, vm_network, netif, routing, or devd.
sshd
node_exporter
dserver
wireguard
uptimed
`

// unattendedNewsyslogLine rotates /var/log/unattended-upgrade.log.
const unattendedNewsyslogLine = "/var/log/unattended-upgrade.log\t\troot:wheel\t600  5     1024  *     Z"

// Unattended carries the unattended-upgrade deployment for FreeBSD hosts.
// Register with WithCluster(cluster.NameFreeBSD). Per-host hourly minute and
// allow-reboot live on each Host via WithValue.
type Unattended struct {
	RequiresRoot
}

// DescPackages returns the description for required packages.
func (Unattended) DescPackages() string {
	return "Install ksh (ksh93) for unattended-upgrade-freebsd"
}

// Packages installs ksh (script interpreter, house rule).
func (Unattended) Packages() {
	WhenHostname(ClusterHosts(), func() {
		Package("ksh")
	})
}

// DescScript returns the description for the wrapper deployment.
func (Unattended) DescScript() string {
	return "Install /usr/local/sbin/unattended-upgrade-freebsd (0755 root:wheel)"
}

// Script installs the FreeBSD ksh wrapper.
func (Unattended) Script() {
	WhenHostname(ClusterHosts(), func() {
		dir := EnsureDir("/usr/local/sbin",
			WithMode(0o755), WithOwner("root"), WithGroup("wheel"))
		InstallFile("/usr/local/sbin/unattended-upgrade-freebsd",
			paths.Frontends+"/scripts/unattended-upgrade-freebsd.sh",
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
		File("/etc/unattended-upgrade-services",
			WithContent(unattendedServicesContent),
			WithMode(0o644), WithOwner("root"), WithGroup("wheel"))
	})
}

// DescStampDir returns the description for the stamp directory.
func (Unattended) DescStampDir() string {
	return "Ensure /var/lib/unattended-upgrade stamp directory"
}

// StampDir creates the persistent stamp directory.
func (Unattended) StampDir() {
	WhenHostname(ClusterHosts(), func() {
		EnsureDir("/var/lib/unattended-upgrade",
			WithMode(0o700), WithOwner("root"), WithGroup("wheel"))
	})
}

// DescCron returns the description for the hourly cron.
func (Unattended) DescCron() string {
	return "Root cron: unattended-upgrade-freebsd daily, per-host hourly minute"
}

// Cron installs the hourly daily-mode job (stamp-gated in-script).
// Boot catch-up is the next hourly tick (gonf Cron has no @reboot field).
func (Unattended) Cron() {
	ForHosts(cluster.ValueUnattendedCronMinute, func(_ string, minute string) {
		Cron("unattended-upgrade-freebsd-daily",
			WithCommand("/usr/local/sbin/unattended-upgrade-freebsd daily"),
			WithMinute(minute), WithHour("*"),
			WithCronEnv("PATH=/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin:/usr/local/sbin"),
		)
	})
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
