package netbsd

import (
	"net/netip"

	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

// Network owns pi0/pi1's steady lab addressing and NPF policy (task b74).
//
// The network-migration helper (tasks 624, i24, j24) used to apply part of
// this at boot: its accepted "retire lab" override added the lab ULA, which
// rc.conf never carried. Since b74 rc.conf holds everything (lab IPv4, ULA,
// default 10.20.0.1), so the hosts boot on their final addresses with plain
// rc.d/network, and /etc/npf.conf is rendered from the policy j24 installed.
// tun0 belongs to the userspace WireGuard service (wireguardmeshgenerator)
// and is never configured here.
type Network struct{ RequiresRoot }

// LANAddress is a controller's address on the lab segment. Interface is the
// wired NIC (mue0 on the Pi 3B+), IPv4 the /24 lab address and ULA the /64
// unique-local address; router advertisements stay off (ip6mode default).
type LANAddress struct {
	Interface, IPv4, ULA string
}

const (
	rcConf  = "/etc/rc.conf"
	npfConf = "/etc/npf.conf"
	// labGateway is triggerfish's lab address, the only default route.
	labGateway = "10.20.0.1"
)

// RcConf writes the lab IPv4, the ULA and the default route to rc.conf.
func (Network) RcConf() {
	EachHost(func(a LANAddress) {
		a.mustValidate()
		// rc.d/network splits ifconfig_<if> at ';' into separate ifconfig
		// calls: the primary IPv4 first, then the ULA as an inet6 alias.
		// RootOwned resets rc.conf to the stock 0644 (it was 0600, likely
		// from the bake script's mktemp/mv); it holds no secrets.
		File(rcConf, WithShellVars(
			"ifconfig_"+a.Interface, "inet "+a.IPv4+" netmask 0xffffff00;inet6 "+a.ULA+" prefixlen 64 alias",
			"defaultroute", labGateway,
		), RootOwned, WithName("rc-conf-network"))
	})
}

// NPF installs the validated /etc/npf.conf and reloads it when it changed.
func (Network) NPF() {
	EachHost(func(a LANAddress) {
		a.mustValidate()
		conf := InstallFile(npfConf, paths.GonfAsset("netbsd", "npf.conf"), RootOwned,
			WithTemplateData(a), WithValidation("/sbin/npfctl", List("validate", CandidatePath)))
		// reload swaps the ruleset atomically and keeps filtering active and
		// existing state; never stop/flush (that drops the SSH session).
		Sh("/sbin/npfctl reload", OnChange(conf))
		File(rcConf, WithShellVar("npf", "YES"), RootOwned, WithName("rc-conf-npf"), DependsOn(conf))
	})
}

// mustValidate rejects inventory rows that would render a broken rc.conf or
// NPF policy: a non-IPv4 lab address or a non-ULA IPv6 address.
func (a LANAddress) mustValidate() {
	if a.Interface == "" {
		panic("netbsd.LANAddress: Interface is empty")
	}
	if !netip.MustParseAddr(a.IPv4).Is4() {
		panic("netbsd.LANAddress: IPv4 is not an IPv4 address")
	}
	if ula := netip.MustParseAddr(a.ULA); !ula.Is6() || !ula.IsPrivate() {
		panic("netbsd.LANAddress: ULA is not a unique-local IPv6 address")
	}
}
