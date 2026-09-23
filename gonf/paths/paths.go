// Package paths holds filesystem roots used by configuration tasks.
package paths

import (
	"path"
	"path/filepath"
)

// Repo roots used when declaring resources. Conf is this repository's
// checkout on the controller: ~/git/conf, or GONF_CONF_ROOT when set, which
// gonf.sh sets to the checkout it runs from so a second worktree records its
// own assets.
var (
	Conf      = checkoutRoot("GONF_CONF_ROOT", "conf")
	Frontends = filepath.Join(Conf, "frontends")
	Garage    = filepath.Join(Conf, "f3s", "garage")
	RNodes    = filepath.Join(Conf, "f3s", "r-nodes")
)

// FrontendAsset returns the controller-local path of a source-controlled
// frontend asset such as etc/httpd.conf.tpl.
func FrontendAsset(relativePath string) string {
	return filepath.Join(Frontends, relativePath)
}

// GarageAsset returns the controller-local path of a source-controlled Garage
// asset such as etc/garage.toml.tmpl.
func GarageAsset(relativePath string) string {
	return filepath.Join(Garage, relativePath)
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
