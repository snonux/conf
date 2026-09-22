package debian

import (
	"path/filepath"

	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"

	"codeberg.org/snonux/conf/gonf/paths"
)

// nftablesConf replaces Debian's /etc/nftables.conf. It owns one table only
// and deliberately has no "flush ruleset": Docker keeps its own tables
// (iptables-nft), which a flush would drop. The first two lines make the
// replace idempotent (declare, so the delete never fails; delete; define).
// The ports are the firewalld set of the Rocky Pis: SSH, Pi-hole DNS
// (tcp+udp), the Pi-hole admin UI, DTail dserver, plus ICMP (the partner
// ping gate) and DHCPv6 client replies (firewalld's dhcpv6-client).
const nftablesConf = `#!/usr/sbin/nft -f
# Managed by gonf (conf: gonf/debian/base.go). Do not edit here.
#
# No "flush ruleset": Docker's tables must survive a reload. Never
# "systemctl restart nftables" on this host either: its ExecStop flushes the
# whole ruleset. "systemctl reload nftables" re-reads this file.
table inet gonf_filter
delete table inet gonf_filter

table inet gonf_filter {
	chain input {
		type filter hook input priority filter; policy drop;
		ct state established,related accept
		ct state invalid drop
		iifname "lo" accept
		meta l4proto { icmp, ipv6-icmp } accept
		# SSH, Pi-hole DNS, Pi-hole admin UI, DTail dserver
		tcp dport { 22, 53, 80, 2222 } accept
		udp dport 53 accept
		# DHCPv6 client replies
		ip6 daddr fe80::/64 udp dport 546 accept
	}
}
`

// dockerSources is the Docker CE apt source (deb822), pinned to the
// vendored signing key rather than a key fetched at deploy time.
const dockerSources = `# Managed by gonf (conf: gonf/debian/base.go). Do not edit here.
Types: deb
URIs: https://download.docker.com/linux/debian
Suites: trixie
Components: stable
Architectures: arm64
Signed-By: /etc/apt/keyrings/docker.asc
`

// dockerDaemonJSON caps container logs (SD-card writes, doc §5). It applies
// to containers created after the change: recreate Pi-hole
// (docker compose up -d --force-recreate) for it to take effect.
const dockerDaemonJSON = `{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "10m",
    "max-file": "3"
  }
}
`

// uptimedTimeSync is the uptimed drop-in the Rocky Pis carry by hand today
// (/etc/systemd/system/uptimed.service.d/time-sync.conf): without an RTC,
// uptimed must not start before chrony has stepped the clock, or it records
// a bogus boot time. On Debian the unit is chrony.service (chronyd.service
// is only its alias).
const uptimedTimeSync = `# Managed by gonf (conf: gonf/debian/base.go). Do not edit here.
[Unit]
Wants=network-online.target chrony.service
After=network-online.target chrony.service

[Service]
ExecStartPre=/usr/bin/chronyc waitsync 60 0.5
`

// journaldConf keeps the journal in RAM (SD-card writes, doc §5), capped so
// it cannot squeeze the 1 GB Pi. The journal of a previous boot is lost;
// /var/log/unattended-upgrade.log keeps the unattended reboot history.
const journaldConf = `# Managed by gonf (conf: gonf/debian/base.go). Do not edit here.
[Journal]
Storage=volatile
RuntimeMaxUse=32M
`

// Base carries the Debian Pi host base that firewalld, dnf and hand-made
// drop-ins provide on the Rocky Pis today (doc §5): the nftables firewall,
// Docker CE for Pi-hole, uptimed after chrony sync, and the SD-card write
// limits for journald and container logs. Register with
// WithCluster(cluster.NameDebianPis).
//
// Not managed here yet (follow-ups): noatime on the root mount (the image's
// fstab), Pi-hole's query-log retention (database.maxDBdays, part of the
// ~/pihole deployment in f3s/pihole/docker-pi), DTail (task m82).
type Base struct {
	RequiresRoot
}

// DescFirewall returns the description for the nftables firewall.
func (Base) DescFirewall() string {
	return "Install the nftables input filter (22, 53 tcp/udp, 80, 2222) and enable nftables"
}

// Firewall installs nftables and the ruleset, validated with nft -c before
// the live file changes, and reloads (never restarts, see nftablesConf) the
// service when the ruleset changed.
func (Base) Firewall() {
	onDebian(func() {
		pkg := aptPackages(List("nftables"))
		conf := File("/etc/nftables.conf",
			WithContent(nftablesConf),
			WithValidation("nft", List("-c", "-f", CandidatePath)),
			WithMode(0o755), WithOwner("root"), WithGroup("root"),
			DependsOn(pkg))
		Service("nftables", WithReload, DependsOn(pkg), OnChange(conf))
	})
}

// DescDocker returns the description for Docker CE.
func (Base) DescDocker() string {
	return "Install Docker CE from its apt repository with capped container logs"
}

// Docker adds the Docker CE apt source with its vendored key
// (f3s/pi-debian/apt/docker.asc, fingerprint
// 9DC8 5822 9FC7 DD38 854A E2D8 8D81 803C 0EBF CD88), installs the engine
// and the compose plugin Pi-hole's ~/pihole deployment uses, and caps the
// container logs. dockerd restarts only when daemon.json changed — that
// restarts Pi-hole too, so the partner resolver carries DNS meanwhile.
func (Base) Docker() {
	onDebian(func() {
		keyrings := rootDir("/etc/apt/keyrings")
		key := InstallFile("/etc/apt/keyrings/docker.asc",
			filepath.Join(paths.Conf, "f3s", "pi-debian", "apt", "docker.asc"),
			WithMode(0o644), WithOwner("root"), WithGroup("root"),
			DependsOn(keyrings))
		sources := File("/etc/apt/sources.list.d/docker.sources",
			WithContent(dockerSources),
			WithMode(0o644), WithOwner("root"), WithGroup("root"),
			DependsOn(key))
		pkgs := aptPackages(List("docker-ce", "docker-ce-cli", "containerd.io", "docker-compose-plugin"),
			DependsOn(sources))
		etc := rootDir("/etc/docker")
		daemon := File("/etc/docker/daemon.json",
			WithContent(dockerDaemonJSON),
			WithValidation("dockerd", List("--validate", "--config-file", CandidatePath)),
			WithMode(0o644), WithOwner("root"), WithGroup("root"),
			DependsOn(pkgs, etc))
		Service("docker", WithRestart, DependsOn(pkgs), OnChange(daemon))
	})
}

// DescUptimed returns the description for uptimed.
func (Base) DescUptimed() string {
	return "Install chrony and uptimed, starting uptimed only after the clock is synchronised"
}

// Uptimed installs chrony (replacing systemd-timesyncd, so chronyc waitsync
// works) and uptimed with its time-sync drop-in. SystemdUnits reloads
// systemd for the drop-in and restarts uptimed only when it changed.
func (Base) Uptimed() {
	onDebian(func() {
		pkgs := aptPackages(List("chrony", "uptimed"))
		dir := rootDir("/etc/systemd/system/uptimed.service.d")
		dropin := File("/etc/systemd/system/uptimed.service.d/time-sync.conf",
			WithContent(uptimedTimeSync),
			WithMode(0o644), WithOwner("root"), WithGroup("root"),
			DependsOn(pkgs, dir))
		SystemdUnits(FanIn(dropin), ActivateService("uptimed", WithRestart))
	})
}

// DescJournald returns the description for the journald SD-card limits.
func (Base) DescJournald() string {
	return "Keep the systemd journal in RAM (Storage=volatile, 32M) to spare the SD card"
}

// Journald installs the journald drop-in and restarts journald when it
// changed (journald reads its own config; no daemon-reload is needed).
func (Base) Journald() {
	onDebian(func() {
		dir := rootDir("/etc/systemd/journald.conf.d")
		conf := File("/etc/systemd/journald.conf.d/50-sd-card.conf",
			WithContent(journaldConf),
			WithMode(0o644), WithOwner("root"), WithGroup("root"),
			DependsOn(dir))
		Service("systemd-journald", WithRestart, OnChange(conf))
	})
}
