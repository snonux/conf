// Package paths holds filesystem roots used by configuration tasks.
package paths

import (
	"path"
	"path/filepath"
)

// Repo roots used when declaring resources. Conf is this repository's
// checkout on the controller: ~/git/conf, or GONF_CONF_ROOT when set, which
// gonf.sh sets to the checkout it runs from so a second worktree records its
// own assets. Recipes do not join onto these roots themselves: they go
// through the *Asset helpers below, so each tree has one path-join
// implementation and a rename only needs an edit here.
var (
	Conf      = checkoutRoot("GONF_CONF_ROOT", "conf")
	Frontends = filepath.Join(Conf, "frontends")
	Garage    = filepath.Join(Conf, "f3s", "garage")
	FHosts    = filepath.Join(Conf, "f3s", "freebsd-hosts")
	Pihole    = filepath.Join(Conf, "f3s", "pihole", "docker-pi")
	RNodes    = filepath.Join(Conf, "f3s", "r-nodes")
)

// FrontendAsset returns the controller-local path of a source-controlled
// asset under the repository's top-level frontends/ tree, such as
// etc/pf.conf.tpl or scripts/unattended-upgrade.sh. It is the one
// implementation of that path-join: the frontends package reaches it through
// legacyFrontendAsset, and the rocky, openbsd, freebsd and netbsd packages
// call it directly for the scripts/ and systemd/ assets they share.
func FrontendAsset(relativePath string) string {
	return filepath.Join(Frontends, relativePath)
}

// GonfAsset returns the controller-local path of a source-controlled asset
// under a gonf/<pkg> package's own assets directory, such as
// gonf/openbsd/assets/unattended-upgrade-services. It is the one shared
// implementation of that path-join for every gonf/<pkg>/assets consumer
// (frontends, openbsd, freebsd, netbsd), so a rename of the per-package
// assets directory only needs to change here.
func GonfAsset(pkg, name string) string {
	return filepath.Join(Conf, "gonf", pkg, "assets", name)
}

// GarageAsset returns the controller-local path of a source-controlled Garage
// asset such as etc/garage.toml.tmpl.
func GarageAsset(relativePath string) string {
	return filepath.Join(Garage, relativePath)
}

// FHostAsset returns the controller-local path of a source-controlled
// FreeBSD f-host asset such as carp/carp-auto-failback.sh.
func FHostAsset(relativePath string) string {
	return filepath.Join(FHosts, relativePath)
}

// PiholeAsset returns the controller-local path of a source-controlled
// Pi-hole (pi2/pi3 Docker) asset such as docker-compose.yml.
func PiholeAsset(relativePath string) string {
	return filepath.Join(Pihole, relativePath)
}

// RNodeAsset returns the controller-local path of a source-controlled r-node
// asset such as nfs-mount-monitor/nfs-mount-monitor.service.
func RNodeAsset(relativePath string) string {
	return filepath.Join(RNodes, relativePath)
}

// FrontendSecret returns the logical path passed to api.MustSecret or
// api.OptionalSecret. Secret helpers resolve it beneath ./secrets, not beneath
// the source asset directory: FrontendSecret("var/nsd/etc/nsd_key.txt")
// resolves to ./secrets/frontends/var/nsd/etc/nsd_key.txt.
func FrontendSecret(relativePath string) string {
	return path.Join("frontends", relativePath)
}

// GarageSecret returns the logical path passed to api.MustSecret or
// api.OptionalSecret for Garage controller secrets.
func GarageSecret(relativePath string) string {
	return path.Join("garage", relativePath)
}
