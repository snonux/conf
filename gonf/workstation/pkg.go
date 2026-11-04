package workstation

import (
	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

// Pkg contains tasks that mutate the system package database.
type Pkg struct {
	RequiresRoot
}

// Opendoas installs opendoas and a passwordless wheel doas.conf (Fedora).
//
// Opendoas is a small, always-safe package task separate from the full
// workstation set in Fedora: every Fedora host should have doas even when
// pkg_fedora is not applied. Kept out of Fedora()'s Packages list so both
// tasks can run in one plan without a duplicate Package[opendoas].
// /etc/doas.conf is converged to permit nopass :wheel (Fedora's stock
// comment line kept).
func (Pkg) Opendoas() {
	pkg := Package("opendoas")
	InstallFile("/etc/doas.conf", paths.GonfAsset("workstation", "doas.conf"), RootOwned, DependsOn(pkg))
}

// FishTools installs fzf and zoxide (Fedora).
//
// FishTools keeps the interactive fish toolchain present on every Fedora
// host even when the full pkg_fedora workstation set is not applied. fzf is
// kept out of Fedora()'s Packages list so both tasks can share one plan.
func (Pkg) FishTools() {
	Packages("fzf", "zoxide")
}

// Taskwarrior installs Taskwarrior 3.x (Fedora package "task").
//
// Taskwarrior is separate from the full pkg_fedora workstation set so every
// Fedora host gets the CLI even when that set is not applied. The package
// name is "task" (3.x); Fedora's "task2" is the retired 2.x line. Kept out
// of Fedora()'s Packages list so both tasks can share one plan.
func (Pkg) Taskwarrior() {
	Package("task")
}

// Helix installs the helix editor (Fedora).
//
// Helix is separate from the full pkg_fedora workstation set so every Fedora
// host gets hx even when that set is not applied. home_helix only syncs
// ~/.config/helix.
func (Pkg) Helix() {
	Package("helix")
}

// Fedora installs Fedora packages.
//
// Fedora installs the workstation package set. It also uninstalls the Rex
// deployment tool: every Rexfile it used to run (this repo's and conf's) has
// been ported to gonf and retired, so an explicit absent resource converges
// hosts that still carry it instead of merely no longer installing it.
// aerc is part of the set, converging home_aerc's /usr/bin/aerc prerequisite
// with the workstation packages instead of leaving it to a manual install.
// gron is gone: Fedora retired its package, so the entry would fail every
// dnf apply; reinstall with "go install github.com/tomnomnom/gron@latest"
// if it is ever needed again.
// opendoas lives in Opendoas; fzf and zoxide in FishTools; task in Taskwarrior;
// helix in Helix.
func (Pkg) Fedora() {
	NoPackage("Rex")
	Packages(
		"fd-find",
		"nodejs-bash-language-server",
		"fortune-mod",
		"syncthing",
		"ncdu",
		"ack",
		"aerc",
		"bat",
		"ctags",
		"golang",
		"gopls",
		"gpaste",
		"htop",
		"java-latest-openjdk-devel",
		"lynx",
		"make",
		"nodejs22",
		"perl-File-Slurp",
		"ripgrep",
		"ruby",
		"sqlite",
		"strace",
		"tig",
		"tmux",
		"strawberry",
		"sway-config-fedora",
		"sway",
		"waybar",
		"zathura",
		"flameshot",
	)
}
