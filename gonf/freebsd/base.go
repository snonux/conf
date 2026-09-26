package freebsd

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/etchosts"
	"github.com/snonux/conf/gonf/paths"
)

// Base manages the host baseline every f-host shares: /etc/hosts, doas.conf,
// the interactive base packages, uptimed and the custom pkg repository. Until
// 2026-09-25 all of it was hand-made (f3s blog parts 2/4/5/6) and had
// drifted (task ck2):
//   - /etc/hosts carried the wg0 names twice (the blog's "IP short fqdn"
//     block and a later "# WireGuard mesh ..." block), a stray literal "EOF"
//     line on f3 (heredoc accident), no LAN rows for rocky and the Pis, and
//     a 192.168.2.138 f3s-storage-ha wg0 row for a VIP that no interface
//     carries (CARP runs on the LAN only) and nothing in this repo uses.
//     The t450 LAN row (retired 2026-09-25) is gone from all four.
//   - doas.conf was the untouched doas.conf.sample, so its alice/bob/cindy/
//     david example rules were live, plus the f3sctl agent rules.
//   - f3 had no uptimed.conf, so its uptimed ran with the default of 50
//     records instead of the fleet's unlimited LOG_MAXIMUM_ENTRIES=0.
//
// Register with OnCluster(cluster.NameFreeBSD); every task applies to all
// four hosts alike.
type Base struct {
	RequiresRoot
}

const (
	etcHosts       = "/etc/hosts"
	doasConf       = "/usr/local/etc/doas.conf"
	doasBin        = "/usr/local/bin/doas"
	uptimedConf    = "/usr/local/etc/uptimed.conf"
	pkgReposDir    = "/usr/local/etc/pkg/repos"
	pkgRepoCustom  = pkgReposDir + "/custom.conf"
	baseAssetsPath = "base/"
)

// basePackages is the interactive toolset of every f-host (blog part 2 plus
// what paul uses there). Role packages (apcupsd, stunnel, relayd, zrepl,
// vm-bhyve, ...) belong to their role recipes, uptimed to Uptimed. fish, go
// and ubench stay hand-installed where present: nothing on the hosts needs
// them.
var basePackages = List("bash", "doas", "git", "helix", "jq", "ksh", "rsync", "tmux")

// Hosts renders /etc/hosts (stock header, LAN and wg0 mesh rows from the
// shared etchosts inventory).
//
// Hosts renders the whole file: the rows were hand-appended and had to be
// de-duplicated anyway, so a line edit would leave the junk in place. The
// LAN and wg0 rows come from gonf/etchosts, the inventory the r-nodes'
// /etc/hosts (rnodes.Base.Hosts) shares; hosts.tmpl only holds the FreeBSD
// stock header.
func (Base) Hosts() {
	InstallFile(etcHosts, paths.FHostAsset(baseAssetsPath+"hosts.tmpl"),
		RootOwned, WithTemplateData(etchosts.TemplateData()))
}

// Doas installs doas.conf (nopass :wheel + f3sctl agent rules), validated
// with doas -C first.
//
// Doas replaces the sample-derived doas.conf. The candidate must parse with
// "doas -C" before it goes live: gonf elevates through doas itself, so a
// broken file would lock gonf and paul out of root. "permit nopass :wheel"
// keeps both working (paul is in wheel on every host; f3 even has wheel as
// his primary group). The live "permit :wheel" line was shadowed by it
// (doas applies the last matching rule) and is dropped. The file also
// carries the f3sctl agent-root rules: this is their only owner, F3sctl
// (f3sctl.go) manages the rest of the agent account and leaves doas.conf
// alone.
func (Base) Doas() {
	InstallFile(doasConf, paths.FHostAsset(baseAssetsPath+"doas.conf"),
		RootOwned, WithValidation(doasBin, List("-C", CandidatePath)))
}

// Packages installs the interactive base packages (bash doas git helix jq
// ksh rsync tmux).
func (Base) Packages() {
	Packages(basePackages...)
}

// Uptimed installs uptimed, its config (LOG_MAXIMUM_ENTRIES=0), enables and
// runs it; restarts on config change.
func (Base) Uptimed() {
	pkg := Package("uptimed")
	conf := InstallFile(uptimedConf, paths.FHostAsset(baseAssetsPath+"uptimed.conf"),
		RootOwned, DependsOn(pkg))
	Service("uptimed", WithRestart, OnChange(conf))
}

// DescPkgRepo returns the description for the custom pkg repository.
func (Base) DescPkgRepo() string {
	return "Install " + pkgRepoCustom + " (pkgrepo.f3s.buetow.org, dtail/f3sctl/...)"
}

// PkgRepo installs the client config of the homelab repository (f3s-pkgrepo
// skill). It carries no secret: the repo is unsigned and served over HTTPS.
func (Base) PkgRepo() {
	EnsureDir(pkgReposDir, RootOwned)
	InstallFile(pkgRepoCustom, paths.FHostAsset(baseAssetsPath+"pkg-custom.conf"), RootOwned)
}
