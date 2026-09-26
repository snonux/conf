package freebsd

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// NFS manages the NFS server and the stunnel TLS front of the CARP storage
// pair f0/f1 (f3s-storage skill, references/nfs.md and carp.md): the rc.conf
// keys, the NFS sysctls, /etc/exports, the stunnel package and its config,
// and f0's zrepl liveness canary cron job. Until 2026-09-25 all of it was
// hand-set; f1, the failover target, had drifted from f0 (task fk2):
//   - mountd_flags="-p 2050" was missing, so after a failover mountd would
//     run with the stock "-r -S" on a random port;
//   - vfs.nfs.enable_uidtostring and vfs.nfsd.enable_stringtouid were missing
//     from sysctl.conf (and 0 live), so NFSv4 owner strings would map
//     differently after a failover.
//
// f0 carried rpc_tlsservd_enable="YES", which f1 lacked. No rc.d script reads
// that variable (the rc.d/tlsservd rcvar is tlsservd_enable, "NO" on both:
// TLS is stunnel's job here), so instead of copying the dead key to f1 it is
// removed from f0.
//
// Services are NOT started, stopped or restarted here: carpcontrol.sh (the
// devd CARP hook, f3s/freebsd-hosts/carp/carpcontrol.sh) runs "service X
// start" for rpcbind/mountd/nfsd/nfsuserd and "service stunnel restart" on
// MASTER and stops them on BACKUP. That is why every *_enable key stays
// "YES" on both hosts: "service X start" refuses a service whose rcvar is
// off. A changed rc.conf flag or stunnel.conf therefore takes effect on the
// next CARP transition or reboot of that host; restart by hand on the MASTER
// only when that must happen sooner. The one exception is /etc/exports:
// mountd re-reads it on SIGHUP without dropping clients, so a change is
// reloaded, and only where mountd runs (the MASTER).
//
// TLS keys and certificates (server-cert.pem, server-key.pem, ca/) stay on
// the hosts, outside git; stunnel.conf only references their paths.
//
// Register with OnCluster(cluster.NameFreeBSD); the bodies narrow to the CARP
// members with WhenHostname, since f2/f3 serve no NFS.
type NFS struct {
	RequiresRoot
}

const (
	rcConf      = "/etc/rc.conf"
	sysctlConf  = "/etc/sysctl.conf"
	nfsExports  = "/etc/exports"
	stunnelConf = "/usr/local/etc/stunnel/stunnel.conf"
)

// nfsRcConfKeys are the NFS/stunnel keys of /etc/rc.conf, in f0's live
// order and spelling (the values f0 has served with since 2025).
// nfs_reserved_port_only="NO" matters: FreeBSD 15 defaults it to YES and
// rc.d/nfsd then sets vfs.nfsd.nfs_privport=1, which locks out the stunnel
// clients connecting from unprivileged ports.
var nfsRcConfKeys = []struct{ key, value string }{
	{"nfs_server_enable", "YES"},
	{"nfsv4_server_enable", "YES"},
	{"nfsuserd_enable", "YES"},
	{"nfsuserd_flags", "-domain lan.buetow.org"},
	{"mountd_enable", "YES"},
	{"rpcbind_enable", "YES"},
	{"nfsd_flags", ""},
	{"tlsservd_enable", "NO"},
	{"mountd_flags", "-p 2050"},
	{"stunnel_enable", "YES"},
	{"nfs_reserved_port_only", "NO"},
}

// deadRcConfLine is f0's no-op rc.conf line (see the NFS type comment).
const deadRcConfLine = `rpc_tlsservd_enable="YES"`

// nfsSysctls are the /etc/sysctl.conf settings of the NFS server: kernel TLS
// on, unprivileged client ports allowed (stunnel), and numeric uid/gid
// strings accepted in both directions for the NFSv4 owner mapping.
var nfsSysctls = []struct{ name, value string }{
	{"kern.ipc.tls.enable", "1"},
	{"vfs.nfsd.nfs_privport", "0"},
	{"vfs.nfs.enable_uidtostring", "1"},
	{"vfs.nfsd.enable_stringtouid", "1"},
}

// liveCheckCommand is f0's zrepl replication canary (f3s-storage skill,
// references/zrepl.md): every 10 minutes it writes the epoch into
// /data/nfs/nfs.LIVE_CHECK, and the replicated copy on f1 shows how far
// zrepl lags. The text is the hand-added crontab line, byte for byte (% is
// escaped for cron), so gonf adopts it. f0 only: on f1 /data/nfs is the
// read-only zrepl sink and the touch would fail.
const liveCheckCommand = `test -f /data/nfs/nfs.DO_NOT_REMOVE && /usr/bin/touch /data/nfs/nfs.LIVE_CHECK && /bin/date +\%s > /data/nfs/nfs.LIVE_CHECK`

// DescRcConf returns the description for the rc.conf keys.
func (NFS) DescRcConf() string {
	return "rc.conf NFS/stunnel keys on f0/f1 (mountd -p 2050, *_enable YES for carpcontrol); applies on next CARP transition"
}

// WhenRcConf narrows the task to the CARP members.
func (NFS) WhenRcConf() TaskOption { return WhenHostnameIn(carpMembers...) }

// RcConf owns one line per key via WithKeyedLine (replaced in place, appended
// when missing); the rest of rc.conf stays hand-managed. Each key ends in "="
// so no key is a prefix of another (nfsd_flags vs nfsuserd_flags).
func (NFS) RcConf() {
	opts := []FileOption{WithoutLine(deadRcConfLine), RootOwned}
	for _, kv := range nfsRcConfKeys {
		opts = append(opts, WithShellVar(kv.key, kv.value))
	}
	File(rcConf, opts...)
}

// DescSysctl returns the description for the NFS sysctls.
func (NFS) DescSysctl() string {
	return "sysctl.conf + live: kern TLS, nfs_privport=0, NFSv4 uid<->string on f0/f1"
}

// WhenSysctl narrows the task to the CARP members.
func (NFS) WhenSysctl() TaskOption { return WhenHostnameIn(carpMembers...) }

// Sysctl writes the settings to sysctl.conf (read at boot) and sets each live
// value that differs. Setting them is safe while the NFS server runs: they
// only steer the owner-string mapping and port check of new requests, and on
// the BACKUP no nfsd runs at all.
func (NFS) Sysctl() {
	opts := []FileOption{RootOwned}
	for _, s := range nfsSysctls {
		opts = append(opts, WithKeyedLine(s.name+"=", s.name+"="+s.value))
	}
	File(sysctlConf, opts...)
	for _, s := range nfsSysctls {
		Command("sysctl", List(s.name+"="+s.value),
			OnlyIf("sh", List("-c", `[ "$(sysctl -n `+s.name+`)" != "`+s.value+`" ]`)))
	}
}

// DescExports returns the description for /etc/exports.
func (NFS) DescExports() string {
	return "/etc/exports on f0/f1 (/data/nfs to the stunnel loopback); mountd reload on change where it runs"
}

// WhenExports narrows the task to the CARP members.
func (NFS) WhenExports() TaskOption { return WhenHostnameIn(carpMembers...) }

// Exports installs /etc/exports (stunnel clients arrive from 127.0.0.1) and,
// on change, sends mountd its SIGHUP reload, which re-reads the exports
// without dropping clients. The status guard limits that to the host where
// carpcontrol.sh started mountd (the MASTER); the BACKUP picks the file up on
// its next start.
func (NFS) Exports() {
	exports := InstallFile(nfsExports, paths.FHostAsset("nfs/exports"), RootOwned)
	Command("sh", List("-c", "if service mountd status >/dev/null 2>&1; then service mountd reload; fi"),
		OnChange(exports))
}

// WhenStunnelPackage narrows the task to the CARP members.
func (NFS) WhenStunnelPackage() TaskOption { return WhenHostnameIn(carpMembers...) }

// StunnelPackage installs stunnel on f0/f1.
func (NFS) StunnelPackage() {
	Packages("stunnel")
}

// DescStunnelConf returns the description for stunnel.conf.
func (NFS) DescStunnelConf() string {
	return "stunnel.conf on f0/f1 (VIP 192.168.1.138:2323 -> 127.0.0.1:2049, client certs required); no restart"
}

// OptsStunnelConf records the package (and its config dir) first.
func (NFS) OptsStunnelConf() TaskOptions {
	return TaskOptions{Needs(NFS.StunnelPackage)}
}

// WhenStunnelConf narrows the task to the CARP members.
func (NFS) WhenStunnelConf() TaskOption { return WhenHostnameIn(carpMembers...) }

// StunnelConf installs the server config. The asset is the live file byte
// for byte (it has no trailing newline). stunnel is not restarted: on the
// MASTER that would cut every NFS mount of the k3s nodes; carpcontrol.sh
// restarts it on the next MASTER transition.
//
// The config sets "pid = /var/run/stunnel/stunnel.pid", the rc.d script's
// stunnel_pidfile: stunnel 5 writes no pid file by default, so until
// 2026-09-26 "service stunnel status/stop/restart" never saw the running
// daemon -- carpcontrol.sh's BACKUP "service stunnel stop" left stunnel
// running on f1, and its MASTER "restart" could not replace the old one.
func (NFS) StunnelConf() {
	InstallFile(stunnelConf, paths.FHostAsset("nfs/stunnel.conf"), RootOwned)
}

// DescLiveCheckCron returns the description for the zrepl canary job.
func (NFS) DescLiveCheckCron() string {
	return "Root cron on f0: write the epoch to /data/nfs/nfs.LIVE_CHECK every 10 min (zrepl canary)"
}

// WhenLiveCheckCron narrows the task to the failback host.
func (NFS) WhenLiveCheckCron() TaskOption { return WhenHostnameIn(carpFailbackHost) }

// LiveCheckCron adopts f0's canary job (see liveCheckCommand).
func (NFS) LiveCheckCron() {
	CronAt("nfs-live-check", "*/10 * * * *", liveCheckCommand)
}
