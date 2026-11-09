package netbsd

import (
	"github.com/snonux/conf/gonf/labdns"
	. "github.com/snonux/gonf/api"
)

// Resolver owns pi0/pi1's /etc/resolv.conf (task u54). The bake script wrote
// it once with the ISP CPE 192.168.1.1, which the lab cannot reach after the
// b24 cable move.
//
// The file is static: rc.conf has dhcpcd=NO and there is no
// /etc/resolvconf.conf, so nothing regenerates it. Enabling dhcpcd would
// overwrite it with whatever DHCP hands out. libc and Go reread a changed
// resolv.conf, so nothing needs a restart.
//
// Rollback on one host: write "nameserver 192.168.1.1" back. A later
// pis_netbsd aggregate run re-applies this file.
type Resolver struct{ RequiresRoot }

// Conf writes /etc/resolv.conf with the lab resolver.
func (Resolver) Conf() {
	File("/etc/resolv.conf", WithContent(`# Managed by gonf (pis_netbsd_resolver_conf, task u54): the router's unbound
# on the lab gateway, reachable before and after the b24/c24 cable move.
nameserver `+labdns.Resolver+`
`), RootOwned)
}
