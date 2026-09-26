package freebsd

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// Zusb installs the load/unload scripts of the removable zusb backup pool
// (f3s-storage skill, references/usb-keys.md) on every f-host, so the USB
// disk stack can be re-plugged into any of them and loaded there without
// per-host setup. Sources live in f3s/freebsd-hosts/zusb.
//
// Until task ik2 the scripts were copied by hand: f0/f1 carried the current
// repo version, while f2/f3 still ran the first commit (da0270e), which
// imports and exports the pool without the USB power sequence (serial-based
// usbconfig power_on/off and SCSI START/STOP UNIT from f957e62). Installing
// the repo version fixes that drift.
//
// gonf never runs the scripts: the pool is loaded and unloaded by hand about
// once a quarter, and the raw key /keys/zusb.key stays on the USB key stick.
//
// Register with OnCluster(cluster.NameFreeBSD).
type Zusb struct {
	RequiresRoot
}

const (
	zusbLoadScript   = "/usr/local/bin/zusb-load"
	zusbUnloadScript = "/usr/local/bin/zusb-unload"
)

// Scripts installs zusb-load and zusb-unload to /usr/local/bin (0755
// root:wheel; not run).
func (Zusb) Scripts() {
	InstallFile(zusbLoadScript, paths.FHostAsset("zusb/zusb-load"), RootExec)
	InstallFile(zusbUnloadScript, paths.FHostAsset("zusb/zusb-unload"), RootExec)
}
