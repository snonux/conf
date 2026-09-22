package frontends

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"
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

// DescACMEInvoke returns the description for the explicit certificate request
// task. Certificate requests intentionally remain separate from setup.
func (Web) DescACMEInvoke() string { return "Request and renew frontend ACME certificates" }

// ACMEInvoke runs the already-installed renewal script on explicit request,
// matching Rex's separate acme_invoke task. It is not part of the aggregate
// setup path, so adding configuration never unexpectedly contacts an ACME CA.
func (Web) ACMEInvoke() {
	onFrontends(func() { Command("/usr/local/bin/acme.sh", List()) })
}

// DescHTTPD returns the description for the OpenBSD httpd recipe.
func (Web) DescHTTPD() string { return "Render, validate, and converge frontend httpd" }

// HTTPD renders each host's configuration on the controller. Candidate
// validation precedes its live configuration update, while OnChange limits a
// restart to a changed live config or rc flag.
func (Web) HTTPD() {
	for _, host := range ClusterHosts() {
		server := MustHostValue[Server](host, ValueServer)
		WhenHostname(host, func() {
			flags := File("/etc/rc.conf.local", WithLine("httpd_flags="), WithName("rc-conf-httpd-flags"))
			config := validatedConfig("/etc/httpd.conf", "httpd", List("-n", "-f"), renderHTTPD(webData(server)), 0o644)
			Dir("/var/www/htdocs/buetow.org", WithMode(0o755), WithOwner("root"), WithGroup("wheel"))
			self := Dir("/var/www/htdocs/buetow.org/self", WithMode(0o755), WithOwner("root"), WithGroup("wheel"))
			fallback := Dir("/var/www/htdocs/f3s_fallback", WithMode(0o755), WithOwner("root"), WithGroup("wheel"))
			fallbackIndex := InstallFile("/var/www/htdocs/f3s_fallback/index.html",
				legacyFrontendAsset("var/www/htdocs/f3s_fallback/index.html"),
				WithMode(0o644), WithOwner("root"), WithGroup("wheel"), DependsOn(fallback))
			File("/var/www/htdocs/buetow.org/self/index.txt", WithContent("Welcome to "+server.FQDN+"!\n"),
				WithMode(0o644), WithOwner("root"), WithGroup("wheel"), DependsOn(self))
			Service("httpd", WithRestart, DependsOn(fallbackIndex), OnChange(flags, config))
		})
	}
}

// DescInetd returns the description for the inetd recipe.
func (Web) DescInetd() string { return "Install and converge frontend inetd" }

// Inetd renders no host-specific content, but still treats its login class as
// a change input because the daemon must re-exec to receive revised limits.
// LoginClass installs the root:wheel 0644 fragment under an OpenBSD-only plan
// requirement; OpenBSD reads /etc/login.conf.d/<class> directly, so no
// cap_mkdb step is needed (cap_mkdb /etc/login.conf never reads fragments).
func (Web) Inetd() {
	onFrontends(func() {
		flags := File("/etc/rc.conf.local", WithLine("inetd_flags="), WithName("rc-conf-inetd-flags"))
		class := LoginClass("inetd", legacyFrontendAsset("etc/login.conf.d/inetd"))
		config := InstallFile("/etc/inetd.conf", legacyFrontendAsset("etc/inetd.conf"),
			WithMode(0o644), WithOwner("root"), WithGroup("wheel"))
		Service("inetd", WithRestart, OnChange(flags, class, config))
	})
}

// DescRelayd returns the description for the TLS relay recipe.
func (Web) DescRelayd() string { return "Render, validate, and converge frontend relayd" }

// Relayd validates a candidate before changing its live configuration. Its
// daemon login class is watched too: raising relayd's descriptor limit does
// not change relayd.conf but must take effect through a restart. The login
// fragments belong to the consumers that need them, so a frontends aggregate
// has one change-gate source per privilege chunk. As in Inetd, LoginClass
// needs no database rebuild; the former cap_mkdb step rebuilt only
// /etc/login.conf.db from an unchanged /etc/login.conf.
func (Web) Relayd() {
	for _, host := range ClusterHosts() {
		server := MustHostValue[Server](host, ValueServer)
		WhenHostname(host, func() {
			flags := File("/etc/rc.conf.local", WithLine("relayd_flags="), WithName("rc-conf-relayd-flags"))
			// TODO(login-class, needs operator decision): etc/login.conf.d/daemon
			// replaces OpenBSD's whole stock daemon class, and it only sets
			// openfiles-max/cur=4096 plus tc=default. Unlike the limits
			// confirmed in frontends/AGENTS.md (edited into /etc/login.conf), it
			// therefore drops ignorenologin, datasize=4096M and maxproc=infinity
			// for relayd and every other daemon-class process. The content is
			// deliberately unchanged here; review it before the next rollout.
			class := LoginClass("daemon", legacyFrontendAsset("etc/login.conf.d/daemon"))
			config := validatedConfig("/etc/relayd.conf", "relayd", List("-n", "-f"), renderRelayd(webData(server)), 0o600)
			Service("relayd", WithRestart, OnChange(flags, class, config))
			File(dailyLocal, WithLine("/usr/sbin/rcctl start relayd"),
				WithMode(0o644), WithOwner("root"), WithGroup("wheel"))
		})
	}
}

// DescPF returns the description for the frontend PF and exporter recipe.
func (Web) DescPF() string { return "Validate and reload frontend PF plus its node_exporter metrics" }

// PF validates a new ruleset before loading it. The reload, node_exporter
// restart, and exporter cron entry are all declarative and change-gated.
func (Web) PF() {
	onFrontends(func() {
		config := InstallFile("/etc/pf.conf", legacyFrontendAsset("etc/pf.conf.tpl"),
			WithMode(0o600), WithOwner("root"), WithGroup("wheel"))
		check := Command("pfctl", List("-n", "-f", "/etc/pf.conf"), OnChange(config), WithName("pf-config-check"))
		Command("pfctl", List("-f", "/etc/pf.conf"), DependsOn(check), OnChange(config), WithName("pf-reload"))

		collector := Dir("/var/node_exporter", WithMode(0o755), WithOwner("root"), WithGroup("wheel"))
		exporter := InstallFile("/usr/local/bin/pf-labels-exporter.sh", legacyFrontendAsset("scripts/pf-labels-exporter.sh"),
			WithMode(0o500), WithOwner("root"), WithGroup("wheel"))
		// pfctl needs root, so the exporter runs from root's crontab.
		Cron("frontend-pf-labels-exporter", WithCommand("-ns /usr/local/bin/pf-labels-exporter.sh"),
			WithLegacyCommand("-ns /usr/local/bin/pf-labels-exporter.sh"), WithMinute("*"), DependsOn(collector, exporter))
		flags := File("/etc/rc.conf.local", WithLine(nodeExporterFlags), WithMode(0o644), WithOwner("root"), WithGroup("wheel"), WithName("rc-conf-node-exporter-flags"))
		Service("node_exporter", WithRestart, DependsOn(collector, exporter), OnChange(flags, exporter))
	})
}

const nodeExporterFlags = `node_exporter_flags="--web.listen-address=$(ifconfig wg0 | awk '/inet /{print $2}'):9100 --collector.textfile.directory=/var/node_exporter"`

func webData(server Server) webConfigData {
	topology := TemplateData()
	return webConfigData{
		Server:    server,
		Prefixes:  topology.Prefixes,
		AcmeHosts: topology.AcmeHosts,
		F3SHosts:  topology.F3SHosts,
	}
}

// validatedConfig stages a complete candidate outside the live configuration
// path, validates it on every apply, and only then permits the managed file to
// be updated. Keeping the candidate means a manually altered live file is
// still validated before Gonf repairs it on a later run. The candidate is
// root-readable only because these configuration formats may later gain
// sensitive material.
func validatedConfig(path, validator string, validatorArgs []string, content string, mode os.FileMode) Resource {
	candidate := filepath.Join("/var/tmp", "gonf-"+filepath.Base(path))
	staged := File(candidate, WithContent(content), WithMode(mode), WithOwner("root"), WithGroup("wheel"))
	args := append(append([]string{}, validatorArgs...), candidate)
	check := Command(validator, args, DependsOn(staged), WithName("validate-"+filepath.Base(path)))
	return File(path, WithContent(content), WithMode(mode), WithOwner("root"), WithGroup("wheel"), DependsOn(check))
}

func appendf(builder *strings.Builder, format string, args ...any) {
	_, _ = fmt.Fprintf(builder, format, args...)
}

func appendString(builder *strings.Builder, value string) {
	_, _ = builder.WriteString(value)
}

func renderHTTPD(data webConfigData) string {
	var builder strings.Builder
	appendString(&builder, "# Plain HTTP for ACME and HTTPS redirect\n")
	for _, host := range data.AcmeHosts {
		if host == data.Server.FQDN {
			continue
		}
		for _, prefix := range data.Prefixes {
			appendHTTPDPort80(&builder, prefix+host)
		}
	}
	appendf(&builder, httpdServerBlocks, data.Server.FQDN, data.Server.FQDN, data.Server.FQDN)
	appendHTTPDGemtexter(&builder, data.Prefixes)
	appendHTTPDSpecialHosts(&builder, data.Prefixes)
	for _, host := range data.F3SHosts {
		for _, prefix := range data.Prefixes {
			appendHTTPDF3S(&builder, prefix+host)
		}
	}
	appendString(&builder, httpdDefaults)
	return builder.String()
}

func appendHTTPDPort80(builder *strings.Builder, host string) {
	appendf(builder, `server %q {
  listen on * port 80
  log style forwarded
  location "/.well-known/acme-challenge/*" {
    root "/acme"
    request strip 2
  }
  location * {
    block return 302 "https://$HTTP_HOST$REQUEST_URI"
  }
}
`, host)
}

func appendHTTPDGemtexter(builder *strings.Builder, prefixes []string) {
	for _, host := range []string{"foo.zone", "stats.foo.zone"} {
		for _, prefix := range prefixes {
			name := prefix + host
			appendf(builder, "server %q {\n  listen on * port 8080\n  log style forwarded\n  location \"/.git*\" {\n    block return 302 \"https://%s\"\n  }\n  location * {\n", name, name)
			if prefix == "www." {
				appendf(builder, "    block return 302 \"https://%s$REQUEST_URI\"\n", host)
			} else {
				appendf(builder, "    root \"/htdocs/gemtexter/%s\"\n    directory auto index\n", host)
			}
			appendString(builder, "  }\n}\n")
		}
	}
}

func appendHTTPDSpecialHosts(builder *strings.Builder, prefixes []string) {
	for _, prefix := range prefixes {
		snonux := "    request rewrite \"/index.html\"\n    root \"/htdocs/f3s_fallback\"\n"
		if prefix == "www." {
			snonux = "    block return 302 \"https://snonux.foo$REQUEST_URI\"\n"
		}
		appendf(builder, httpdRedirectBlocks,
			prefix+"buetow.org", prefix+"blog.buetow.org", prefix+"snonux.foo", snonux, prefix+"paul.buetow.org")
	}
	for _, prefix := range prefixes {
		appendf(builder, httpdDtailBlock, prefix+"dtail.dev")
	}
	for _, host := range []string{"irregular.ninja", "alt.irregular.ninja", "joern.buetow.org", "dory.buetow.org", "ecat.buetow.org", "gogios.buetow.org"} {
		root := map[string]string{
			"irregular.ninja":     "/htdocs/irregular.ninja",
			"alt.irregular.ninja": "/htdocs/alt.irregular.ninja",
			"joern.buetow.org":    "/htdocs/joern/",
			"dory.buetow.org":     "/htdocs/joern/dory.buetow.org",
			"ecat.buetow.org":     "/htdocs/joern/ecat.buetow.org",
			"gogios.buetow.org":   "/htdocs/buetow.org/self/gogios",
		}[host]
		for _, prefix := range prefixes {
			appendf(builder, httpdRootBlock, prefix+host, root)
		}
	}
}

func appendHTTPDF3S(builder *strings.Builder, host string) {
	appendf(builder, httpdF3SBlocks, host, host)
}

func renderRelayd(data webConfigData) string {
	var builder strings.Builder
	appendString(&builder, relaydPreamble)
	appendString(&builder, "http protocol \"https\" {\n")
	for _, host := range data.AcmeHosts {
		if host == "blowfish.buetow.org" || host == "fishfinger.buetow.org" || strings.HasPrefix(host, "ipv4.") || strings.HasPrefix(host, "ipv6.") {
			continue
		}
		appendf(&builder, "     tls keypair %s\n     tls keypair standby.%s\n", host, host)
	}
	appendf(&builder, relaydHTTPSStart, data.Server.FQDN)
	for _, prefix := range data.Prefixes {
		appendf(&builder, "    block request header \"Host\" value %q\n", prefix+"code.f3s.buetow.org")
	}
	for _, host := range data.AcmeHosts {
		if contains(data.F3SHosts, host) || host == "snonux.foo" {
			continue
		}
		for _, prefix := range data.Prefixes {
			appendf(&builder, "    match request header \"Host\" value %q forward to <localhost>\n", prefix+host)
		}
	}
	appendRelaydF3SRouting(&builder, data)
	appendString(&builder, relaydHTTPSFinish)
	appendf(&builder, relaydRelayBlocks,
		data.Server.IPv4, data.Server.IPv6,
		data.Server.IPv4, data.Server.IPv6,
		data.Server.IPv4, data.Server.IPv6,
		data.Server.IPv4, data.Server.IPv6)
	return builder.String()
}

func appendRelaydF3SRouting(builder *strings.Builder, data webConfigData) {
	for _, host := range data.F3SHosts {
		for _, prefix := range data.Prefixes {
			name := prefix + host
			switch {
			case host == "f3s.buetow.org":
				appendf(builder, "    match request header \"Host\" value %q forward to <f3s_static_proxy>\n", name)
			case host == "registry.f3s.buetow.org":
				appendf(builder, "    match request header \"Host\" value %q forward to <f3s_registry>\n", name)
			case host == "jellyfin.f3s.buetow.org":
				appendf(builder, "    match request header \"Host\" value %q forward to <f3s_jellyfin>\n", name)
			case host == "anki.f3s.buetow.org":
				appendf(builder, "    match request header \"Host\" value %q forward to <f3s_anki>\n", name)
			case host == "garage.f3s.buetow.org" || strings.HasSuffix(host, ".garage.f3s.buetow.org"):
				appendf(builder, "    match request header \"Host\" value %q forward to <garage>\n", name)
			}
		}
	}
	appendString(builder, "    match request header \"Host\" value \"www.snonux.foo\" forward to <localhost>\n")
	for _, prefix := range []string{"", "standby."} {
		appendf(builder, "    match request header \"Host\" value %q forward to <f3s_static_proxy>\n", prefix+"snonux.foo")
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

const httpdServerBlocks = `
# Current server's FQDN (e.g. for mail server ACME cert requests)
server %q {
  listen on * port 80
  log style forwarded
  location "/.well-known/acme-challenge/*" {
    root "/acme"
    request strip 2
  }
  location * {
    block return 302 "https://%s"
  }
}

server %q {
  listen on * port 8080
  log style forwarded
  location * {
    root "/htdocs/buetow.org/self"
    directory auto index
  }
}

# f3s cluster fallback page on port 8080 when cluster is down
server "f3s.buetow.org" {
  listen on * port 8080
  no log
  location * {
    request rewrite "/index.html"
    root "/htdocs/f3s_fallback"
  }
}

server "*.f3s.buetow.org" {
  listen on * port 8080
  no log
  location * {
    request rewrite "/index.html"
    root "/htdocs/f3s_fallback"
  }
}

# Gemtexter hosts
`

const httpdRedirectBlocks = `server %q {
  listen on * port 8080
  log style forwarded
  location * {
    block return 302 "https://paul.buetow.org$REQUEST_URI"
  }
}

server %q {
  listen on * port 8080
  log style forwarded
  location * {
    block return 302 "https://foo.zone$REQUEST_URI"
  }
}

server %q {
  listen on * port 8080
  log style forwarded
  location * {
%s  }
}

server %q {
  listen on * port 8080
  log style forwarded
  location * {
    block return 302 "https://foo.zone/about$REQUEST_URI"
  }
}
`

const httpdDtailBlock = `server %q {
  listen on * port 8080
  log style forwarded
  location * {
    block return 302 "https://github.com/snonux/dtail"
  }
}
`

const httpdRootBlock = `server %q {
  listen on * port 8080
  log style forwarded
  location * {
    root %q
    directory auto index
  }
}
`

const httpdF3SBlocks = `server "%s-port80" {
  listen on * port 80
  log style forwarded
  location "/.well-known/acme-challenge/*" {
    root "/acme"
    request strip 2
  }
  location * {
    block return 302 "https://$HTTP_HOST$REQUEST_URI"
  }
}

server "%s-port8080" {
  listen on * port 8080
  log style forwarded
  location * {
    request rewrite "/index.html"
    root "/htdocs/f3s_fallback"
  }
}
`

const httpdDefaults = `# Defaults
server "default" {
  listen on * port 80
  log style forwarded
  block return 302 "https://foo.zone$REQUEST_URI"
}

server "default" {
  listen on * port 8080
  log style forwarded
  block return 302 "https://foo.zone$REQUEST_URI"
}
`

const relaydPreamble = `log connection

table <f3s> {
  192.168.2.120
  192.168.2.121
  192.168.2.122
}
table <f3s_static> {
  192.168.2.203
  192.168.2.204
}
table <f3s_static_proxy> {
  127.0.0.1
  ::1
}
table <f3s_registry> {
  192.168.2.120
  192.168.2.121
  192.168.2.122
}
table <f3s_jellyfin> {
  192.168.2.120
  192.168.2.121
  192.168.2.122
}
table <f3s_anki> {
  192.168.2.120
  192.168.2.121
  192.168.2.122
}
table <garage> {
  192.168.2.130
  192.168.2.131
  192.168.2.132
}
table <forgejo_ssh> {
  192.168.2.120
  192.168.2.121
  192.168.2.122
}
table <localhost> {
  127.0.0.1
  ::1
}
`

const relaydHTTPSStart = `     tls keypair %s

     http websockets

    match request header set "X-Forwarded-For" value "$REMOTE_ADDR"
    match request header set "X-Forwarded-Proto" value "https"
    pass header "Connection"
    pass header "Upgrade"
    pass header "Sec-WebSocket-Key"
    pass header "Sec-WebSocket-Version"
    pass header "Sec-WebSocket-Extensions"
    pass header "Sec-WebSocket-Protocol"
`

const relaydHTTPSFinish = `    match response header "Server" value "OpenBSD httpd" tag "HTTPD_FALLBACK"
    match response tagged "HTTPD_FALLBACK" header set "Cache-Control" value "no-cache, no-store, must-revalidate"
    match response tagged "HTTPD_FALLBACK" header set "Pragma" value "no-cache"
    match response tagged "HTTPD_FALLBACK" header set "Expires" value "0"
}
`

const relaydRelayBlocks = `
relay "https4" {
    listen on %s port 443 tls
    protocol "https"
    session timeout 300
    forward to <f3s> port 80 check tcp
    forward to <localhost> port 8080 check http "/" code 200
    forward to <f3s_static_proxy> port 18080 check tcp
    forward to <f3s_registry> port 30001 check tcp
    forward to <f3s_jellyfin> port 30096 check tcp
    forward to <f3s_anki> port 30800 check tcp
    forward to <garage> port 3900 check tcp
}

relay "https6" {
    listen on %s port 443 tls
    protocol "https"
    session timeout 300
    forward to <f3s> port 80 check tcp
    forward to <localhost> port 8080 check http "/" code 200
    forward to <f3s_static_proxy> port 18080 check tcp
    forward to <f3s_registry> port 30001 check tcp
    forward to <f3s_jellyfin> port 30096 check tcp
    forward to <f3s_anki> port 30800 check tcp
    forward to <garage> port 3900 check tcp
}

tcp protocol "gemini" {
    tls keypair foo.zone
    tls keypair stats.foo.zone
    tls keypair snonux.foo
    tls keypair paul.buetow.org
    tls keypair standby.foo.zone
    tls keypair standby.stats.foo.zone
    tls keypair standby.snonux.foo
    tls keypair standby.paul.buetow.org
}

relay "gemini4" {
    listen on %s port 1965 tls
    protocol "gemini"
    forward to 127.0.0.1 port 11965
}
relay "gemini6" {
    listen on %s port 1965 tls
    protocol "gemini"
    forward to 127.0.0.1 port 11965
}

http protocol "forgejo-alt" {
    tls keypair code.f3s.buetow.org
    http websockets
    match request header set "X-Forwarded-For" value "$REMOTE_ADDR"
    match request header set "X-Forwarded-Proto" value "https"
    pass header "Connection"
    pass header "Upgrade"
    pass header "Sec-WebSocket-Key"
    pass header "Sec-WebSocket-Version"
    pass header "Sec-WebSocket-Extensions"
    pass header "Sec-WebSocket-Protocol"
    match request header "Host" value "code.f3s.buetow.org" forward to <f3s>
}
relay "forgejo_alt4" {
    listen on %s port 2443 tls
    protocol "forgejo-alt"
    forward to <f3s> port 80 check tcp
}
relay "forgejo_alt6" {
    listen on %s port 2443 tls
    protocol "forgejo-alt"
    forward to <f3s> port 80 check tcp
}

relay "forgejo_ssh4" {
    listen on %s port 2022
    forward to <forgejo_ssh> port 30222 check tcp
}
relay "forgejo_ssh6" {
    listen on %s port 2022
    forward to <forgejo_ssh> port 30222 check tcp
}

relay "f3s_static_proxy4" {
    listen on 127.0.0.1 port 18080
    forward to <f3s_static> port 80 check tcp
    forward to <localhost> port 8080 check http "/" code 200
}
relay "f3s_static_proxy6" {
    listen on ::1 port 18080
    forward to <f3s_static> port 80 check tcp
    forward to <localhost> port 8080 check http "/" code 200
}
`
