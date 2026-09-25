package freebsd

import (
	. "github.com/snonux/gonf/api"
)

// Baseline owns the base-system keys of /etc/rc.conf and /etc/sysctl.conf
// every f-host was installed with (f3s blog parts 2-4) plus the later fixes
// (task dk2, 2026-09-25). The files stay shared: each key is one
// WithKeyedLine, replaced in place when its value differs and appended when
// missing, and every other line (network identity, role keys, lines of other
// recipes) is left alone. The loader.conf *_load lines belong to Loader.
//
// Deliberately NOT managed here:
//   - hostname, ifconfig_re0*, defaultrouter: network identity, set at
//     install time; a wrong value would cut the host off the LAN.
//   - ifconfig_re0_alias0 (the CARP VIP): Carp / task gk2.
//   - NFS, zfskeys, zrepl, node_exporter, uptimed, apcupsd *_enable and
//     flags: NFS, ZfsKeys, Zrepl, Monitoring, Base and Apcupsd.
//   - vm_*, wireguard_*, relayd/pf/pflog, garage, dserver, shellyfans: hand-
//     set role keys, out of this task's scope.
//
// Values are the live ones of 2026-09-25. Keys that differ between hosts
// come from BaselineHost; an unset field leaves that key unmanaged on the
// host, so gonf neither adds nor removes it there.
//
// rc.conf is read at boot and at shutdown (rcshutdown_timeout is read by
// rc.shutdown), sysctl.conf at boot; the one sysctl here is also set live.
//
// Register with OnCluster(cluster.NameFreeBSD); every f-host carries a
// BaselineHost.
type Baseline struct {
	RequiresRoot
}

// BaselineHost holds the baseline keys that differ between f-hosts (host
// data, see gonf/cluster). The zero value manages none of them.
type BaselineHost struct {
	// ClearTmp manages clear_tmp_enable="YES" (wipe /tmp at boot). f0/f1
	// had it, f2/f3 never did; kept as found.
	ClearTmp bool
	// ShutdownTimeout is rcshutdown_timeout, rc.shutdown's watchdog in
	// seconds (default 90). "300" on the k3s hosts f0-f2 since 2026-06-28:
	// stopping the bhyve k3s guest can outlast 90 s, the watchdog then
	// drops to single-user and the host never powers off (f3s skill,
	// console-jetkvm-shutdown.md). f3 has none; "" leaves it unmanaged.
	ShutdownTimeout string
	// Cryptodev manages cryptodev_load="YES" in loader.conf (/dev/crypto).
	// f0-f2 load it, f3 does not; its ZFS encryption works without, since
	// ZFS brings its own crypto.
	Cryptodev bool
}

// baselineRcConfKeys are the rc.conf keys identical on f0-f3, in the live
// spelling. ntpd_sync_on_start="NO" is the 2026-08-09 fix: with YES, rc
// blocked ~14 min on ntpd -q before sshd started. dumpdev="AUTO" keeps the
// crash dumps the kernel panic investigation depends on.
var baselineRcConfKeys = []struct{ key, value string }{
	{"sshd_enable", "YES"},
	{"ntpd_enable", "YES"},
	{"ntpd_sync_on_start", "NO"},
	{"powerd_enable", "YES"},
	{"moused_nondefault_enable", "NO"},
	{"dumpdev", "AUTO"},
	{"zfs_enable", "YES"},
}

// baselineSysctls are the /etc/sysctl.conf settings of every f-host.
// vfs.zfs.vdev.min_auto_ashift=12 (4K sectors for new vdevs) is the 15.1
// name of vfs.zfs.min_auto_ashift. The NFS sysctls belong to NFS.Sysctl.
var baselineSysctls = []struct{ name, value string }{
	{"vfs.zfs.vdev.min_auto_ashift", "12"},
}

// DescRcConf returns the description for the baseline rc.conf keys.
func (Baseline) DescRcConf() string {
	return "rc.conf base keys: sshd/ntpd/powerd/zfs, ntpd_sync_on_start=NO, dumpdev, per-host clear_tmp and rcshutdown_timeout"
}

// RcConf manages the keys line by line; see the Baseline comment. Each key
// ends in "=" so no key is a prefix of another (zfs_enable= does not match
// zfskeys_enable=).
func (Baseline) RcConf() {
	EachHost(func(h BaselineHost) {
		opts := []FileOption{Perm(0o644, Root), WithName("rc-conf-baseline")}
		for _, kv := range baselineRcConfKeys {
			opts = append(opts, rcConfKeyedLine(kv.key, kv.value))
		}
		if h.ClearTmp {
			opts = append(opts, rcConfKeyedLine("clear_tmp_enable", "YES"))
		}
		if h.ShutdownTimeout != "" {
			opts = append(opts, rcConfKeyedLine("rcshutdown_timeout", h.ShutdownTimeout))
		}
		File(rcConf, opts...)
	})
}

// DescSysctl returns the description for the baseline sysctls.
func (Baseline) DescSysctl() string {
	return "sysctl.conf + live: vfs.zfs.vdev.min_auto_ashift=12"
}

// Sysctl writes the settings to sysctl.conf and sets a differing live value.
// min_auto_ashift only affects vdevs added later, so setting it live is
// harmless.
func (Baseline) Sysctl() {
	opts := []FileOption{Perm(0o644, Root), WithName("sysctl-conf-baseline")}
	for _, s := range baselineSysctls {
		opts = append(opts, WithKeyedLine(s.name+"=", s.name+"="+s.value))
	}
	File(sysctlConf, opts...)
	for _, s := range baselineSysctls {
		Command("sysctl", List(s.name+"="+s.value),
			OnlyIf("sh", List("-c", `[ "$(sysctl -n `+s.name+`)" != "`+s.value+`" ]`)),
			WithName("sysctl-live-"+s.name))
	}
}

// rcConfKeyedLine owns the rc.conf line key="value".
func rcConfKeyedLine(key, value string) FileOption {
	return WithKeyedLine(key+"=", key+`="`+value+`"`)
}
