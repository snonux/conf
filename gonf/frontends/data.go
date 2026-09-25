// Package frontends holds the shared, source-controlled frontend topology used
// by the migrated OpenBSD configuration tasks and their templates.
package frontends

import (
	"fmt"
	"strings"
)

const (
	// Domain is the DNS suffix for the two public OpenBSD frontend hosts.
	Domain = "buetow.org"

	// Master and Standby name the active and failover frontend roles used by
	// public service records. They do not select the DNS publication host.
	Master  = "fishfinger"
	Standby = "blowfish"

	// DNSPublisher is the d52 contract's sole stable host for publishing
	// effective NSD zone files and SOA serials. It is independent of
	// Master/Standby service-routing roles; e52 enforces the contract.
	DNSPublisher = "blowfish"
)

// Server is the stable per-frontend data that Rex previously derived from its
// %ips map. gonf/cluster attaches each frontend's Server as host data
// (WithData), which the tasks read with EachHost; it is also suitable for
// WithTemplateData.
type Server struct {
	Name string
	FQDN string
	IPv4 string
	IPv6 string
}

// WireGuardAddress is a frontend-visible WireGuard peer address.
type WireGuardAddress struct {
	Name string
	IPv4 string
	IPv6 string
}

// Data is the shared collection data consumed by frontend configuration
// templates. All fields are JSON-compatible for WithTemplateData.
type Data struct {
	Domain         string
	Prefixes       []string
	AcmeHosts      []string
	F3SHosts       []string
	GarageBuckets  []string
	DNSZones       []string
	DNSZonesRemove []string
}

var servers = map[string]Server{
	"blowfish": {
		Name: "blowfish",
		FQDN: "blowfish." + Domain,
		IPv4: "23.88.35.144",
		IPv6: "2a01:4f8:c17:20f1::42",
	},
	"fishfinger": {
		Name: "fishfinger",
		FQDN: "fishfinger." + Domain,
		IPv4: "46.23.94.99",
		IPv6: "2a03:6000:6f67:624::99",
	},
}

var prefixes = []string{"", "www.", "standby."}

var f3sHosts = []string{
	"f3s.buetow.org",
	"ychat.f3s.buetow.org",
	"player.f3s.buetow.org",
	"xplayer.f3s.buetow.org",
	"pihole.f3s.buetow.org",
	"jellyfin.f3s.buetow.org",
	"navidrome.f3s.buetow.org",
	"code.f3s.buetow.org",
	"immich.f3s.buetow.org",
	"argocd.f3s.buetow.org",
	"keybr.f3s.buetow.org",
	"anki.f3s.buetow.org",
	"bag.f3s.buetow.org",
	"flux.f3s.buetow.org",
	"audiobookshelf.f3s.buetow.org",
	"garage.f3s.buetow.org",
	"radicale.f3s.buetow.org",
	"syncthing.f3s.buetow.org",
	"koreader.f3s.buetow.org",
	"filebrowser.f3s.buetow.org",
	"webdav.f3s.buetow.org",
	"pkgrepo.f3s.buetow.org",
	"goprecords.f3s.buetow.org",
	"bgtutor-mcp.f3s.buetow.org",
	"ipv6test.f3s.buetow.org",
	"ipv4.ipv6test.f3s.buetow.org",
	"ipv6.ipv6test.f3s.buetow.org",
}

var garageBuckets = []string{"taskwarrior", "quicklog"}

// Per-site exceptions to the default site policy (see SiteFor). A site
// without an entry is served over HTTPS on 443 by relayd's generic routing,
// and its HTTPS check accepts any non-error status.
var (
	// siteHTTPSPorts are sites relayd serves on a dedicated TLS port.
	siteHTTPSPorts = map[string]string{"code.f3s.buetow.org": "2443"}
	// siteHTTPSStatus is the status line an HTTPS check must see, for sites
	// that answer an anonymous request with an error by design.
	siteHTTPSStatus = map[string]string{
		"player.f3s.buetow.org":   "HTTP/1.1 401",
		"xplayer.f3s.buetow.org":  "HTTP/1.1 401",
		"webdav.f3s.buetow.org":   "HTTP/1.1 401",
		"koreader.f3s.buetow.org": "HTTP/1.1 412",
		"pihole.f3s.buetow.org":   "HTTP/1.1 404",
		"anki.f3s.buetow.org":     "HTTP/1.1 404",
		"pkgrepo.f3s.buetow.org":  "HTTP/1.1 404",
		// bgtutor serves only /mcp (token required) and /healthz.
		"bgtutor-mcp.f3s.buetow.org": "HTTP/1.1 404",
	}
	// sitesWithoutHTTPSCheck are served but not HTTPS-checked (ychat speaks
	// its own protocol behind the TLS relay).
	sitesWithoutHTTPSCheck = map[string]bool{"ychat.f3s.buetow.org": true}
	// siteRelaydUpstreams are f3s sites relayd forwards to a dedicated
	// table instead of its generic f3s routing.
	siteRelaydUpstreams = map[string]string{
		"f3s.buetow.org":          "f3s_static_proxy",
		"registry.f3s.buetow.org": "f3s_registry",
		"jellyfin.f3s.buetow.org": "f3s_jellyfin",
		"anki.f3s.buetow.org":     "f3s_anki",
	}
)

var acmeHosts = []string{
	"buetow.org",
	"git.buetow.org",
	"paul.buetow.org",
	"dory.buetow.org",
	"ecat.buetow.org",
	"znc.buetow.org",
	"dtail.dev",
	"foo.zone",
	"stats.foo.zone",
	"irregular.ninja",
	"alt.irregular.ninja",
	"snonux.foo",
	"gogios.buetow.org",
	"blowfish.buetow.org",
	"fishfinger.buetow.org",
}

var dnsZones = []string{"buetow.org", "dtail.dev", "foo.zone", "irregular.ninja", "snonux.foo"}

var wireGuardAddresses = []WireGuardAddress{
	{Name: "blowfish", IPv4: "192.168.2.110", IPv6: "fd42:beef:cafe:2::110"},
	{Name: "fishfinger", IPv4: "192.168.2.111", IPv6: "fd42:beef:cafe:2::111"},
	{Name: "r0", IPv4: "192.168.2.120", IPv6: "fd42:beef:cafe:2::120"},
	{Name: "r1", IPv4: "192.168.2.121", IPv6: "fd42:beef:cafe:2::121"},
	{Name: "r2", IPv4: "192.168.2.122", IPv6: "fd42:beef:cafe:2::122"},
	{Name: "rocky", IPv4: "192.168.2.123", IPv6: "fd42:beef:cafe:2::123"},
	{Name: "f0", IPv4: "192.168.2.130", IPv6: "fd42:beef:cafe:2::130"},
	{Name: "f1", IPv4: "192.168.2.131", IPv6: "fd42:beef:cafe:2::131"},
	{Name: "f2", IPv4: "192.168.2.132", IPv6: "fd42:beef:cafe:2::132"},
	{Name: "pi0", IPv4: "192.168.2.203", IPv6: "fd42:beef:cafe:2::203"},
	{Name: "pi1", IPv4: "192.168.2.204", IPv6: "fd42:beef:cafe:2::204"},
	{Name: "earth", IPv4: "192.168.2.200", IPv6: "fd42:beef:cafe:2::200"},
	{Name: "pixel7pro", IPv4: "192.168.2.201", IPv6: "fd42:beef:cafe:2::201"},
}

// Site is one public name of the topology together with the policy every
// consumer derives from it: certificates (acme.go), relayd routing (web.go),
// DNS records (maildns.go) and Gogios checks (monitoring.go). Adding a site,
// f3s service or Garage bucket to the lists above is the only edit needed;
// the exceptions live in the site* tables.
type Site struct {
	Name string
	// Family is 4 or 6 for an ipv4./ipv6. name that resolves over one
	// address family only; such a name is an alternative name of its
	// parent site's certificate, never a certificate or check target of its
	// own apart from its plain HTTP check. It is 0 for a dual-stack name.
	Family int
	// FrontendHost marks a frontend's own FQDN: a host certificate, with no
	// www./standby. variants and no site checks.
	FrontendHost bool
	// F3S marks a site served by the f3s cluster; its checks pause while
	// the cluster is deliberately taken down.
	F3S bool
	// HTTPSPort is the dedicated TLS port, "" for 443.
	HTTPSPort string
	// HTTPSStatus is the status line the HTTPS check expects, "" for any.
	HTTPSStatus string
	// HTTPSCheck is false for a site that is served but not HTTPS-checked.
	HTTPSCheck bool
	// RelaydUpstream is the relayd table a dedicated f3s route forwards to,
	// "" when the generic routing applies.
	RelaydUpstream string
}

// SiteFor returns the policy of name, derived from the topology lists and
// the site* exception tables. It does not check that name is listed: a name
// outside the lists gets the default policy (dual-stack, not f3s, port 443,
// any HTTPS status, HTTPS-checked, generic routing), and callers pass only
// topology names (Sites, F3SHosts).
func SiteFor(name string) Site {
	site := Site{
		Name:           name,
		F3S:            contains(TemplateData().F3SHosts, name),
		HTTPSPort:      siteHTTPSPorts[name],
		HTTPSStatus:    siteHTTPSStatus[name],
		HTTPSCheck:     !sitesWithoutHTTPSCheck[name],
		RelaydUpstream: siteRelaydUpstreams[name],
	}
	switch {
	case strings.HasPrefix(name, "ipv4."):
		site.Family = 4
	case strings.HasPrefix(name, "ipv6."):
		site.Family = 6
	}
	for _, server := range servers {
		if name == server.FQDN {
			site.FrontendHost = true
		}
	}
	// Garage answers anonymous S3 requests with 403, and every bucket is a
	// virtual-host site of the one Garage upstream.
	if name == "garage.f3s.buetow.org" || strings.HasSuffix(name, ".garage.f3s.buetow.org") {
		site.HTTPSStatus = "HTTP/1.1 403"
		site.RelaydUpstream = "garage"
	}
	return site
}

// Sites returns the policy of every ACME host, in topology order: the
// public sites, the frontend FQDNs, then the f3s services and buckets.
func Sites() []Site {
	hosts := TemplateData().AcmeHosts
	sites := make([]Site, 0, len(hosts))
	for _, host := range hosts {
		sites = append(sites, SiteFor(host))
	}
	return sites
}

// ServerFor returns one frontend's stable addressing data.
func ServerFor(name string) (Server, bool) {
	server, ok := servers[name]
	return server, ok
}

// MustServer returns one frontend's stable addressing data. An unknown name
// is a programmer error in static consumer inventory, so it panics rather
// than reporting a declaration error: servers is a compile-time map and
// every caller passes a package constant (Master, Standby, DNSPublisher) or
// a literal server name (gonf/cluster), never recipe or input data, so no
// recipe run or input file can reach the panic (see gonf's AGENTS.md,
// "Registration-time contract"). A lookup driven by input data would use
// ServerFor instead and report its own declaration error.
func MustServer(name string) Server {
	server, ok := ServerFor(name)
	if !ok {
		panic(fmt.Sprintf("unknown frontend server %q", name))
	}
	return server
}

// TemplateData returns independent slices so callers may safely derive a
// task-specific template payload without mutating the shared topology.
func TemplateData() Data {
	f3s := append([]string(nil), f3sHosts...)
	for _, bucket := range garageBuckets {
		f3s = append(f3s, bucket+".garage.f3s.buetow.org")
	}
	acme := append([]string(nil), acmeHosts...)
	acme = append(acme, f3s...)
	return Data{
		Domain:         Domain,
		Prefixes:       append([]string(nil), prefixes...),
		AcmeHosts:      acme,
		F3SHosts:       f3s,
		GarageBuckets:  append([]string(nil), garageBuckets...),
		DNSZones:       append([]string(nil), dnsZones...),
		DNSZonesRemove: []string{},
	}
}

// WireGuardAddresses returns independent peer rows for /etc/hosts and
// monitoring configuration.
func WireGuardAddresses() []WireGuardAddress {
	return append([]WireGuardAddress(nil), wireGuardAddresses...)
}

// WireGuardHostLines returns the legacy /etc/hosts rows in their established
// IPv4-then-IPv6 order. Consumers append these lines instead of replacing
// administrator-owned host entries.
func WireGuardHostLines() []string {
	peers := WireGuardAddresses()
	lines := make([]string, 0, len(peers)*2)
	for _, peer := range peers {
		lines = append(lines, peer.IPv4+" "+peer.Name+".wg0.wan.buetow.org "+peer.Name+".wg0")
	}
	for _, peer := range peers {
		lines = append(lines, peer.IPv6+" "+peer.Name+".wg0.wan.buetow.org "+peer.Name+".wg0")
	}
	return lines
}
