package frontends

import (
	"path/filepath"
	"strings"

	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"

	"codeberg.org/snonux/conf/gonf/paths"
)

const (
	dailyLocal       = "/etc/daily.local"
	frontendAssetDir = "gonf/frontends/assets"
)

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

// DescServiceAccounts retains the disabled Gorum identity as a separately
// applicable account-only task. DTail and Gogios declare their own accounts
// with their respective services so either task is independently usable.
func (Maintenance) DescServiceAccounts() string {
	return "Create the disabled Gorum service account"
}

func (Maintenance) ServiceAccounts() {
	onFrontends(func() {
		frontendAccount(serviceAccount{Name: "_gorum", Home: "/var/run/gorum", LoginClass: "nologin"})
	})
}

// DescBase returns the description shown for the frontend base task.
func (Maintenance) DescBase() string {
	return "Install frontend base packages and local administration helpers"
}

// Base installs the common operator packages and the small rc files owned by
// the Rex base task. DTail, uptimed, ZNC and node_exporter are merely named in
// pkg_scripts here (see pkgScriptsLine); their packages and services are owned
// by the tasks that install them.
func (Maintenance) Base() {
	ForHosts(ValueServer, func(_ string, server Server) {
		Package(List("figlet", "tig", "vger", "zsh", "bash", "helix"))
		ensureRCLocal()
		rcConfLocalLine(pkgScriptsLine(server.Name), "rc-conf-pkg-scripts-"+server.Name)
	})
}

// DescMyname returns the description shown for the frontend hostname task.
func (Maintenance) DescMyname() string {
	return "Set each frontend's OpenBSD hostname"
}

// Myname writes the stable FQDN from inventory rather than rendering the Rex
// closure-based template on the destination.
func (Maintenance) Myname() {
	ForHosts(ValueServer, func(_ string, server Server) {
		File("/etc/myname", WithContent(server.FQDN+"\n"),
			WithMode(0o644), WithOwner("root"), WithGroup("wheel"))
	})
}

// DescWireGuardHosts returns the description shown for the hosts task.
func (Maintenance) DescWireGuardHosts() string {
	return "Append WireGuard mesh IPv4 and IPv6 host entries"
}

// WireGuardHosts appends the source-controlled mesh rows without replacing
// administrator-owned /etc/hosts content. The mode and ownership are explicit
// (0644 root:wheel, as on both frontends): a line edit without WithMode
// applies gonf's 0640 default even when every line is already present, and
// an unreadable /etc/hosts breaks wg0 name resolution for every non-root
// daemon (Gogios checks run as _gogios).
func (Maintenance) WireGuardHosts() {
	onFrontends(func() {
		File("/etc/hosts", WithLines(WireGuardHostLines()...),
			WithMode(0o644), WithOwner("root"), WithGroup("wheel"))
	})
}

// DescUptimed returns the description shown for the uptime recorder task.
func (Maintenance) DescUptimed() string {
	return "Install and enable the uptimed service"
}

// Uptimed installs the recorder and converges it to enabled/running.
func (Maintenance) Uptimed() {
	onFrontends(func() {
		uptimed := Package("uptimed")
		Service("uptimed", DependsOn(uptimed))
	})
}

// DescGoprecords returns the description shown for the optional uploader task.
func (Maintenance) DescGoprecords() string {
	return "Install optional daily uptimed uploads to goprecords"
}

// Goprecords installs the uploader on every frontend. A host lacking its
// controller-side token receives no token, hook, or schedule, matching Rex's
// safe skip behavior while keeping a missing token out of plans and logs.
//
// The token is read inside the ForHosts body, so only runs that ForHosts
// narrows to a host selection skip other hosts' tokens: a single-host push
// or preview whose destination maps exactly to one inventory host resolves
// only that frontend's token (plus any alias sharing its SSH host), and a
// local Run only the names its own hostname contains. Every run that records
// all cluster members still reads every host's token: a cluster push,
// `gonf plan`, a raw `gonf push -- <ssh args>`, or a destination that is
// unknown to the inventory or contradicts its user or port.
//
// Optional-token policy (task 262, explicit now — it used to be an
// unlabelled side effect of the early return below). OptionalSecret
// distinguishes "no such secret" (secret.ErrNotFound) from every other
// failure: a locked, unreachable or misconfigured secret store still fails
// the whole plan record loudly (see gonf's docs/secrets.md), it is never
// read as "this host has no token". Given a genuine not-found, three
// policies were considered for a host's already-deployed
// /etc/goprecords-upload.token and its daily.local hook:
//   - keep (chosen): declare nothing further for this host and leave
//     whatever is already on the destination exactly as it is. Matches P9's
//     "no automatic deletion of legacy copies" and this task's
//     declaration/mechanism-layer scope: actively disabling or removing a
//     live token is a destination-state change of its own, which needs its
//     own controlled rotation/recovery exercise (see
//     docs/consumer-dsl-simplification-plan.md, P9) and explicit
//     authorization, not a side effect of a secret-provider change.
//   - disable: also strip the daily.local hook line (WithoutLine) so a
//     stale token stops being submitted even though the token file itself
//     is left in place. Rejected for now: it still leaves secret material
//     on disk while silently changing the host's schedule, which reads as
//     more surprising than either doing nothing or doing both.
//   - remove: also delete the token file (NoFile) and the hook line, fully
//     converging to "no token configured". Rejected for now: an explicit
//     NoFile deletion of what may be the operator's only remaining copy of
//     a secret needs the controlled recovery exercise P9 calls for, not an
//     automatic decision made here.
//
// Revisit once P9's controlled rotation/recovery exercise authorizes a
// change; until then this function's behavior is unchanged from before this
// comment, only its policy is now named and documented instead of implicit.
func (Maintenance) Goprecords() {
	ForHosts(ValueServer, func(_ string, server Server) {
		token, ok := OptionalSecret(paths.FrontendSecret("etc/goprecords/" + server.Name + ".token"))
		Package("curl")
		if !ok {
			// keep: see the optional-token policy above.
			return
		}
		token = strings.TrimRight(token, "\r\n")
		if token == "" {
			return
		}
		File("/etc/goprecords-upload.token", WithContent(token+"\n"),
			WithMode(0o600), WithOwner("root"), WithGroup("wheel"))
		uploader := InstallFile("/usr/local/bin/goprecords-upload-client.sh",
			legacyFrontendAsset("scripts/goprecords-upload-client.sh"),
			WithMode(0o755), WithOwner("root"), WithGroup("wheel"))
		NoFile("/usr/local/bin/goprecords-upload.sh")
		File(dailyLocal,
			WithoutLine("/usr/local/bin/goprecords-upload.sh"),
			WithLine("GOPRECORDS_HOST="+server.Name+" /usr/local/bin/goprecords-upload-client.sh"),
			WithMode(0o644), WithOwner("root"), WithGroup("wheel"), DependsOn(uploader))
	})
}

// DescRsync returns the description shown for the frontend rsync task.
func (Maintenance) DescRsync() string {
	return "Install frontend rsync service configuration and synchronization cron"
}

// Rsync installs the common daemon configuration and adopts the former Rex
// root cron entry. The command intentionally retains Rex's leading -ns argument.
func (Maintenance) Rsync() {
	onFrontends(func() {
		rsync := Package("rsync")
		InstallFile("/etc/rsyncd.conf", frontendAsset("rsyncd.conf"),
			WithMode(0o644), WithOwner("root"), WithGroup("wheel"))
		script := InstallFile("/usr/local/bin/rsync.sh", legacyFrontendAsset("scripts/rsync.sh.tpl"),
			WithMode(0o755), WithOwner("root"), WithGroup("wheel"))
		Cron("frontend-rsync", WithCommand("-ns /usr/local/bin/rsync.sh"),
			WithLegacyCommand("-ns /usr/local/bin/rsync.sh"), WithMinute("*/5"), DependsOn(rsync, script))
	})
}

// DescGemtexter returns the description shown for the static-site task.
func (Maintenance) DescGemtexter() string {
	return "Install the daily Gemtexter content updater"
}

// Gemtexter installs the source-controlled updater and appends it to the
// existing daily.local file without replacing other maintenance hooks.
func (Maintenance) Gemtexter() {
	onFrontends(func() {
		script := InstallFile("/usr/local/bin/gemtexter.sh", legacyFrontendAsset("scripts/gemtexter.sh.tpl"),
			WithMode(0o744), WithOwner("root"), WithGroup("wheel"))
		File(dailyLocal, WithLine("/usr/local/bin/gemtexter.sh"),
			WithMode(0o644), WithOwner("root"), WithGroup("wheel"), DependsOn(script))
	})
}

// DescACME returns the description shown for the certificate setup task.
func (Maintenance) DescACME() string {
	return "Install per-frontend ACME client configuration and daily renewal hook"
}

// ACME installs Go-native equivalents of the former Perl templates, both
// rendered from one certificate list (acmeData, see acme.go). The actual
// invocation remains a separate network-service task so a setup plan cannot
// request certificates or restart daemons.
func (Maintenance) ACME() {
	ForHosts(ValueServer, func(_ string, server Server) {
		data := acmeData(server)
		config := InstallFile("/etc/acme-client.conf", frontendAsset("acme-client.conf.tmpl"),
			WithTemplateData(data), WithMode(0o644), WithOwner("root"), WithGroup("wheel"))
		script := InstallFile("/usr/local/bin/acme.sh", frontendAsset("acme.sh.tmpl"),
			WithTemplateData(data), WithMode(0o744), WithOwner("root"), WithGroup("wheel"))
		File(dailyLocal, WithLine("/usr/local/bin/acme.sh"),
			WithMode(0o644), WithOwner("root"), WithGroup("wheel"), DependsOn(config, script))
	})
}

// DescIRCBouncer returns the description shown for the fishfinger-only ZNC
// service task.
func (Maintenance) DescIRCBouncer() string {
	return "Install and enable the fishfinger IRC bouncer"
}

// OptsIRCBouncer marks the ZNC deployment as an Operational, by-name action,
// so no pattern aggregate can pick it up. Privileged() is repeated because the
// per-method companion replaces the RequiresRoot struct default.
func (Maintenance) OptsIRCBouncer() TaskOptions { return TaskOptions{Privileged(), Operational()} }

// IRCBouncer keeps Rex's separate service group and applies only to the host
// with the existing runtime configuration; it does not enter the all-frontend
// aggregate.
func (Maintenance) IRCBouncer() {
	WhenHostname(Master, func() {
		znc := Package("znc")
		Service("znc", DependsOn(znc))
	})
}

func frontendAsset(name string) string {
	return filepath.Join(paths.Conf, frontendAssetDir, name)
}

func onFrontends(fn func()) {
	WhenHostname(ClusterHosts(), fn)
}

func legacyFrontendAsset(name string) string {
	return filepath.Join(paths.Frontends, name)
}

// ensureRCLocal declares /etc/rc.local with the attributes it has on both
// frontends, 0644 root:wheel. Base and Gogios both declare it, and every
// declaration and line edit of the file must agree: a line edit without a
// mode would chmod it to gonf's 0640 default on every apply.
func ensureRCLocal() Resource {
	return EnsureFile("/etc/rc.local", WithMode(0o644), WithOwner("root"), WithGroup("wheel"))
}

// rcConfLocalLine declares one line of /etc/rc.conf.local under name. Every
// line edit of the file sets the same attributes, 0644 root:wheel as rcctl
// leaves it (and as it is on both frontends): a line edit without a mode
// defaults to 0640, so mixing the two chmodded the file back and forth on
// every apply.
func rcConfLocalLine(line, name string) Resource {
	return File("/etc/rc.conf.local", WithLine(line), WithMode(0o644), WithOwner("root"), WithGroup("wheel"), WithName(name))
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
