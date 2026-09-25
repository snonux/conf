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
// Register with OnCluster(cluster.NameFreeBSD).
type ShellyFans struct {
	RequiresRoot
}

const (
	shellyFansScript   = "/usr/local/sbin/shelly-fans-on"
	shellyFansRcScript = "/usr/local/etc/rc.d/shellyfans"
)

// DescScripts returns the description for the fan helper and rc.d script.
func (ShellyFans) DescScripts() string {
	return "Install shelly-fans-on and /usr/local/etc/rc.d/shellyfans (0555 root:wheel; not run)"
}

// Scripts installs the helper and the rc.d service (0555, as hand-installed).
func (ShellyFans) Scripts() {
	InstallFile(shellyFansScript, paths.FHostAsset("shelly-fans/shelly-fans-on"), Perm(0o555, Root))
	InstallFile(shellyFansRcScript, paths.FHostAsset("shelly-fans/shellyfans.rc"), Perm(0o555, Root))
}

// DescRcConf returns the description for the shellyfans rc.conf key.
func (ShellyFans) DescRcConf() string {
	return "rc.conf: shellyfans_enable=YES (rack fans on at boot)"
}

// OptsRcConf records the scripts first, so shellyfans_enable never names a
// missing rc.d service.
func (ShellyFans) OptsRcConf() TaskOptions {
	return TaskOptions{Needs("scripts")}
}

// RcConf owns the single shellyfans_enable line in place; the rest of
// rc.conf belongs to other recipes or stays hand-managed, hence the WithName.
// The value is the one sysrc wrote on every host, so nothing changes.
func (ShellyFans) RcConf() {
	File(rcConf,
		WithKeyedLine("shellyfans_enable=", `shellyfans_enable="YES"`),
		Perm(0o644, Root),
		WithName("rc-conf-shellyfans"))
}
