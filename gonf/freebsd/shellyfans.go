package freebsd

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// ShellyFans installs the boot-time rack-fan switch on the f-hosts (f3s
// skill, references/shelly-plug.md): /usr/local/sbin/shelly-fans-on switches
// the shelly1 plug on, and the shellyfans rc.d service runs it in the
// background after NETWORKING and f3skeys at every boot. Sources live in
// f3s/freebsd-hosts/shelly-fans; until task ik2 they were copied by hand
// and matched the repo byte for byte on f0-f3.
//
// The plug password stays on the USB key stick (/keys/shelly_plug.secret,
// read by the helper at run time); it is neither in git nor in gonf. gonf
// never runs the helper or the service: switching the plug is left to the
// next boot and to f3sctl.
//
// f3 is not cooled by shelly1, but it runs the service too (live state
// adopted), so every f-host gets it and no per-host data is needed.
//
// Since p34 the helper talks HTTPS to the plug and pins the private f3s
// Shelly CA, installed here as public material next to the helper's config
// (the same certificate gonf/netbsd ships to pi0/pi1 for f3sctl).
//
// Register with OnCluster(cluster.NameFreeBSD).
type ShellyFans struct {
	RequiresRoot
}

const (
	shellyFansScript   = "/usr/local/sbin/shelly-fans-on"
	shellyFansRcScript = "/usr/local/etc/rc.d/shellyfans"
	shellyFansCADir    = "/usr/local/etc/shellyfans"
	shellyFansCA       = shellyFansCADir + "/shelly-ca.pem"
)

// Scripts installs the fan helper, its pinned CA, rc service and managed address override without running them.
func (ShellyFans) Scripts() {
	EnsureDir(shellyFansCADir, RootOwned)
	ca := InstallFile(shellyFansCA, paths.FHostAsset("shelly-fans/shelly-ca.pem"), RootOwned)
	// The CA is in place before a helper that requires it.
	InstallFile(shellyFansScript, paths.FHostAsset("shelly-fans/shelly-fans-on"), Perm(0o555, Root),
		WithValidation("/bin/sh", List("-n", CandidatePath)), DependsOn(ca))
	InstallFile(shellyFansRcScript, paths.FHostAsset("shelly-fans/shellyfans.rc"), Perm(0o555, Root))
	EnsureDir("/usr/local/etc/rc.conf.d", RootOwned)
	File("/usr/local/etc/rc.conf.d/shellyfans", RootOwned,
		WithContent("# Managed rack fan address; the standalone helper uses the same default.\nshellyfans_ip=\"10.30.0.28\"\n"))
}

// DescRcConf returns the description for the shellyfans rc.conf key.
func (ShellyFans) DescRcConf() string {
	return "rc.conf: shellyfans_enable=YES (rack fans on at boot)"
}

// OptsRcConf records the scripts first, so shellyfans_enable never names a
// missing rc.d service.
func (ShellyFans) OptsRcConf() TaskOptions {
	return TaskOptions{Needs(ShellyFans.Scripts)}
}

// RcConf owns the single shellyfans_enable line in place; the rest of
// rc.conf belongs to other recipes or stays hand-managed, hence the WithName.
// The value is the one sysrc wrote on every host, so nothing changes.
func (ShellyFans) RcConf() {
	File(rcConf,
		WithShellVar("shellyfans_enable", "YES"),
		RootOwned,
		WithName("rc-conf-shellyfans"))
}
