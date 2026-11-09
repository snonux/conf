package freebsd

import (
	"fmt"

	"github.com/snonux/conf/gonf/labdns"
	. "github.com/snonux/gonf/api"
)

// Resolver owns a FreeBSD host's nameserver. Until 2026-11-09
// /etc/resolvconf.conf was hand-made and named the ISP CPE 192.168.1.1, which
// the lab cannot reach after the b24 cable move.
//
// It is registered twice (gonf/tasks/tasks.go), because the two FreeBSD
// clusters differ in their LAN interface:
//   - freebsd_resolver_conf on the f-hosts (task u54), Interface "re0";
//   - freebsd_guest_resolver_conf on the normally stopped standalone guest
//     "freebsd" (task v54), Interface "vtnet0". Push it when the guest is
//     next started; see IQ topics/infra/f3s/network/host-dns.md.
//
// resolvconf(8) owns /etc/resolv.conf: on the f-hosts no dhclient runs (re0
// is static) and rtsold is off (-accept_rtadv), so name_servers is the only
// source and "resolvconf -u" regenerates the file at once. private_interfaces
// keeps any RA RDNSS (or DHCP) servers learned on Interface out of the global
// list, as before; on the guest this is the SLAAC fix it already carried
// (IQ topics/infra/f3s/vms/rocky-linux-vms.md, "Slow SSH login").
//
// Rollback on one host: name_servers="192.168.1.1" in /etc/resolvconf.conf,
// then resolvconf -u. A later run of the owning task re-applies this file.
type Resolver struct {
	RequiresRoot
	// Interface is the LAN interface named in private_interfaces.
	Interface string
	// Owner names the gonf task and the conf task that introduced it, for
	// the "Managed by gonf (...)" header of the rendered file.
	Owner string
}

const resolvconfConf = "/etc/resolvconf.conf"

// Conf writes /etc/resolvconf.conf with the lab resolver and regenerates
// /etc/resolv.conf.
func (r Resolver) Conf() {
	if r.Interface == "" || r.Owner == "" {
		// A zero Resolver would render private_interfaces="" and an empty
		// header; refuse at record time rather than push that.
		Refuse("Resolver", resolvconfConf, fmt.Errorf("resolver needs both Interface and Owner"))
		return
	}
	conf := File(resolvconfConf, WithContent(r.render()), RootOwned)
	Sh("resolvconf -u", OnChange(conf))
}

// render returns the resolvconf.conf content. The f-host rendering (Owner
// "freebsd_resolver_conf, task u54", Interface "re0") must stay byte-identical
// to what u54 deployed, so the f-hosts see no change.
func (r Resolver) render() string {
	return `# Managed by gonf (` + r.Owner + `). Static nameserver: the
# router's unbound on the lab gateway, reachable before and after the b24/c24
# cable move. Mark ` + r.Interface + ` as private so SLAAC-provided IPv6 DNS (Router
# Advertisement RDNSS) is not added globally.
name_servers="` + labdns.Resolver + `"
private_interfaces="` + r.Interface + `"
`
}
