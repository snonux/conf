package frontends

import (
	"encoding/json"
	"fmt"
	"strings"

	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"
)

const (
	customOpenBSDPackages = "https://pkgrepo.f3s.buetow.org/openbsd/7.8/packages/amd64/"
	gogiosPluginDir       = "/usr/local/libexec/nagios"
	f3sTakenDown          = "/tmp/f3s_taken_down"
)

// Monitoring owns frontend monitoring and custom-package configuration.
type Monitoring struct{ RequiresRoot }

func (Monitoring) DescPkgRepo() string {
	return "Configure the signed frontend package repository for interactive pkg_add"
}

// PkgRepo preserves Rex's root-shell convenience setting. Custom package
// resources below also set PKG_PATH directly, so non-login applies are safe.
func (Monitoring) PkgRepo() {
	onFrontends(func() {
		File("/root/.profile", WithLine("export PKG_PATH="+customOpenBSDPackages))
	})
}

func (Monitoring) DescDTail() string {
	return "Install DTail from the signed fleet repository and enable dserver"
}

// DTail preserves the former cleanup, account, retention, key-cache, and
// daemon behavior. Cleanup only removes an old unpackaged installation.
func (Monitoring) DTail() {
	onFrontends(func() {
		cleanup := cleanupLegacyDTail()
		pkg := Package("dtail", WithEnv(map[string]string{"PKG_PATH": customOpenBSDPackages}), IsLatest, DependsOn(cleanup))
		account := frontendAccount(serviceAccount{Name: "_dserver", Home: "/var/run/dserver", LoginClass: "nologin"})
		File(dailyLocal,
			WithLine("/usr/local/bin/dserver-update-key-cache.sh"),
			WithLine("find /var/log/dserver -name \"*.log\" -mtime +7 -delete"),
			WithMode(0o644), WithOwner("root"), WithGroup("wheel"), DependsOn(pkg, account))
		Service("dserver", DependsOn(pkg, account))
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

func (Monitoring) DescGogios() string {
	return "Install Gogios monitoring, checks, schedules, and status publishing"
}

// Gogios renders the old embedded-Perl configuration from the shared Go
// topology, retaining stable check names because peer state keys use them.
func (Monitoring) Gogios() {
	for _, host := range ClusterHosts() {
		server := MustHostValue[Server](host, ValueServer)
		WhenHostname(host, func() {
			plugins := Package(List("monitoring-plugins", "nrpe"))
			cleanup := cleanupLegacyGogios()
			gogios := Package("gogios", WithEnv(map[string]string{"PKG_PATH": customOpenBSDPackages}), IsLatest, DependsOn(cleanup))
			account := frontendAccount(serviceAccount{Name: "_gogios", Home: "/var/run/gogios"})
			statusDir := Dir("/var/www/htdocs/buetow.org/self/gogios", WithMode(0o755), WithOwner("_gogios"), WithGroup("_gogios"), DependsOn(account))
			runDir := Dir("/var/run/gogios", WithMode(0o755), WithOwner("_gogios"), WithGroup("_gogios"), DependsOn(account))
			config := File("/etc/gogios.json", WithContent(renderGogios(server)), WithMode(0o744), WithOwner("root"), WithGroup("wheel"), DependsOn(plugins, gogios, statusDir, runDir))
			plugin := InstallFile("/usr/local/bin/check_shuriken_age", "/home/paul/git/shuriken.sh/contrib/check_shuriken_age", WithMode(0o755), WithOwner("root"), WithGroup("wheel"))
			Cron("gogios-renotify", WithCronUser("_gogios"), WithCommand("/usr/local/bin/gogios -renotify >/dev/null 2>&1"),
				WithLegacyCommand("/usr/local/bin/gogios -renotify >/dev/null 2>&1"), WithMinute("0"), WithHour("7"), DependsOn(config, plugin))
			Cron("gogios-checks", WithCronUser("_gogios"), WithCommand("/usr/local/bin/gogios >/dev/null 2>&1"),
				WithLegacyCommand("/usr/local/bin/gogios >/dev/null 2>&1"), WithMinute("*/5"), WithHour("8-22"), DependsOn(config, plugin))
			Cron("gogios-force", WithCronUser("_gogios"), WithCommand("/usr/local/bin/gogios -force >/dev/null 2>&1"),
				WithLegacyCommand("/usr/local/bin/gogios -force >/dev/null 2>&1"), WithMinute("0"), WithHour("3"), WithWeekday("0"), DependsOn(config, plugin))
			EnsureFile("/etc/rc.local")
			File("/etc/rc.local",
				WithLine("if [ ! -d /var/run/gogios ]; then mkdir /var/run/gogios; fi"),
				WithLine("chown _gogios /var/run/gogios"),
				DependsOn(account))
		})
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

func (Monitoring) DescFoostats() string {
	return "Install Foostats reporting, dependencies, daily hook, and log rotation"
}

func (Monitoring) Foostats() {
	onFrontends(func() {
		deps := Package(List("p5-Digest-SHA3", "p5-PerlIO-gzip", "p5-JSON", "p5-String-Util", "p5-LWP-Protocol-https"))
		script := InstallFile("/usr/local/bin/foostats.pl", "/home/paul/git/foostats/foostats.pl", WithMode(0o500), WithOwner("root"), WithGroup("wheel"), DependsOn(deps))
		data := InstallFile("/var/www/htdocs/buetow.org/self/foostats/fooodds.txt", "/home/paul/git/foostats/fooodds.txt", WithMode(0o440), WithOwner("root"), WithGroup("wheel"))
		Dir("/var/www/htdocs/gemtexter/stats.foo.zone", WithMode(0o755), WithOwner("root"), WithGroup("wheel"))
		Dir("/var/gemini/stats.foo.zone", WithMode(0o755), WithOwner("root"), WithGroup("wheel"))
		File(dailyLocal, WithLine("perl /usr/local/bin/foostats.pl --parse-logs --replicate --report"), WithMode(0o644), WithOwner("root"), WithGroup("wheel"), DependsOn(script, data))
		InstallFile("/etc/newsyslog.conf", legacyFrontendAsset("etc/newsyslog.conf"), WithMode(0o644), WithOwner("root"), WithGroup("wheel"))
	})
}

func frontendAccount(spec serviceAccount) Resource {
	opts := []LocalUserOption{WithPrimaryGroup(spec.Name), WithHome(spec.Home)}
	if spec.LoginClass != "" {
		opts = append(opts, WithLoginClass(spec.LoginClass))
	}
	account := User(spec.Name, opts...)
	return Command("usermod", List("-d", spec.Home, spec.Name), DependsOn(account), Unless("sh", List("-c", accountHomeGuard(spec))), WithName("usermod-home-"+spec.Name))
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
}

func renderGogios(server Server) string {
	peer := MustServer(Master)
	if server.Name == Master {
		peer = MustServer(Standby)
	}
	config := map[string]any{
		"EmailTo": "paul", "EmailFrom": "gogios@mx.buetow.org", "CheckTimeoutS": 10, "CheckConcurrency": 3, "MinNotifyIntervalS": 3600,
		"StateDir": "/var/run/gogios", "HTMLStatusFile": "/var/www/htdocs/buetow.org/self/gogios/index.html",
		"PeerURL": "https://" + peer.FQDN + "/gogios/index.json", "PeerPrimaryName": MustServer(Master).FQDN, "PeerSecondaryName": MustServer(Standby).FQDN,
		"PrometheusHosts": []string{"r0.wg0:30090", "r1.wg0:30090", "r2.wg0:30090"}, "PrometheusOnlyIfNotExists": f3sTakenDown, "Checks": gogiosChecks(server),
	}
	encoded, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("marshal Gogios configuration: %v", err))
	}
	return string(encoded) + "\n"
}

func gogiosChecks(server Server) map[string]gogiosCheck {
	checks := make(map[string]gogiosCheck)
	addPing := func(host string, proto int, address string, retries int, f3s bool) {
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
	for _, role := range []string{"master", "standby"} {
		for _, proto := range []int{4, 6} {
			addPing(role+".buetow.org", proto, role+".buetow.org", 3, false)
		}
	}
	for _, peer := range WireGuardAddresses() {
		if peer.Name == "earth" || peer.Name == "pixel7pro" || peer.Name == "rocky" {
			continue
		}
		for _, proto := range []int{4, 6} {
			address := peer.IPv4
			if proto == 6 {
				address = peer.IPv6
			}
			addPing(peer.Name+".wg0.wan.buetow.org", proto, address, 5, peer.Name != Master && peer.Name != Standby)
		}
	}
	for _, host := range []string{Master, Standby} {
		fqdn := host + ".buetow.org"
		checks["Check DTail "+fqdn] = gogiosCheck{Plugin: "/usr/local/bin/dtailhealth", Args: List("--server", fqdn+":2222"), RandomSpread: 10, RunInterval: 3600, DependsOn: pingDependencies(fqdn)}
		for _, proto := range []int{4, 6} {
			addPing(fqdn, proto, fqdn, 3, false)
		}
		checks["Check TLS Certificate "+fqdn] = gogiosCheck{Plugin: gogiosPluginDir + "/check_http", Args: List("--sni", "-H", fqdn, "-C", "20"), RandomSpread: 10, RunInterval: 3600, DependsOn: pingDependencies(fqdn)}
	}
	for _, host := range []string{"pi0", "pi1"} {
		checks["Check HTTP "+host+".wg0.wan.buetow.org"] = gogiosCheck{Plugin: gogiosPluginDir + "/check_http", Args: List(host+".wg0.wan.buetow.org", "-4"), RandomSpread: 10, OnlyIfNotExists: f3sTakenDown}
	}
	for _, host := range TemplateData().AcmeHosts {
		addHostChecks(checks, host)
		addHTTPSChecks(checks, host)
	}
	for _, host := range TemplateData().AcmeHosts {
		if strings.HasPrefix(host, "ipv4.") || strings.HasPrefix(host, "ipv6.") {
			proto := 4
			if strings.HasPrefix(host, "ipv6.") {
				proto = 6
			}
			checks[fmt.Sprintf("Check HTTP IPv%d %s", proto, host)] = httpCheck(List(host, fmt.Sprintf("-%d", proto)), []string{fmt.Sprintf("Check Ping%d master.buetow.org", proto)})
		}
	}
	for _, host := range []string{Master, Standby} {
		for _, proto := range []int{4, 6} {
			fqdn := host + ".buetow.org"
			deps := []string{fmt.Sprintf("Check Ping%d %s", proto, fqdn)}
			checks[fmt.Sprintf("Check Dig %s IPv%d", fqdn, proto)] = checkWithPlugin(gogiosPluginDir+"/check_dig", List("-H", fqdn, "-l", Domain, fmt.Sprintf("-%d", proto)), deps)
			checks[fmt.Sprintf("Check SMTP %s IPv%d", fqdn, proto)] = checkWithPlugin(gogiosPluginDir+"/check_smtp", List("-H", fqdn, fmt.Sprintf("-%d", proto)), deps)
			checks[fmt.Sprintf("Check Gemini TCP %s IPv%d", fqdn, proto)] = checkWithPlugin(gogiosPluginDir+"/check_tcp", List("-H", fqdn, "-p", "1965", fmt.Sprintf("-%d", proto)), deps)
		}
	}
	checks["Check Users "+server.Name] = gogiosCheck{Plugin: gogiosPluginDir + "/check_users", Args: List("-w", "2", "-c", "3"), RandomSpread: 10, RunInterval: 600}
	swap := List("-w", "95%", "-c", "90%")
	if server.Name == Master {
		swap = List("-w", "20%", "-c", "10%")
	}
	checks["Check SWAP "+server.Name] = gogiosCheck{Plugin: gogiosPluginDir + "/check_swap", Args: swap, RandomSpread: 10, RunInterval: 300}
	for name, args := range map[string][]string{"Procs": List("-w", "100", "-c", "150"), "Disk": List("-w", "30%", "-c", "10%"), "Load": List("-w", "2,1,1", "-c", "4,3,3")} {
		checks["Check "+name+" "+server.Name] = gogiosCheck{Plugin: gogiosPluginDir + "/check_" + strings.ToLower(name), Args: args, RandomSpread: 10, RunInterval: 300}
	}
	for _, host := range []string{"irregular.ninja", "alt.irregular.ninja"} {
		checks["Check Shuriken Age "+host] = gogiosCheck{Plugin: "/usr/local/bin/check_shuriken_age", Args: List("-f", "/var/www/htdocs/"+host+"/status.json", "-w", "1814400", "-c", "3024000"), RandomSpread: 10, RunInterval: 1800, DependsOn: []string{"Check HTTP IPv4 " + host, "Check HTTP IPv6 " + host}}
	}
	return checks
}

func addHostChecks(checks map[string]gogiosCheck, host string) {
	if host == MustServer(Master).FQDN || host == MustServer(Standby).FQDN || strings.HasPrefix(host, "ipv4.") || strings.HasPrefix(host, "ipv6.") {
		return
	}
	port := ""
	if host == "code.f3s.buetow.org" {
		port = "2443"
	}
	for _, prefix := range []string{"", "standby.", "www."} {
		name := prefix + host
		depends := "master.buetow.org"
		if prefix == "standby." {
			depends = "standby.buetow.org"
		}
		args := List("--sni", "-H", name, "-C", "20")
		if port != "" {
			args = append(args, "-p", port)
		}
		checks["Check TLS Certificate "+name] = gogiosCheck{Plugin: gogiosPluginDir + "/check_http", Args: args, RandomSpread: 10, RunInterval: 3600, DependsOn: pingDependencies(depends)}
		for _, proto := range []int{4, 6} {
			checks[fmt.Sprintf("Check HTTP IPv%d %s", proto, name)] = httpCheck(List(name, fmt.Sprintf("-%d", proto)), []string{fmt.Sprintf("Check Ping%d %s", proto, depends)})
		}
	}
}

func addHTTPSChecks(checks map[string]gogiosCheck, host string) {
	if host == MustServer(Master).FQDN || host == MustServer(Standby).FQDN || strings.HasPrefix(host, "ipv4.") || strings.HasPrefix(host, "ipv6.") || host == "ychat.f3s.buetow.org" {
		return
	}
	args := List("--sni", "-S", "-H", host)
	if host == "code.f3s.buetow.org" {
		args = append(args, "-p", "2443")
	}
	if expected := httpsExpected(host); expected != "" {
		args = append(args, "-e", expected)
	}
	for _, proto := range []int{4, 6} {
		check := httpCheck(argsWithProto(args, proto), []string{fmt.Sprintf("Check Ping%d master.buetow.org", proto)})
		check.RunInterval = 300
		if isF3SHost(host) {
			check.OnlyIfNotExists = f3sTakenDown
		}
		checks[fmt.Sprintf("Check HTTPS IPv%d %s", proto, host)] = check
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
func isF3SHost(host string) bool {
	for _, candidate := range TemplateData().F3SHosts {
		if host == candidate {
			return true
		}
	}
	return false
}
func httpsExpected(host string) string {
	if strings.HasSuffix(host, ".garage.f3s.buetow.org") || host == "garage.f3s.buetow.org" {
		return "HTTP/1.1 403"
	}
	return map[string]string{"player.f3s.buetow.org": "HTTP/1.1 401", "xplayer.f3s.buetow.org": "HTTP/1.1 401", "webdav.f3s.buetow.org": "HTTP/1.1 401", "koreader.f3s.buetow.org": "HTTP/1.1 412", "pihole.f3s.buetow.org": "HTTP/1.1 404", "anki.f3s.buetow.org": "HTTP/1.1 404", "grafana.f3s.buetow.org": "HTTP/1.1 404", "pkgrepo.f3s.buetow.org": "HTTP/1.1 404"}[host]
}
