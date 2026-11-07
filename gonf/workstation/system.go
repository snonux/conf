package workstation

import (
	"path/filepath"
	"strings"

	"github.com/snonux/conf/gonf/etchosts"
	"github.com/snonux/conf/gonf/fleetfiles"
	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

// System contains root-owned configuration outside the package database.
// Per-task Opts* guards pick the host: Hosts and Wireguard are earth-only;
// Uptimed and Timezone run on earth and zen; FishShell on earth, zen and
// rocky.
type System struct {
	RequiresRoot
}

const (
	etcHosts     = "/etc/hosts"
	etcLocaltime = "/etc/localtime"
	wireGuardDir = "/etc/wireguard"
	uptimedConf  = "/etc/uptimed.conf"

	// timeZone is the fleet's zone: the laptops live in Sofia, Bulgaria.
	timeZone = "Europe/Sofia"
)

// wireGuardConfigs are the tunnel configs gonf keeps private. Their content
// has other owners: wg0.conf is rendered and installed by
// ~/git/wireguardmeshgenerator (run it through conf's by-name task
// wireguard_mesh_install), wg1.conf is replaced (and extended with peers) by
// the hyperstack tooling's wg1-setup.sh on every VM create.
var wireGuardConfigs = List(wireGuardDir+"/wg0.conf", wireGuardDir+"/wg1.conf")

func (System) OptsHosts() TaskOptions { return TaskOptions{WhenHostnameContains("earth")} }

// Hosts manages the fleet block of /etc/hosts (earth).
//
// Hosts owns only the "# BEGIN GONF fleet" block of /etc/hosts: the LAN and
// wg0 mesh rows from the etchosts package. Everything outside the markers stays
// as it is: Fedora's stock loopback header, and the hyperstack*.wg1 rows that
// wg1-setup.sh deletes and re-appends with sed/tee on every VM create (its
// sed only matches lines ending in the VM's own wg1 name, never a fleet row).
// Mode 0644 root:root is Fedora's stock for the file; before gonf it had
// drifted to 0664 root:wheel.
func (System) Hosts() {
	fleetfiles.MergeHostsIn(etchosts.EarthHostsBlock(), "/usr/local/bin")
}

func (System) OptsWireguard() TaskOptions {
	return TaskOptions{WhenHostnameContains("earth")}
}

// Wireguard keeps /etc/wireguard and the wg0/wg1 configs private (earth,
// perms only).
//
// Wireguard (task system_wireguard) manages config permissions without
// rewriting content: the directory is 0700 root:root and each existing tunnel
// config 0600 root:root, since they hold private keys. A config that does not exist
// (no hyperstack VM yet) is not created.
//
// wg-quick@wg0 and wg-quick@wg1 are enabled for auto-start at boot (user
// decision 2026-09-25) for each tunnel whose config exists. Enable only, via
// Service WithEnableOnly: a deploy must never bring a tunnel up or down
// on this roaming laptop.
//
// The directory also holds files neither generator writes (dated backups,
// the single-gateway variants wg0-blowfish.conf and wg0-fishfinger.conf).
// They are 0600 already and are left alone: deleting them is the user's
// decision.
func (System) Wireguard() {
	Dir(wireGuardDir, RootPrivate)
	for _, conf := range wireGuardConfigs {
		WhenPathExists(conf, func() {
			EnsureFile(conf, RootPrivate)
			unit := "wg-quick@" + strings.TrimSuffix(filepath.Base(conf), ".conf")
			Service(unit, WithEnableOnly)
		})
	}
}

func (System) OptsUptimed() TaskOptions {
	return TaskOptions{WhenHostnameIn("earth", "zen")}
}

// Uptimed installs and enables the uptimed daemon (earth, zen).
//
// Uptimed installs the Fedora package, deploys /etc/uptimed.conf (earth's
// template: LOG_MAXIMUM_ENTRIES=0 so records are kept forever), and enables
// the system service. Config changes restart the daemon.
func (System) Uptimed() {
	pkg := Package("uptimed")
	conf := InstallFile(uptimedConf, paths.GonfAsset("goprecords", "uptimed.conf"), RootOwned, DependsOn(pkg))
	Service("uptimed", WithRestart, OnChange(conf), DependsOn(pkg, conf))
}

func (System) OptsFishShell() TaskOptions {
	return TaskOptions{WhenHostnameIn("earth", "zen", "rocky")}
}

// FishShell installs fish and sets it as paul's login shell (earth, zen, rocky).
//
// FishShell installs the fish package first, then manages paul's login
// shell without changing an existing account's other settings.
// On rocky, paul has no general sudo: push as root@rocky.lan.
func (System) FishShell() {
	pkg := Package("fish")
	User("paul", WithShell("/usr/bin/fish"), WithManageShell, DependsOn(pkg))
}

func (System) OptsTimezone() TaskOptions {
	return TaskOptions{WhenLinux(), WhenHostnameIn("earth", "zen")}
}

// Timezone points /etc/localtime at Europe/Sofia (earth, zen).
//
// Timezone writes the same ../usr/share/zoneinfo/Europe/Sofia symlink
// timedatectl set-timezone writes, so a zone changed by hand (or left
// behind while roaming) is repointed on the next apply. The link resource
// repoints an existing symlink in place and refuses a dangling target;
// the clock itself stays NTP-managed and is not touched.
func (System) Timezone() {
	Symlink(etcLocaltime, "../usr/share/zoneinfo/"+timeZone)
}
