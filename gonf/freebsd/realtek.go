package freebsd

import (
	. "github.com/snonux/gonf/api"
)

// Realtek removes what the Realtek vendor NIC driver experiment (conf task
// t74, 2026-11-13) left on the f-hosts. Transitional: deploy once, verify,
// then delete this file with its registration (conf task x74, "remove host
// artifacts before code").
//
// t74 moved f1-f3 from the in-tree re(4) to net/realtek-re-kmod to stop
// the RTL8168H transmit stalls ("re0: watchdog timeout"). The user then
// chose the in-tree driver after all: the vendor module is built per
// FreeBSD minor release and has to be matched and checked at every update.
// x74 rebooted f1-f3 back to the in-tree re(4); f0 never left it. See IQ
// topics/infra/f3s/hosts/re0-watchdog.md.
//
// Register with OnCluster(cluster.NameFreeBSD).
type Realtek struct {
	RequiresRoot
}

const (
	realtekPackage = "realtek-re-kmod"
	realtekCheck   = "/usr/local/libexec/gonf-realtek-kmod-check"
)

// The loader.conf lines t74 wrote, spelled exactly as written then, so
// WithoutLines finds them on a host that still has them.
const (
	realtekLoadLine = `if_re_load="YES"` + "\t" +
		`# Realtek vendor re(4), realtek-re-kmod (conf t74); 15.x minor upgrade: run gonf-realtek-kmod-check first (IQ re0-watchdog.md)`
	realtekNameLine  = `if_re_name="/boot/modules/if_re.ko"`
	realtekS5WolLine = `hw.re.s5wol="1"` + "\t" +
		`# vendor re(4) powers the PHY down at shutdown otherwise: no Wake-on-LAN (f3sctl power on)`
)

// LoaderConf removes the vendor driver lines from /boot/loader.conf, so the
// in-tree re(4) claims re0 at the next boot.
func (Realtek) LoaderConf() {
	File(loaderConf, WithoutLines(realtekLoadLine, realtekNameLine, realtekS5WolLine),
		RootOwned, WithName("loader-conf-realtek-absent"))
}

// OptsPackage removes the module only after loader.conf stops loading it.
func (Realtek) OptsPackage() TaskOptions {
	return TaskOptions{Needs(Realtek.LoaderConf)}
}

// Package removes realtek-re-kmod (/boot/modules/if_re.ko). A module that
// is still loaded keeps running until the next boot.
func (Realtek) Package() {
	NoPackage(realtekPackage)
}

// Check removes gonf-realtek-kmod-check, the vendor driver's upgrade guard.
func (Realtek) Check() {
	NoFile(realtekCheck)
}
