package frontends

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// customOpenBSDPackages is shared by both frontends and must name the
// release they run (both 7.9 since 2026-09-26). It has to match `uname -r`:
// the unattended-upgrade audit builds the same URL from it. Bump it after
// the next release upgrade, once dtail/gogios are rebuilt into the new tree
// (packages/Makefile OPENBSD_VERSION).
const (
	customOpenBSDPackages = "https://pkgrepo.f3s.buetow.org/openbsd/7.9/packages/amd64/"
	gogiosPluginDir       = "/usr/local/libexec/nagios"
	f3sTakenDown          = "/var/db/f3s/monitoring/f3s_taken_down"
)

// Monitoring owns frontend monitoring and custom-package configuration.
type Monitoring struct{ RequiresRoot }

// PkgRepo configures the signed frontend package repository for interactive
// pkg_add.
//
// PkgRepo preserves Rex's root-shell convenience setting and sets the same
// PKG_PATH for rex himself: doas.conf has keepenv, so rex's `doas pkg_add -u`
// sees rex's environment, not root's profile. Custom package resources below
// also set PKG_PATH directly, so non-login applies are safe.
// The value is "installpath:<custom>", like unattended-upgrade.sh's: a set
// PKG_PATH replaces installurl(5), so the former custom-only value made a
// root-shell `pkg_add -u` miss every official update, while an unset one
// (rex's `doas pkg_add -u` until 2026-09-26) misses dtail and gogios
// ("Couldn't find updates for ..."). installpath resolves installurl, so
// both trees are searched.
// WithKeyedLine owns the one PKG_PATH export in the profile: an older,
// unquoted, or otherwise differently-formed legacy assignment is replaced in
// place instead of staying beside the current line as a duplicate, so this
// also covers a quoted/unquoted or stale-URL variant Rex might have left
// behind, without ever needing to know its exact prior form. The mode and
// ownership are explicit (0644 root:wheel, as on both frontends) because a
// line edit without WithMode applies gonf's 0640 default even when the line
// is already present.
func (Monitoring) PkgRepo() {
	line := WithKeyedLine("export PKG_PATH=", `export PKG_PATH="installpath:`+customOpenBSDPackages+`"`)
	File("/root/.profile", line, RootOwned)
	File("/home/rex/.profile", line, Perm(0o644, "rex:rex"))
}

// DTail installs DTail from the signed fleet repository and enables dserver.
//
// DTail preserves the former cleanup, account, retention, key-cache, and
// daemon behavior. Cleanup only removes an old unpackaged installation.
//
// /etc/dserver/dtail.json is managed too (task 3h2): an unmanaged, hand-edited
// copy on blowfish once pointed HostKeyFile below /var/run, which OpenBSD
// wipes at boot, so dserver failed after a reboot. The asset pins the host key
// to /var/db/dserver/ssh_host_key (persistent; the key itself is never managed
// here) and keeps the key cache under /var/run/dserver/cache, which the daily
// dserver-update-key-cache.sh run rebuilds. It holds only paths and settings,
// no secrets. Mode and ownership match the package's 0644 root:bin install.
// The file depends on the package, which creates /etc/dserver, and a changed
// config restarts dserver so the daemon reads it.
func (Monitoring) DTail() {
	cleanup := cleanupLegacyDTail()
	pkg := Package("dtail", WithEnv(map[string]string{"PKG_PATH": customOpenBSDPackages}), IsLatest, DependsOn(cleanup))
	account := frontendAccount(serviceAccount{Name: "_dserver", Home: "/var/run/dserver", LoginClass: "nologin"})
	File(dailyLocal,
		WithLine("/usr/local/bin/dserver-update-key-cache.sh"),
		WithLine("find /var/log/dserver -name \"*.log\" -mtime +7 -delete"),
		RootOwned, DependsOn(pkg, account))
	config := InstallFile("/etc/dserver/dtail.json", frontendAsset("dtail.json"),
		Perm(0o644, "root:bin"), DependsOn(pkg))
	Service("dserver", WithRestart, DependsOn(pkg, account), OnChange(config))
}

// OptsDTailChecks keeps this configuration-only action out of the aggregate,
// where Gogios already publishes the same configuration.
func (Monitoring) OptsDTailChecks() TaskOptions {
	return TaskOptions{Operational()}
}

// DTailChecks publishes Gogios configuration with the WireGuard DTail targets.
// It assumes the existing Gogios installation and changes no package, account,
// plugin, cron job or running service. Use alongside the dserver bind rollout.
func (Monitoring) DTailChecks() {
	EachHost(func(server Server) {
		File("/etc/gogios.json", WithContent(renderGogios(server)), Perm(0o744, Root))
	})
}

func cleanupLegacyDTail() Resource {
	return Command("sh", List("-ceu", `
for binary in dserver dcat dgrep dmap dtail dtailhealth; do
  path=/usr/local/bin/$binary
  [ -e "$path" ] || continue
  pkg_info -e "dtail-*" >/dev/null 2>&1 && continue
  rm -f -- "$path"
done`),
		OnlyIf("sh", List("-c", "! pkg_info -e 'dtail-*' >/dev/null 2>&1")),
		WithName("cleanup-unpackaged-dtail"))
}

// Gogios installs Gogios monitoring, checks, schedules, and status
// publishing.
//
// Gogios renders the old embedded-Perl configuration from the shared Go
// topology, retaining stable check names because peer state keys use them.
// A missing check_shuriken_age plugin source (shurikenAgePlugin) is reported
// as a declaration error before anything is declared, which fails the
// recording with its actionable message; the Gogios resources, whose
// schedules depend on the plugin, are then not declared at all.
func (Monitoring) Gogios() {
	pluginSource := shurikenAgePlugin()
	if pluginSource == "" {
		return
	}
	EachHost(func(server Server) {
		plugins := Packages("monitoring-plugins")
		cleanup := cleanupLegacyGogios()
		gogios := Package("gogios", WithEnv(map[string]string{"PKG_PATH": customOpenBSDPackages}), IsLatest, DependsOn(cleanup))
		account := frontendAccount(serviceAccount{Name: "_gogios", Home: "/var/run/gogios"})
		statusDir := Dir("/var/www/htdocs/buetow.org/self/gogios", Perm(0o755, "_gogios:_gogios"), DependsOn(account))
		runDir := Dir("/var/run/gogios", Perm(0o755, "_gogios:_gogios"), DependsOn(account))
		mute := installMuteControl(account)
		config := File("/etc/gogios.json", WithContent(renderGogios(server)), Perm(0o640, "root:_gogios"), DependsOn(plugins, gogios, statusDir, runDir, mute))
		plugin := InstallFile("/usr/local/bin/check_shuriken_age", pluginSource, RootExec)
		removeStaleDNSRoleFiles()
		gogiosUser := WithCronUser("_gogios")
		// -renotify and -force run at minute 2, off the */5 grid: Gogios
		// (1.6.0+) serialises runs with a lock in /var/run/gogios, where a
		// plain run skips and these two wait, so the offset keeps them from
		// waiting on (or re-running right after) a plain run.
		CronAt("gogios-renotify", "2 7 * * *", "/usr/local/bin/gogios -renotify >/dev/null 2>&1", gogiosUser, DependsOn(config, plugin))
		// Checks run around the clock (audit item 23): an 08-22 window let an
		// overnight outage go unnoticed until the morning. Gogios has no
		// notification quiet hours, so overnight CRITICAL changes mail too;
		// WARNING/UNKNOWN changes still wait for the 07:02 renotify.
		CronAt("gogios-checks", "*/5 * * * *", "/usr/local/bin/gogios >/dev/null 2>&1", gogiosUser, DependsOn(config, plugin))
		CronAt("gogios-force", "2 3 * * 0", "/usr/local/bin/gogios -force >/dev/null 2>&1", gogiosUser, DependsOn(config, plugin))
		rcLocal := ensureRCLocal()
		File("/etc/rc.local",
			WithLine("if [ ! -d /var/run/gogios ]; then mkdir /var/run/gogios; fi"),
			WithLine("chown _gogios /var/run/gogios"),
			RootOwned, DependsOn(account, rcLocal))
	})
}

// staleDNSRoleFiles are the role caches the retired dns-failover rotation kept
// under /var/nsd/run. The DNS publisher (dns-publish.ksh) no longer writes
// them, so they froze at the last rotation; Gogios read current_standby to
// elect its checker and kept electing a host that was no longer the DNS
// standby. Gogios now resolves DNSStandbyRecord instead, and nothing else
// reads these files, so they are removed rather than left to mislead a
// fallback or a human.
var staleDNSRoleFiles = List(
	"/var/nsd/run/current_master", "/var/nsd/run/current_standby",
	"/var/nsd/run/master_a", "/var/nsd/run/master_aaaa",
	"/var/nsd/run/standby_a", "/var/nsd/run/standby_aaaa",
)

func removeStaleDNSRoleFiles() {
	for _, path := range staleDNSRoleFiles {
		File(path, IsAbsent)
	}
}

func cleanupLegacyGogios() Resource {
	return Command("sh", List("-ceu", `
path=/usr/local/bin/gogios
[ -e "$path" ] || exit 0
pkg_info -e "gogios-*" >/dev/null 2>&1 || rm -f -- "$path"`),
		OnlyIf("sh", List("-c", "! pkg_info -e 'gogios-*' >/dev/null 2>&1")),
		WithName("cleanup-unpackaged-gogios"))
}

func (Monitoring) OptsFoostats() TaskOptions {
	return TaskOptions{Needs(Maintenance.Rsync)}
}

// Foostats installs private visitor counters, shared keyed pseudonyms, and aggregate reporting.
func (Monitoring) Foostats() {
	source, odds := foostatsSource("foostats.pl"), foostatsSource("fooodds.txt")
	if source == "" || odds == "" {
		return
	}
	EachHost(func(server Server) { installFoostats(server, source, odds) })
}

func (Monitoring) OptsFoostatsPurgePublic() TaskOptions {
	return TaskOptions{Needs(Monitoring.Foostats), Operational()}
}

// FoostatsPurgePublic removes obsolete public visitor archives after an approved migration.
func (Monitoring) FoostatsPurgePublic() {
	Command("perl", List("-MFile::Find", "-e", `
my $root = "/var/www/htdocs/buetow.org/self/foostats";
die "Refusing a symlink statistics root\n" if -l $root;
exit 0 unless -d $root;
# Sys::Hostname values included historical underscore labels. Older copies
# also put the source hostname before the protocol/date rather than after it.
my $host = qr/[A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)*/;
my $day = qr/(?:web|gemini)_[0-9]{8}/;
find({no_chdir => 1, wanted => sub {
    my $path = $File::Find::name;
    my ($name) = $path =~ m{([^/]+)$};
    return unless $name =~ /\A(?:$day\.$host|$host\.$day)\.json\.gz(?:\.tmp)?\z/;
    return unless -f $path || -l $path;
    unlink $path or die "Cannot remove an obsolete visitor archive\n";
}}, $root);
`), WithName("foostats-purge-public-archives"))
}

// shurikenAgePlugin is the Gogios album-age plugin in the controller's
// shuriken.sh checkout (paths.Shuriken, ~/git/shuriken.sh by default), its
// one source of truth as in Rex; there is no in-repo copy to fall back to,
// so RequireControllerFile reports a missing or unreadable source with a
// hint and refuses the recording; Gogios skips the dependent declarations.
func shurikenAgePlugin() string {
	return RequireControllerFile(filepath.Join(paths.Shuriken, "contrib", "check_shuriken_age"),
		"frontends_gogios (check_shuriken_age plugin)",
		"clone shuriken.sh to ~/git/shuriken.sh or set GONF_SHURIKEN_ROOT to its checkout")
}

// foostatsSource requires the canonical Foostats checkout. The former
// in-repo fallback predates private statistics and must never be deployed.
func foostatsSource(name string) string {
	path := RequireControllerFile(filepath.Join(paths.Foostats, name), "frontends_foostats",
		"clone foostats or set GONF_FOOSTATS_ROOT to the reviewed private-statistics checkout")
	if path != "" && name == "foostats.pl" && !foostatsPrivateSource(path) {
		return ""
	}
	return path
}

// frontendAccount declares a frontend service account. WithManageHome keeps
// Rex's existing-account `usermod -d` behavior: gonf rewrites only the passwd
// home field when it differs, without moving, creating, or chowning the
// directory (callers that need the directory declare it with Dir).
func frontendAccount(spec serviceAccount) Resource {
	opts := []LocalUserOption{WithPrimaryGroup(spec.Name), WithHome(spec.Home), WithManageHome}
	if spec.LoginClass != "" {
		opts = append(opts, WithLoginClass(spec.LoginClass))
	}
	return User(spec.Name, opts...)
}

type gogiosCheck struct {
	Plugin          string   `json:"Plugin"`
	Args            []string `json:"Args"`
	OnlyIfNotExists string   `json:"OnlyIfNotExists,omitempty"`
	RandomSpread    int      `json:"RandomSpread,omitempty"`
	Retries         int      `json:"Retries,omitempty"`
	RetryInterval   int      `json:"RetryInterval,omitempty"`
	RunInterval     int      `json:"RunInterval,omitempty"`
	DependsOn       []string `json:"DependsOn,omitempty"`
	// Local marks a check of the frontend itself (see addSystemChecks),
	// which Gogios 1.6.0+ also runs on the passive frontend.
	Local bool `json:"Local,omitempty"`
}

// gogiosConfig is the typed top level of /etc/gogios.json. Its fields are
// declared in alphabetical order of their JSON names: encoding/json writes
// struct fields in declaration order, and the file was written from a map
// (sorted keys) before, so this order keeps it byte for byte.
type gogiosConfig struct {
	CheckConcurrency          int                    `json:"CheckConcurrency"`
	CheckTimeoutS             int                    `json:"CheckTimeoutS"`
	Checks                    map[string]gogiosCheck `json:"Checks"`
	DNSStandbyRecord          string                 `json:"DNSStandbyRecord"`
	EmailFrom                 string                 `json:"EmailFrom"`
	EmailTo                   string                 `json:"EmailTo"`
	HTMLStatusFile            string                 `json:"HTMLStatusFile"`
	MinNotifyIntervalS        int                    `json:"MinNotifyIntervalS"`
	PeerPrimaryName           string                 `json:"PeerPrimaryName"`
	PeerSecondaryName         string                 `json:"PeerSecondaryName"`
	PeerURL                   string                 `json:"PeerURL"`
	PrometheusHosts           []string               `json:"PrometheusHosts"`
	PrometheusOnlyIfNotExists string                 `json:"PrometheusOnlyIfNotExists"`
	StateDir                  string                 `json:"StateDir"`
}

// renderGogios renders server's Gogios configuration. Each frontend polls the
// other one as its peer; the primary/secondary names follow the roles. They
// only name the pair: the checker is the current DNS standby, which Gogios
// (1.5.0+) finds by resolving DNSStandbyRecord. The passive frontend mirrors
// the checker's report as of the checker's previous run (both run from the
// same */5 cron, so the copy lags up to one interval), which means
// gogios.buetow.org (it follows the DNS master, normally the passive node)
// shows the same state up to ~5 minutes late. Each frontend's own host checks
// (Local) run on that frontend in either role, so both reports list both.
//
// Its panics are genuine programmer-bug invariants, not recipe or input
// errors, so they stay panics rather than declaration errors (see gonf's
// AGENTS.md, "Registration-time contract"). addGarageChecks panics only on a
// garageNodes entry without a WireGuard row (see its doc comment). Here,
// gogiosConfig holds only strings, ints, string slices and a string-keyed
// map of such structs, which encoding/json always marshals, so MarshalIndent
// cannot fail unless a future edit adds an unmarshalable field type. (MustServer's names are the
// package's own constants; see its doc comment.)
func renderGogios(server Server) string {
	peer := MustServer(Master)
	if server.Name == Master {
		peer = MustServer(Standby)
	}
	// Port 30090 serves the read proxy, permitting these frontends and earth's
	// WireGuard clients. Gogios uses GET /api/v1/alerts.
	config := gogiosConfig{
		EmailTo:                   "paul",
		EmailFrom:                 "gogios@mx.buetow.org",
		CheckTimeoutS:             10,
		CheckConcurrency:          3,
		MinNotifyIntervalS:        3600,
		StateDir:                  "/var/run/gogios",
		HTMLStatusFile:            "/var/www/htdocs/buetow.org/self/gogios/index.html",
		PeerURL:                   "https://" + peer.FQDN + "/gogios/index.json",
		PeerPrimaryName:           MustServer(Master).FQDN,
		PeerSecondaryName:         MustServer(Standby).FQDN,
		DNSStandbyRecord:          "standby." + Domain,
		PrometheusHosts:           []string{"r0.wg0:30090", "r1.wg0:30090", "r2.wg0:30090"},
		PrometheusOnlyIfNotExists: f3sTakenDown,
		Checks:                    gogiosChecks(server),
	}
	encoded, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("marshal Gogios configuration: %v", err))
	}
	return string(encoded) + "\n"
}

// gogiosChecks builds every Gogios check for server. The builders run in the
// order the checks were originally declared, so a later builder may replace
// an earlier check of the same name exactly as before the split.
func gogiosChecks(server Server) map[string]gogiosCheck {
	checks := make(map[string]gogiosCheck)
	addPingChecks(checks)
	addFrontendHostChecks(checks)
	addPiHTTPChecks(checks)
	addSiteChecks(checks)
	addPeerCertificateChecks(checks, server)
	addLegacyHTTPSChecks(checks, server)
	addGarageChecks(checks)
	addFrontendServiceChecks(checks)
	addSystemChecks(checks, server)
	addShurikenChecks(checks)
	return checks
}

// addPingCheck adds "Check Ping<proto> <host>" pinging address. WireGuard
// hosts get wider loss thresholds; f3s hosts are skipped while the cluster is
// deliberately taken down.
func addPingCheck(checks map[string]gogiosCheck, host string, proto int, address string, retries int, f3s bool) {
	args := List("-H", address, fmt.Sprintf("-%d", proto), "-w", "100,10%", "-c", "200,15%")
	if strings.Contains(host, ".wg0.") {
		args = List("-H", address, fmt.Sprintf("-%d", proto), "-w", "100,20%", "-c", "200,30%")
	}
	check := gogiosCheck{Plugin: gogiosPluginDir + "/check_ping", Args: args, RandomSpread: 10, Retries: retries, RetryInterval: 3}
	if f3s {
		check.OnlyIfNotExists = f3sTakenDown
	}
	checks[fmt.Sprintf("Check Ping%d %s", proto, host)] = check
}

// addPingChecks pings the master/standby service names and every WireGuard
// mesh peer except the roaming clients and the rocky VM. The standalone
// peers (f3) are pinged by address too, although the frontends' /etc/hosts
// does not list them; like every non-frontend peer they pause while
// /var/db/f3s/monitoring/f3s_taken_down exists (`f3sctl power all off` sets it and powers f3
// off with the rest).
func addPingChecks(checks map[string]gogiosCheck) {
	for _, role := range []string{"master", "standby"} {
		for _, proto := range []int{4, 6} {
			addPingCheck(checks, role+".buetow.org", proto, role+".buetow.org", 3, false)
		}
	}
	for _, peer := range append(WireGuardAddresses(), StandaloneWireGuardAddresses()...) {
		if peer.Name == "earth" || peer.Name == "pixel7pro" || peer.Name == "rocky" {
			continue
		}
		for _, proto := range []int{4, 6} {
			address := peer.IPv4
			if proto == 6 {
				address = peer.IPv6
			}
			addPingCheck(checks, peer.Name+".wg0.wan.buetow.org", proto, address, 5, peer.Name != Master && peer.Name != Standby)
		}
	}
}

// addFrontendHostChecks checks each frontend's DTail server, public pings
// and host certificate.
func addFrontendHostChecks(checks map[string]gogiosCheck) {
	for _, host := range []string{Master, Standby} {
		fqdn := host + ".buetow.org"
		checks["Check DTail "+fqdn] = gogiosCheck{Plugin: "/usr/local/bin/dtailhealth", Args: List("--server", fqdn+":2222"), RandomSpread: 10, RunInterval: 3600, DependsOn: pingDependencies(fqdn)}
		for _, proto := range []int{4, 6} {
			addPingCheck(checks, fqdn, proto, fqdn, 3, false)
		}
		checks["Check TLS Certificate "+fqdn] = gogiosCheck{Plugin: gogiosPluginDir + "/check_http", Args: List("--sni", "-H", fqdn, "-C", "20"), RandomSpread: 10, RunInterval: 3600, DependsOn: pingDependencies(fqdn)}
	}
}

// addLegacyHTTPSChecks verifies the mirrored legacy sites against this
// frontend's own public addresses, including aliases that resolve to its peer.
// check_http deliberately does not verify names or trust chains; check_curl's
// -D does both. Checking each frontend catches stale peer copies before DNS
// failover exposes them. Local keeps them running on the passive frontend;
// curl checks reachability itself, so no non-local ping can suppress it.
// A 200 response also checks the explicit Host route.
func addLegacyHTTPSChecks(checks map[string]gogiosCheck, server Server) {
	for _, host := range List("joern.buetow.org", "dory.buetow.org", "ecat.buetow.org") {
		for _, prefix := range TemplateData().Prefixes {
			name := prefix + host
			for _, proto := range []int{4, 6} {
				address := server.IPv4
				if proto == 6 {
					address = server.IPv6
				}
				args := List("-S", "-D", "--sni", "-H", name, "-I", address,
					fmt.Sprintf("-%d", proto), "-C", "20,7", "--continue-after-certificate",
					"-e", "HTTP/1.1 200", "-t", "10")
				checks[fmt.Sprintf("Check Legacy HTTPS IPv%d %s on %s", proto, name, server.Name)] = gogiosCheck{
					Plugin: gogiosPluginDir + "/check_curl", Args: args,
					RandomSpread: 10, RunInterval: 300, Local: true,
				}
			}
		}
	}
}

// addPiHTTPChecks checks the pi0/pi1 static-site backends over WireGuard.
func addPiHTTPChecks(checks map[string]gogiosCheck) {
	for _, host := range []string{"pi0", "pi1"} {
		checks["Check HTTP "+host+".wg0.wan.buetow.org"] = gogiosCheck{Plugin: gogiosPluginDir + "/check_http", Args: List(host+".wg0.wan.buetow.org", "-4"), RandomSpread: 10, OnlyIfNotExists: f3sTakenDown}
	}
}

// addSiteChecks checks every ACME site (Sites, the policy shared with the
// certificates, routing and DNS): certificates, HTTP and HTTPS per prefix,
// then plain HTTP for the ipv4./ipv6. single-family names.
func addSiteChecks(checks map[string]gogiosCheck) {
	sites := Sites()
	for _, site := range sites {
		// Test ingress names (including www./standby./ipv4./ipv6. aliases)
		// are excluded from every Gogios site check.
		if strings.HasSuffix(site.Name, "-test.f3s."+Domain) {
			continue
		}
		addHostChecks(checks, site)
		addAlternateCertificateChecks(checks, site)
		addHTTPSChecks(checks, site)
		if site.Family != 0 {
			checks[fmt.Sprintf("Check HTTP IPv%d %s", site.Family, site.Name)] = httpCheck(
				List(site.Name, fmt.Sprintf("-%d", site.Family)), []string{fmt.Sprintf("Check Ping%d master.buetow.org", site.Family)})
		}
	}
}

// addAlternateCertificateChecks watches the primary certificate on each
// frontend, independently of which frontend currently owns the site's DNS.
// The standby SNI certificate is a separate keypair, so checking only the
// standby name cannot detect stale primary material on the failover host.
func addAlternateCertificateChecks(checks map[string]gogiosCheck, site Site) {
	if site.FrontendHost || site.Family != 0 || site.HTTPSPort == "" {
		return
	}
	for _, name := range List(Master, Standby) {
		server := MustServer(name)
		checkName := fmt.Sprintf("Check TLS Certificate %s via %s:%s", site.Name, server.FQDN, site.HTTPSPort)
		checks[checkName] = gogiosCheck{
			Plugin: gogiosPluginDir + "/check_http",
			Args: List("--sni", "-H", site.Name, "-I", server.IPv4,
				"-p", site.HTTPSPort, "-C", "20"),
			RandomSpread: 10, RunInterval: 3600,
			DependsOn: List("Check Ping4 " + server.FQDN),
		}
	}
}

// addGarageChecks checks each Garage node's admin /health endpoint (:3903, no
// token needed) over WireGuard. The garage.f3s.buetow.org site checks cannot
// see a single node: relayd forwards to whichever node answers its TCP check,
// so one node could be down while the public 403 stays green. A node answers
// 200 while it has quorum and 503 without. The check does not require
// "fully operational", because then one dead node would fail all three
// checks instead of only its own. While a host's ping check is not OK its
// Garage check is skipped as UNKNOWN (DependsOn), so a powered-off host is
// CRITICAL only once. There is no RunInterval: Gogios stamps a result after
// the random spread, so a 300s interval would skip every other */5 run. The
// retries cover the short restart garage_config does after a config change.
//
// Its panic is a programmer-bug invariant like MustServer's: garageNodes is a
// static list, so a node without a WireGuard row is a typo in this package,
// not recipe or input data.
func addGarageChecks(checks map[string]gogiosCheck) {
	for _, node := range garageNodes {
		peer, err := wireGuardAddressFor(node)
		if err != nil {
			panic(fmt.Sprintf("Garage node %q: %v", node, err))
		}
		host := node + ".wg0.wan.buetow.org"
		checks["Check Garage Health "+host] = gogiosCheck{
			Plugin:          gogiosPluginDir + "/check_http",
			Args:            List("-H", peer.IPv4, "-p", "3903", "-u", "/health"),
			OnlyIfNotExists: f3sTakenDown,
			RandomSpread:    10,
			Retries:         2,
			RetryInterval:   10,
			DependsOn:       []string{"Check Ping4 " + host},
		}
	}
}

// addFrontendServiceChecks checks DNS, SMTP and Gemini on each frontend over
// both address families.
func addFrontendServiceChecks(checks map[string]gogiosCheck) {
	for _, host := range []string{Master, Standby} {
		for _, proto := range []int{4, 6} {
			fqdn := host + ".buetow.org"
			deps := []string{fmt.Sprintf("Check Ping%d %s", proto, fqdn)}
			checks[fmt.Sprintf("Check Dig %s IPv%d", fqdn, proto)] = checkWithPlugin(gogiosPluginDir+"/check_dig", List("-H", fqdn, "-l", Domain, fmt.Sprintf("-%d", proto)), deps)
			checks[fmt.Sprintf("Check SMTP %s IPv%d", fqdn, proto)] = checkWithPlugin(gogiosPluginDir+"/check_smtp", List("-H", fqdn, fmt.Sprintf("-%d", proto)), deps)
			checks[fmt.Sprintf("Check Gemini TCP %s IPv%d", fqdn, proto)] = checkWithPlugin(gogiosPluginDir+"/check_tcp", List("-H", fqdn, "-p", "1965", fmt.Sprintf("-%d", proto)), deps)
		}
	}
}

// addSystemChecks checks the local host's users, swap, processes, disk and
// load. Swap thresholds are free-space percentages and the same on both
// frontends: OpenBSD pages idle memory out over a long uptime, so a few
// hundred MB of used swap with plenty of free RAM is normal. The old
// non-Master "-w 95% -c 90%" went CRITICAL at 10% used (blowfish, 2026-09-26,
// 11% used after 10 days, 1.1G RAM free, no paging). They
// are Local: only this host can run them, so it runs (and mails for) them
// even while the other frontend is the elected checker; their names carry
// the host name, which Gogios requires to tell the peers' checks apart.
func addSystemChecks(checks map[string]gogiosCheck, server Server) {
	checks["Check Users "+server.Name] = gogiosCheck{Plugin: gogiosPluginDir + "/check_users", Args: List("-w", "2", "-c", "3"), RandomSpread: 10, RunInterval: 600, Local: true}
	swap := List("-w", "20%", "-c", "10%")
	checks["Check SWAP "+server.Name] = gogiosCheck{Plugin: gogiosPluginDir + "/check_swap", Args: swap, RandomSpread: 10, RunInterval: 300, Local: true}
	for name, args := range map[string][]string{"Procs": List("-w", "100", "-c", "150"), "Disk": List("-w", "30%", "-c", "10%"), "Load": List("-w", "2,1,1", "-c", "4,3,3")} {
		checks["Check "+name+" "+server.Name] = gogiosCheck{Plugin: gogiosPluginDir + "/check_" + strings.ToLower(name), Args: args, RandomSpread: 10, RunInterval: 300, Local: true}
	}
}

// addShurikenChecks checks the age of the irregular.ninja photo albums.
func addShurikenChecks(checks map[string]gogiosCheck) {
	for _, host := range []string{"irregular.ninja", "alt.irregular.ninja"} {
		checks["Check Shuriken Age "+host] = gogiosCheck{Plugin: "/usr/local/bin/check_shuriken_age", Args: List("-f", "/var/www/htdocs/"+host+"/status.json", "-w", "1814400", "-c", "3024000"), RandomSpread: 10, RunInterval: 1800, DependsOn: []string{"Check HTTP IPv4 " + host, "Check HTTP IPv6 " + host}}
	}
}

// addHostChecks checks a dual-stack site's certificate and plain HTTP under
// each name prefix; the standby. name depends on the standby frontend.
func addHostChecks(checks map[string]gogiosCheck, site Site) {
	if site.FrontendHost || site.Family != 0 {
		return
	}
	for _, prefix := range []string{"", "standby.", "www."} {
		name := prefix + site.Name
		depends := "master.buetow.org"
		if prefix == "standby." {
			depends = "standby.buetow.org"
		}
		args := List("--sni", "-H", name, "-C", "20")
		if site.HTTPSPort != "" {
			args = append(args, "-p", site.HTTPSPort)
		}
		checks["Check TLS Certificate "+name] = gogiosCheck{Plugin: gogiosPluginDir + "/check_http", Args: args, RandomSpread: 10, RunInterval: 3600, DependsOn: pingDependencies(depends)}
		for _, proto := range []int{4, 6} {
			checks[fmt.Sprintf("Check HTTP IPv%d %s", proto, name)] = httpCheck(List(name, fmt.Sprintf("-%d", proto)), []string{fmt.Sprintf("Check Ping%d %s", proto, depends)})
		}
	}
}

// addHTTPSChecks checks a dual-stack site over HTTPS on both address
// families, expecting its Site.HTTPSStatus when it has one. f3s sites pause
// while the cluster is taken down.
func addHTTPSChecks(checks map[string]gogiosCheck, site Site) {
	if site.FrontendHost || site.Family != 0 || !site.HTTPSCheck {
		return
	}
	args := List("--sni", "-S", "-H", site.Name)
	if site.HTTPSPort != "" {
		args = append(args, "-p", site.HTTPSPort)
	}
	if site.HTTPSStatus != "" {
		args = append(args, "-e", site.HTTPSStatus)
	}
	for _, proto := range []int{4, 6} {
		check := httpCheck(argsWithProto(args, proto), []string{fmt.Sprintf("Check Ping%d master.buetow.org", proto)})
		check.RunInterval = 300
		if site.F3S {
			check.OnlyIfNotExists = f3sTakenDown
		}
		checks[fmt.Sprintf("Check HTTPS IPv%d %s", proto, site.Name)] = check
	}
}

func pingDependencies(host string) []string {
	return []string{"Check Ping4 " + host, "Check Ping6 " + host}
}
func httpCheck(args []string, deps []string) gogiosCheck {
	return checkWithPlugin(gogiosPluginDir+"/check_http", args, deps)
}
func checkWithPlugin(plugin string, args []string, deps []string) gogiosCheck {
	return gogiosCheck{Plugin: plugin, Args: args, RandomSpread: 10, DependsOn: deps}
}
func argsWithProto(args []string, proto int) []string {
	return append(append([]string(nil), args...), fmt.Sprintf("-%d", proto))
}
