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
//     The t450 LAN row, present on f0 and f3 only, is now on all four.
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

// DescHosts returns the description for /etc/hosts.
func (Base) DescHosts() string {
	return "Render /etc/hosts (stock header, LAN and wg0 mesh rows from the shared etchosts inventory)"
}

// Hosts renders the whole file: the rows were hand-appended and had to be
// de-duplicated anyway, so a line edit would leave the junk in place. The
// LAN and wg0 rows come from gonf/etchosts, the inventory the r-nodes'
// /etc/hosts (rnodes.Base.Hosts) shares; hosts.tmpl only holds the FreeBSD
// stock header.
func (Base) Hosts() {
	InstallFile(etcHosts, paths.FHostAsset(baseAssetsPath+"hosts.tmpl"),
		Perm(0o644, Root), WithTemplateData(etchosts.TemplateData()))
}

// DescDoas returns the description for doas.conf.
func (Base) DescDoas() string {
	return "Install doas.conf (nopass :wheel + f3sctl agent rules), validated with doas -C first"
}

// Doas replaces the sample-derived doas.conf. The candidate must parse with
// "doas -C" before it goes live: gonf elevates through doas itself, so a
// broken file would lock gonf and paul out of root. "permit nopass :wheel"
// keeps both working (paul is in wheel on every host; f3 even has wheel as
// his primary group). The live "permit :wheel" line was shadowed by it
// (doas applies the last matching rule) and is dropped.
func (Base) Doas() {
	InstallFile(doasConf, paths.FHostAsset(baseAssetsPath+"doas.conf"),
		Perm(0o644, Root), WithValidation(doasBin, List("-C", CandidatePath)))
}

// DescPackages returns the description for the base packages.
func (Base) DescPackages() string {
	return "Install the interactive base packages (bash doas git helix jq ksh rsync tmux)"
}

// Packages installs basePackages.
func (Base) Packages() {
	Packages(basePackages...)
}

// DescUptimed returns the description for uptimed.
func (Base) DescUptimed() string {
	return "Install uptimed, its config (LOG_MAXIMUM_ENTRIES=0), enable and run it; restart on config change"
}

// Uptimed installs the package and f0-f2's config (identical on the three;
// it is the package sample with LOG_MAXIMUM_ENTRIES=0, keeping every record
// for goprecords) and restarts the daemon only when the config changed.
func (Base) Uptimed() {
	pkg := Package("uptimed")
	conf := InstallFile(uptimedConf, paths.FHostAsset(baseAssetsPath+"uptimed.conf"),
		Perm(0o644, Root), DependsOn(pkg))
	Service("uptimed", WithRestart, OnChange(conf))
}

// DescPkgRepo returns the description for the custom pkg repository.
func (Base) DescPkgRepo() string {
	return "Install " + pkgRepoCustom + " (pkgrepo.f3s.buetow.org, dtail/f3sctl/...)"
}

// PkgRepo installs the client config of the homelab repository (f3s-pkgrepo
// skill). It carries no secret: the repo is unsigned and served over HTTPS.
func (Base) PkgRepo() {
	EnsureDir(pkgReposDir, Perm(0o755, Root))
	InstallFile(pkgRepoCustom, paths.FHostAsset(baseAssetsPath+"pkg-custom.conf"), Perm(0o644, Root))
}
