package freebsd

import (
	"github.com/snonux/conf/gonf/labdns"
	. "github.com/snonux/gonf/api"
)

// Resolver owns the f-hosts' nameserver (task u54). Until 2026-11-09
// /etc/resolvconf.conf was hand-made and named the ISP CPE 192.168.1.1, which
// the lab cannot reach after the b24 cable move.
//
// resolvconf(8) owns /etc/resolv.conf: no dhclient runs (re0 is static) and
// rtsold is off (-accept_rtadv), so name_servers is the only source and
// "resolvconf -u" regenerates the file at once. private_interfaces keeps any
// RA RDNSS on re0 out of the global list, as before.
//
// Rollback on one host: name_servers="192.168.1.1" in /etc/resolvconf.conf,
// then resolvconf -u. A later freebsd aggregate run re-applies this file.
type Resolver struct{ RequiresRoot }

const resolvconfConf = "/etc/resolvconf.conf"

// Conf writes /etc/resolvconf.conf with the lab resolver and regenerates
// /etc/resolv.conf.
func (Resolver) Conf() {
	conf := File(resolvconfConf, WithContent(`# Managed by gonf (freebsd_resolver_conf, task u54). Static nameserver: the
# router's unbound on the lab gateway, reachable before and after the b24/c24
# cable move. Mark re0 as private so SLAAC-provided IPv6 DNS (Router
# Advertisement RDNSS) is not added globally.
name_servers="`+labdns.Resolver+`"
private_interfaces="re0"
`), RootOwned)
	Sh("resolvconf -u", OnChange(conf))
}
