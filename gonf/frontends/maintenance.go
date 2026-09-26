package frontends

import (
	"strings"

	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/goprecords"
	"github.com/snonux/conf/gonf/paths"
)

const dailyLocal = "/etc/daily.local"

// Maintenance carries the root-only, non-network frontend maintenance tasks.
// Its package, content, and schedule policy comes directly from the matching
// Rex tasks, while the host-specific facts live in cluster inventory.
type Maintenance struct {
	RequiresRoot
}

type serviceAccount struct {
	Name       string
	Home       string
	LoginClass string
}

// ServiceAccounts creates the disabled Gorum service account.
func (Maintenance) ServiceAccounts() {
	frontendAccount(serviceAccount{Name: "_gorum", Home: "/var/run/gorum", LoginClass: "nologin"})
}

// Base installs frontend base packages and local administration helpers.
//
// Base installs the common operator packages and the small rc files owned by
// the Rex base task. DTail, uptimed, ZNC and node_exporter are merely named in
// pkg_scripts here (see pkgScriptsLine); their packages and services are owned
// by the tasks that install them.
func (Maintenance) Base() {
	EachHost(func(server Server) {
		Packages("figlet", "tig", "vger", "zsh", "bash", "helix")
		ensureRCLocal()
		rcConfLocalLine(pkgScriptsLine(server.Name), "rc-conf-pkg-scripts-"+server.Name)
	})
}

// Myname sets each frontend's OpenBSD hostname.
func (Maintenance) Myname() {
	EachHost(func(server Server) {
		File("/etc/myname", WithContent(server.FQDN+"\n"),
			RootOwned)
	})
}

// WireGuardHosts appends WireGuard mesh IPv4 and IPv6 host entries.
//
// WireGuardHosts appends the source-controlled mesh rows without replacing
// administrator-owned /etc/hosts content. The mode and ownership are explicit
// (0644 root:wheel, as on both frontends): a line edit without WithMode
// applies gonf's 0640 default even when every line is already present, and
// an unreadable /etc/hosts breaks wg0 name resolution for every non-root
// daemon (Gogios checks run as _gogios).
func (Maintenance) WireGuardHosts() {
	File("/etc/hosts", WithLines(WireGuardHostLines()...), RootOwned)
}

// Uptimed installs and enables the uptimed service.
func (Maintenance) Uptimed() {
	uptimed := Package("uptimed")
	Service("uptimed", DependsOn(uptimed))
}

// goprecordsSchedule runs the frontend upload hourly, a quarter past so it
// does not pile onto the :00 newsyslog run.
//
// Why hourly, not /etc/daily.local (task xk2): until 2026-09-25 the upload
// ran once a day from daily.local at 01:30 frontend time (23:30 UTC). Since
// mid-August the f3s cluster is powered off every night by then, so relayd's
// https relay finds its <f3s> table down and falls back to the local httpd
// (the "f3s is down" page), which answers the PUT with 405; curl -f fails,
// the client's set -e aborts, and no frontend upload landed after
// 2026-08-15 while gonf showed no drift. An hourly run catches every hour
// the cluster is up, the same as the f-hosts' freebsd_goprecords_upload.
const goprecordsSchedule = "15 * * * *"

// Goprecords installs optional hourly uptimed uploads to goprecords (root
// cron, output to syslog).
//
// Goprecords installs the uploader (goprecords.Client) on every frontend
// and runs it hourly from root's crontab with its output in syslog
// (goprecords.CronCommand; a 405 while f3s is down is logged, not mailed).
// A host lacking its controller-side token receives no token or schedule
// and keeps whatever it already has (the optional-token policy, see
// goprecords.Client).
//
// The former daily.local lines (the old goprecords-upload.sh and the
// daily client run) are removed only after the cron entry is in place.
//
// The token is read inside the EachHost body, so only runs that EachHost
// narrows to a host selection skip other hosts' tokens: a single-host push
// or preview whose destination maps exactly to one inventory host resolves
// only that frontend's token (plus any alias sharing its SSH host), and a
// local Run only the names its own hostname contains. Every run that records
// all cluster members still reads every host's token: a cluster push,
// `gonf plan`, a raw `gonf push -- <ssh args>`, or a destination that is
// unknown to the inventory or contradicts its user or port.
func (Maintenance) Goprecords() {
	EachHost(func(server Server) {
		uploader, ok := goprecords.Client(paths.FrontendSecret("etc/goprecords/" + server.Name + ".token"))
		if !ok {
			return
		}
		NoFile("/usr/local/bin/goprecords-upload.sh")
		cron := CronAt("goprecords-upload", goprecordsSchedule,
			goprecords.CronCommand(server.Name), DependsOn(uploader))
		File(dailyLocal,
			WithoutLine("/usr/local/bin/goprecords-upload.sh"),
			WithoutLine(goprecords.CommandLine(server.Name)),
			RootOwned, DependsOn(cron))
	})
}

// Rsync installs frontend rsync service configuration and synchronization
// cron.
//
// Rsync installs the common daemon configuration and the root cron entry
// (which adopted the former Rex line). The command intentionally retains
// Rex's leading -ns argument.
func (Maintenance) Rsync() {
	rsync := Package("rsync")
	InstallFile("/etc/rsyncd.conf", frontendAsset("rsyncd.conf"), RootOwned)
	script := InstallFile("/usr/local/bin/rsync.sh", legacyFrontendAsset("scripts/rsync.sh.tpl"),
		RootExec)
	CronAt("frontend-rsync", "*/5 * * * *", "-ns /usr/local/bin/rsync.sh", DependsOn(rsync, script))
}

// Gemtexter installs the daily Gemtexter content updater.
func (Maintenance) Gemtexter() {
	script := InstallFile("/usr/local/bin/gemtexter.sh", legacyFrontendAsset("scripts/gemtexter.sh.tpl"),
		Perm(0o744, Root))
	File(dailyLocal, WithLine("/usr/local/bin/gemtexter.sh"), RootOwned, DependsOn(script))
}

// ACME installs per-frontend ACME client configuration and daily renewal
// hook.
//
// ACME installs Go-native equivalents of the former Perl templates, both
// rendered from one certificate list (acmeData, see acme.go). The actual
// invocation remains a separate network-service task so a setup plan cannot
// request certificates or restart daemons.
func (Maintenance) ACME() {
	EachHost(func(server Server) {
		data := acmeData(server)
		config := InstallFile("/etc/acme-client.conf", frontendAsset("acme-client.conf.tmpl"),
			WithTemplateData(data), RootOwned)
		script := InstallFile("/usr/local/bin/acme.sh", frontendAsset("acme.sh.tmpl"),
			WithTemplateData(data), Perm(0o744, Root))
		File(dailyLocal, WithLine("/usr/local/bin/acme.sh"),
			RootOwned, DependsOn(config, script))
	})
}

// OptsIRCBouncer marks the ZNC deployment as an Operational, by-name action,
// so no pattern aggregate can pick it up.
func (Maintenance) OptsIRCBouncer() TaskOptions { return TaskOptions{Operational()} }

// IRCBouncer installs and enables the fishfinger IRC bouncer.
func (Maintenance) IRCBouncer() {
	WhenHostname(Master, func() {
		znc := Package("znc")
		Service("znc", DependsOn(znc))
	})
}

// ensureRCLocal declares /etc/rc.local with the attributes it has on both
// frontends, 0644 root:wheel. Base and Gogios both declare it, and every
// declaration and line edit of the file must agree: a line edit without a
// mode would chmod it to gonf's 0640 default on every apply.
func ensureRCLocal() Resource {
	return EnsureFile("/etc/rc.local", RootOwned)
}

// rcConfLocalLine declares one line of /etc/rc.conf.local under name: the
// pkg_scripts line and nsd_flags (see nsdFlags); the other daemons' NAME_flags
// lines are set through rcctl by Service(..., WithFlags(...)). Every line edit of the file sets
// the same attributes, 0644 root:wheel as rcctl leaves it (and as it is on
// both frontends): a line edit without a mode defaults to 0640, which would
// chmod the file back and forth on every apply.
func rcConfLocalLine(line, name string) Resource {
	return File("/etc/rc.conf.local", WithLine(line), RootOwned, WithName(name))
}

// pkgScriptsLine renders the rc.conf.local pkg_scripts line exactly as
// rcctl(8) writes it: unquoted, with every package daemon a frontend Service
// enables (dserver, uptimed, znc on fishfinger, and node_exporter from the PF
// task) in rcctl's append order. `rcctl enable` of a daemon missing from
// pkg_scripts deletes every pkg_scripts= line and writes one combined,
// unquoted line, so a literal line lacking such a daemon (or quoted, as Rex
// wrote it) was missing again on the next apply and was re-appended; OpenBSD's
// rc takes the last assignment, which could disable node_exporter at boot.
// With the complete, rcctl-formatted line the Services find their daemons
// enabled, rcctl never rewrites the line, and a second apply is a no-op.
// icinga2 (still in the Rex list) is not installed on either frontend and was
// already dropped from the live line by rcctl, so it is not listed.
func pkgScriptsLine(name string) string {
	scripts := []string{"uptimed", "httpd", "dserver"}
	if name == Master {
		scripts = append(scripts, "znc")
	}
	scripts = append(scripts, "node_exporter")
	return "pkg_scripts=" + strings.Join(scripts, " ")
}
