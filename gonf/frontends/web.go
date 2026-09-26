package frontends

import (
	"fmt"
	"path/filepath"

	. "github.com/snonux/gonf/api"
)

// webConfigData is the controller-side input to the frontend web and relay
// renderers. Keeping the per-host addresses and the shared topology together
// makes a plan carry rendered configuration rather than a destination-side
// template that could observe different controller facts.
type webConfigData struct {
	Server    Server
	Prefixes  []string
	AcmeHosts []string
	F3SHosts  []string
}

// Web contains the web-facing part of the OpenBSD frontend recipe. It is kept
// separate from Maintenance because its changes affect public listeners and
// consequently require syntax checks before a daemon can be restarted.
type Web struct {
	RequiresRoot
}

// OptsACMEInvoke marks the certificate request as an Operational action, so
// no pattern aggregate can ever pick it up by name.
func (Web) OptsACMEInvoke() TaskOptions { return TaskOptions{Operational()} }

// ACMEInvoke requests and renews frontend ACME certificates.
//
// ACMEInvoke runs the already-installed renewal script on explicit request,
// matching Rex's separate acme_invoke task. It is not part of the aggregate
// setup path, so adding configuration never unexpectedly contacts an ACME CA.
func (Web) ACMEInvoke() {
	Sh("/usr/local/bin/acme.sh")
}

// DescHTTPD returns the description for the OpenBSD httpd recipe.
func (Web) DescHTTPD() string { return "Render, validate, and converge frontend httpd" }

// HTTPD renders each host's configuration on the controller. The core File
// validation (WithValidation) checks a private candidate with `httpd -n`
// before every non-dry-run live reconciliation, while OnChange limits a
// restart to a changed live config; a changed rc flag (WithFlags("") keeps
// the httpd_flags= line rcctl writes) restarts it too. A render failure
// refuses the File (WithContentFrom) instead of declaring it with empty
// content, which fails the record.
func (Web) HTTPD() {
	EachHost(func(server Server) {
		NoFile(legacyCandidate("/etc/httpd.conf"))
		config := File("/etc/httpd.conf", WithContentFrom(renderHTTPD(webData(server))),
			RootOwned,
			WithValidation("httpd", List("-n", "-f", CandidatePath)))
		fallbackIndex := htdocs(server)
		Service("httpd", WithFlags(""), WithRestart, DependsOn(fallbackIndex), OnChange(config))
	})
}

// Inetd installs and converges frontend inetd.
//
// Inetd renders no host-specific content, but still treats its login class as
// a change input because the daemon must re-exec to receive revised limits.
// WithFlags("") keeps the inetd_flags= line that enables it.
// LoginClass installs the root:wheel 0644 fragment under an OpenBSD-only plan
// requirement; OpenBSD reads /etc/login.conf.d/<class> directly, so no
// cap_mkdb step is needed (cap_mkdb /etc/login.conf never reads fragments).
// The inetd fragment only adds maxproc=10 and inherits the rest through
// tc=daemon, which resolves against /etc/login.conf (not other fragments), so
// it builds on the full stock daemon class and is kept as is.
func (Web) Inetd() {
	class := LoginClass("inetd", legacyFrontendAsset("etc/login.conf.d/inetd"))
	config := InstallFile("/etc/inetd.conf", legacyFrontendAsset("etc/inetd.conf"), RootOwned)
	Service("inetd", WithFlags(""), WithRestart, OnChange(class, config))
}

// DescRelayd returns the description for the TLS relay recipe.
func (Web) DescRelayd() string {
	return "Render, validate, and converge frontend relayd (needs /etc/ssl certificates from a frontends_acme_invoke run)"
}

// OptsRelayd records the ACME setup (frontends_acme) before relayd; see
// MailDNS.OptsSMTPD for why the Operational frontends_acme_invoke is not a
// need.
func (Web) OptsRelayd() TaskOptions { return TaskOptions{Needs(Maintenance.ACME)} }

// Relayd validates a candidate with `relayd -n` (core WithValidation) before
// changing its live configuration. Its daemon login class is watched too: a
// change to the descriptor limits relayd runs with does not change
// relayd.conf but must take effect through a restart.
//
// relayd's class is the stock daemon entry of /etc/login.conf, which on both
// frontends carries the manual openfiles-max/cur=4096 edit documented in
// frontends/AGENTS.md. That entry is authoritative and deliberately NOT
// managed by gonf. The former /etc/login.conf.d/daemon fragment (only
// openfiles 4096 plus tc=default) replaced the whole class and so dropped
// ignorenologin, datasize=4096M, maxproc=infinity and stacksize-cur=8M; on
// the owner's decision of 2026-09-22 it is removed instead of installed.
// NoLoginClass deletes the fragment and any stale daemon.db beside it, and
// either removal is a class change that restarts relayd once, so the
// running daemon picks up the login.conf class. Afterwards the handle is
// unchanged and restarts nothing. No database rebuild is needed: the
// removal does not touch /etc/login.conf or /etc/login.conf.db.
//
// As in HTTPD, WithFlags("") keeps the relayd_flags= line, and a render
// failure refuses the File.
func (Web) Relayd() {
	EachHost(func(server Server) {
		class := NoLoginClass("daemon")
		NoFile(legacyCandidate("/etc/relayd.conf"))
		config := File("/etc/relayd.conf", WithContentFrom(renderRelayd(webData(server))),
			RootPrivate,
			WithValidation("relayd", List("-n", "-f", CandidatePath)))
		Service("relayd", WithFlags(""), WithRestart, OnChange(class, config))
		File(dailyLocal, WithLine("/usr/sbin/rcctl start relayd"),
			RootOwned)
	})
}

// PF validates and reloads frontend PF plus its node_exporter metrics.
//
// PF validates a new ruleset with `pfctl -n` against a private candidate
// (core WithValidation) before it replaces /etc/pf.conf, so an invalid
// ruleset never becomes the live file and pf-reload never loads it. The
// reload, node_exporter restart, and exporter cron entry are all declarative
// and change-gated. node_exporter's flags carry each host's own WireGuard
// address, so the task iterates the hosts.
func (Web) PF() {
	EachHost(func(server Server) { pfAndExporter(server.Name) })
}

// pfAndExporter declares host's validated PF ruleset and reload, and the
// pf-labels exporter feeding node_exporter's textfile collector. A host
// missing from the WireGuard inventory has no listen address for
// node_exporter: its Service is refused as a declaration error (which fails
// the record) and not declared; the PF and exporter resources, which do not
// need the address, are still declared and checked.
func pfAndExporter(host string) {
	config := InstallFile("/etc/pf.conf", legacyFrontendAsset("etc/pf.conf.tpl"),
		RootPrivate, WithValidation("pfctl", List("-n", "-f", CandidatePath)))
	Sh("pfctl -f /etc/pf.conf", OnChange(config), WithName("pf-reload"))

	collector := Dir("/var/node_exporter", RootOwned)
	exporter := InstallFile("/usr/local/bin/pf-labels-exporter.sh", legacyFrontendAsset("scripts/pf-labels-exporter.sh"),
		Perm(0o500, Root))
	// pfctl needs root, so the exporter runs from root's crontab.
	CronAt("frontend-pf-labels-exporter", "* * * * *", "-ns /usr/local/bin/pf-labels-exporter.sh",
		DependsOn(collector, exporter))
	flags, err := nodeExporterFlags(host)
	if err != nil {
		Refuse("Service", "node_exporter", err)
		return
	}
	Service("node_exporter", WithFlags(flags), WithRestart, DependsOn(collector, exporter), OnChange(exporter))
}

// nodeExporterFlags returns node_exporter's rc flags, set with `rcctl set
// node_exporter flags` (WithFlags): listening on the host's WireGuard IPv4
// from the inventory. rcctl stores them unquoted as the node_exporter_flags
// line of /etc/rc.conf.local. The former literal line kept a
// `$(ifconfig wg0 ...)` substitution, which rc(8) does not expand. A host
// without an inventory address is returned as an error for the caller to
// report.
func nodeExporterFlags(host string) (string, error) {
	peer, err := wireGuardAddressFor(host)
	if err != nil {
		return "", err
	}
	return "--web.listen-address=" + peer.IPv4 +
		":9100 --collector.textfile.directory=/var/node_exporter", nil
}

// htdocs declares the httpd document roots and their static files, and
// returns the fallback index the httpd service waits for. Ownership matches
// both frontends as they are (Rex never set any): buetow.org is admin:daemon
// (admin owns its tmp/ subdirectory), self and f3s_fallback are root:daemon,
// and self/index.txt is rex:wheel. httpd (www) only reads, which the
// world-readable modes allow regardless of owner; Gogios writes solely into
// its own self/gogios (_gogios, declared by the Gogios task) and Foostats
// runs as root. Re-owning them to root:wheel would only churn the hosts.
// The files apply after their Dir without DependsOn (gonf orders a path
// after its parent).
func htdocs(server Server) Resource {
	Dir("/var/www/htdocs/buetow.org", Perm(0o755, "admin:daemon"))
	Dir("/var/www/htdocs/buetow.org/self", Perm(0o755, "root:daemon"))
	Dir("/var/www/htdocs/f3s_fallback", Perm(0o755, "root:daemon"))
	fallbackIndex := InstallFile("/var/www/htdocs/f3s_fallback/index.html",
		legacyFrontendAsset("var/www/htdocs/f3s_fallback/index.html"),
		RootOwned)
	File("/var/www/htdocs/buetow.org/self/index.txt", WithContent("Welcome to "+server.FQDN+"!\n"),
		Perm(0o644, "rex:wheel"))
	return fallbackIndex
}

func webData(server Server) webConfigData {
	topology := TemplateData()
	return webConfigData{
		Server:    server,
		Prefixes:  topology.Prefixes,
		AcmeHosts: topology.AcmeHosts,
		F3SHosts:  topology.F3SHosts,
	}
}

// legacyCandidate returns the fixed /var/tmp candidate path the former
// consumer-side validation staged for live file path. Core WithValidation
// candidates are private, per apply and removed afterwards, so HTTPD and Relayd
// delete these leftovers (a no-op once gone). Drop the cleanup when no host
// has them any more.
func legacyCandidate(path string) string {
	return filepath.Join("/var/tmp", "gonf-"+filepath.Base(path))
}

// renderHTTPD renders /etc/httpd.conf on the controller from the native
// httpd.conf.tmpl asset (frontendAsset, package assets/) with the typed,
// already-prefixed httpdTemplateData built from data. Rendering happens
// here, at recipe-declaration time, not on the destination: the resulting
// text is handed to WithContent like any other literal string, preserving
// the pre-P4 controller-render semantics (see api.RenderTemplate's doc
// comment for why that differs from a destination-rendered
// WithTemplateData file). A read/parse/render failure is returned, wrapped
// with the template's name, for the caller to report (refuseRender).
func renderHTTPD(data webConfigData) (string, error) {
	rendered, err := RenderTemplate(frontendAsset("httpd.conf.tmpl"), httpdTemplateDataFor(data))
	if err != nil {
		return "", fmt.Errorf("render httpd.conf.tmpl: %w", err)
	}
	return rendered, nil
}

// refuseRender reports a failed controller render of the live file path as
// a gonf declaration error through Refuse (File[path] is the
// resource left undeclared), instead of panicking: gonf's contract is that
// recipe and input errors never end the process. While a plan is recorded
// the error fails that record with the Refuse line below as its
// location (the CLI prints both and exits 1); a panic would instead bypass
// gonf's error path and dump a raw stack trace to stderr. The caller then
// declares none of the host's resources that depend on the rendered file,
// so nothing is registered with empty content, while the rest of the
// recipe keeps running and later declarations are still checked.
func refuseRender(path string, err error) {
	Refuse("File", path, err)
}

// httpdTemplateData is httpd.conf.tmpl's typed root. Every field is either
// a plain value or an already-expanded, already-prefixed slice (or slice of
// small structs), so the template itself only ranges and branches over
// explicit data — it never encodes the site catalogue, the fixed root-path
// table, or any prefix concatenation itself.
type httpdTemplateData struct {
	Port80Hosts []string
	ServerFQDN  string
	Gemtexter   []httpdGemtexterEntry
	Redirects   []httpdRedirectEntry
	DtailHosts  []string
	RootHosts   []httpdRootEntry
	F3SBlocks   []string
}

// httpdGemtexterEntry is one gemtexter server block: Name is the already
// prefixed label, Host the bare site name used in its non-www document
// root, and IsWWW selects the www. variant's plain redirect instead.
type httpdGemtexterEntry struct {
	Name  string
	Host  string
	IsWWW bool
}

// httpdRedirectEntry is one prefix's row of the four fixed redirect/landing
// blocks (buetow.org, blog.buetow.org, snonux.foo, paul.buetow.org).
// IsWWW selects the snonux.foo block's plain redirect instead of its
// default f3s-fallback rewrite.
type httpdRedirectEntry struct {
	BuetowOrg     string
	BlogBuetowOrg string
	SnonuxFoo     string
	IsWWW         bool
	PaulBuetowOrg string
}

// httpdRootEntry pairs a server label with the document root it serves:
// used both for the fixed, non-topology host table below (bare Name) and
// for its per-prefix expansion (prefixed Name).
type httpdRootEntry struct {
	Name string
	Root string
}

func httpdTemplateDataFor(data webConfigData) httpdTemplateData {
	return httpdTemplateData{
		Port80Hosts: httpdPort80Hosts(data),
		ServerFQDN:  data.Server.FQDN,
		Gemtexter:   httpdGemtexterEntries(data.Prefixes),
		Redirects:   httpdRedirectEntries(data.Prefixes),
		DtailHosts:  prefixed(data.Prefixes, "dtail.dev"),
		RootHosts:   httpdRootEntries(data.Prefixes),
		F3SBlocks:   httpdF3SBlocks(data),
	}
}

// prefixed returns host prefixed by every one of prefixes, in order —
// the shared "prefix + host" expansion every httpd/relayd data list uses.
func prefixed(prefixes []string, host string) []string {
	out := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		out = append(out, prefix+host)
	}
	return out
}

// httpdPort80Hosts lists every ACME host except the current server's own
// FQDN (which gets its own fixed block, not the loop's), each expanded
// across every configured prefix.
func httpdPort80Hosts(data webConfigData) []string {
	var hosts []string
	for _, host := range data.AcmeHosts {
		if host == data.Server.FQDN {
			continue
		}
		hosts = append(hosts, prefixed(data.Prefixes, host)...)
	}
	return hosts
}

func httpdGemtexterEntries(prefixes []string) []httpdGemtexterEntry {
	var entries []httpdGemtexterEntry
	for _, host := range []string{"foo.zone", "stats.foo.zone"} {
		for _, prefix := range prefixes {
			entries = append(entries, httpdGemtexterEntry{Name: prefix + host, Host: host, IsWWW: prefix == "www."})
		}
	}
	return entries
}

func httpdRedirectEntries(prefixes []string) []httpdRedirectEntry {
	var entries []httpdRedirectEntry
	for _, prefix := range prefixes {
		entries = append(entries, httpdRedirectEntry{
			BuetowOrg:     prefix + "buetow.org",
			BlogBuetowOrg: prefix + "blog.buetow.org",
			SnonuxFoo:     prefix + "snonux.foo",
			IsWWW:         prefix == "www.",
			PaulBuetowOrg: prefix + "paul.buetow.org",
		})
	}
	return entries
}

// httpdRootHosts are the fixed, non-topology single-purpose hosts and their
// document roots (formerly appendHTTPDSpecialHosts's inline root map).
var httpdRootHosts = []httpdRootEntry{
	{Name: "irregular.ninja", Root: "/htdocs/irregular.ninja"},
	{Name: "alt.irregular.ninja", Root: "/htdocs/alt.irregular.ninja"},
	{Name: "joern.buetow.org", Root: "/htdocs/joern/"},
	{Name: "dory.buetow.org", Root: "/htdocs/joern/dory.buetow.org"},
	{Name: "ecat.buetow.org", Root: "/htdocs/joern/ecat.buetow.org"},
	{Name: "gogios.buetow.org", Root: "/htdocs/buetow.org/self/gogios"},
}

// httpdRootEntries expands httpdRootHosts across every prefix, host outer
// and prefix inner, matching the original grouped-by-host block order.
func httpdRootEntries(prefixes []string) []httpdRootEntry {
	var entries []httpdRootEntry
	for _, host := range httpdRootHosts {
		for _, prefix := range prefixes {
			entries = append(entries, httpdRootEntry{Name: prefix + host.Name, Root: host.Root})
		}
	}
	return entries
}

func httpdF3SBlocks(data webConfigData) []string {
	var hosts []string
	for _, host := range data.F3SHosts {
		hosts = append(hosts, prefixed(data.Prefixes, host)...)
	}
	return hosts
}

// renderRelayd renders /etc/relayd.conf on the controller from the native
// relayd.conf.tmpl asset with the typed relaydTemplateData built from data.
// See renderHTTPD above for why this stays a controller render (WithContent,
// not a destination-rendered WithTemplateData file) and how a render
// failure is returned.
func renderRelayd(data webConfigData) (string, error) {
	rendered, err := RenderTemplate(frontendAsset("relayd.conf.tmpl"), relaydTemplateDataFor(data))
	if err != nil {
		return "", fmt.Errorf("render relayd.conf.tmpl: %w", err)
	}
	return rendered, nil
}

// relaydTemplateData is relayd.conf.tmpl's typed root; see httpdTemplateData
// above for the same "already expanded, already prefixed" convention.
type relaydTemplateData struct {
	KeypairNames   []string
	ServerFQDN     string
	CodeBlocks     []string
	LocalhostHosts []string
	F3SRoutes      []relaydF3SRoute
	ServerIPv4     string
	ServerIPv6     string
}

// relaydF3SRoute is one prefixed host routed to a dedicated f3s upstream
// table (Site.RelaydUpstream), rather than the generic f3s forwarding.
type relaydF3SRoute struct {
	Host     string
	Upstream string
}

func relaydTemplateDataFor(data webConfigData) relaydTemplateData {
	return relaydTemplateData{
		KeypairNames:   relaydKeypairNames(),
		ServerFQDN:     data.Server.FQDN,
		CodeBlocks:     prefixed(data.Prefixes, "code.f3s.buetow.org"),
		LocalhostHosts: relaydLocalhostHosts(data),
		F3SRoutes:      relaydF3SRoutes(data),
		ServerIPv4:     data.Server.IPv4,
		ServerIPv6:     data.Server.IPv6,
	}
}

// relaydKeypairNames are the ACME site certificates and their standby
// twins (acmeSites, the list acme.sh requests); the server's own FQDN
// keypair is a fixed line the template adds itself.
func relaydKeypairNames() []string {
	var names []string
	for _, site := range acmeSites(Sites()) {
		names = append(names, site.Name)
	}
	return names
}

// relaydLocalhostHosts are every ACME host routed to the generic <localhost>
// upstream: neither an f3s-cluster host (which gets its own routing, see
// relaydF3SRoutes) nor snonux.foo (which the template routes explicitly).
func relaydLocalhostHosts(data webConfigData) []string {
	var hosts []string
	for _, host := range data.AcmeHosts {
		if contains(data.F3SHosts, host) || host == "snonux.foo" {
			continue
		}
		hosts = append(hosts, prefixed(data.Prefixes, host)...)
	}
	return hosts
}

// relaydF3SRoutes lists only the f3s sites with a dedicated upstream
// (Site.RelaydUpstream); the rest use the generic f3s forwarding the
// relay blocks already provide.
func relaydF3SRoutes(data webConfigData) []relaydF3SRoute {
	var routes []relaydF3SRoute
	for _, host := range data.F3SHosts {
		upstream := SiteFor(host).RelaydUpstream
		if upstream == "" {
			continue
		}
		for _, prefix := range data.Prefixes {
			routes = append(routes, relaydF3SRoute{Host: prefix + host, Upstream: upstream})
		}
	}
	return routes
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
