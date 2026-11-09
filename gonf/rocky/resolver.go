package rocky

import (
	"github.com/snonux/conf/gonf/labdns"
	. "github.com/snonux/gonf/api"
)

// Resolver owns the nameserver of the Rocky hosts (pi2, pi3, r0-r2 and the
// standalone guest; task u54). Their hand-made NetworkManager profiles name
// the ISP CPE 192.168.1.1 (the guest: the Pi-holes' retained addresses, then
// the CPE), which the lab cannot reach after the b24 cable move.
//
// A global DNS drop-in replaces every per-connection DNS server and search
// domain in /etc/resolv.conf, so the profiles stay untouched: they are
// bootstrap state (rnodes.Base) and the network-migration rollback snapshots
// them. Their stale ipv4.dns is ignored while this drop-in exists, and
// "nmcli general reload" applies it without touching any interface.
//
// The search domain keeps the profiles' lan.buetow.org (the guest had none).
// kubelet copies resolv.conf into pods only when it creates them, so CoreDNS
// (forward . /etc/resolv.conf, dnsPolicy Default) needs a rollout restart
// once r0-r2 carry the new file.
//
// Rollback on one host: rm the drop-in, then nmcli general reload conf,dns-rc.
// A later rocky aggregate run re-applies it.
type Resolver struct{ RequiresRoot }

const resolverDropIn = "/etc/NetworkManager/conf.d/90-gonf-dns.conf"

// Conf installs the NetworkManager global DNS drop-in for the lab resolver
// and reloads NetworkManager's configuration.
func (Resolver) Conf() {
	conf := File(resolverDropIn, WithContent(`# Managed by gonf (rocky_resolver_conf, task u54). Global DNS overrides the
# per-connection ipv4.dns of every profile: the router's unbound on the lab
# gateway, reachable before and after the b24/c24 cable move.
[global-dns]
searches=lan.buetow.org

[global-dns-domain-*]
servers=`+labdns.Resolver+`
`), RootOwned)
	// SELinux is enforcing and gonf keeps no labels (see rnodes.Base).
	label := Sh("restorecon "+resolverDropIn, OnChange(conf))
	Sh("nmcli general reload conf,dns-rc", OnChange(conf), DependsOn(label))
}
