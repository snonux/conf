package freebsd

import (
	"strings"

	. "github.com/snonux/gonf/api"
)

// ZfsKeys manages the boot-time loading of the ZFS encryption keys on the
// f-hosts (f3s-storage skill, references/usb-keys.md): the rc.conf keys of
// the f3skeys service (mounts the F3S_KEYS USB stick on /keys) and of the
// base zfskeys service (loads the keys of zfskeys_datasets from their
// file:// keylocation), and the keylocation of the replicated encrypted
// zrepl sinks.
//
// The key files themselves (/keys/*.key, raw 32-byte keys on the USB stick)
// are secrets and never in git or in gonf: the recipe only points datasets
// at their paths. No key is loaded, changed or unloaded here.
//
// Register with OnCluster(cluster.NameFreeBSD); every f-host carries a
// KeyDatasets.
type ZfsKeys struct {
	RequiresRoot
}

// KeyDatasets is a FreeBSD host's key setup (host data, see gonf/cluster).
type KeyDatasets struct {
	// Datasets is zfskeys_datasets, in load order.
	Datasets []string
	// SinkKeys are the zrepl sinks received raw (send.encrypted) that must
	// carry a file:// keylocation; a subset of Datasets.
	SinkKeys []SinkKey
}

// SinkKey points a raw-received encryption root at the sender's key file.
type SinkKey struct {
	Dataset string
	KeyFile string // absolute path on the /keys stick
}

// staleZfskeysComment is the commented-out zfskeys_datasets f1 still carried
// from before zroot/garage existed; the live line below it replaced it.
const staleZfskeysComment = `#zfskeys_datasets="zdata/enc zroot/bhyve zdata/sink/f0/zdata/enc/nfsdata"`

// DescRcConf returns the description for the key-loading rc.conf keys.
func (ZfsKeys) DescRcConf() string {
	return "rc.conf: f3skeys_enable, zfskeys_enable, zfskeys_datasets per f-host (boot-time key loading from /keys)"
}

// RcConf owns the three lines via WithKeyedLine (replaced in place, appended
// when missing); the rest of rc.conf stays hand-managed or belongs to other
// recipes (NFS.RcConf), hence the WithName. rc.conf is read at boot, so a
// change takes effect on the next boot; f3s-load-zfs-keys reads
// zfskeys_datasets for a manual load at any time.
func (ZfsKeys) RcConf() {
	EachHost(func(k KeyDatasets) {
		File(rcConf,
			WithoutLine(staleZfskeysComment),
			WithKeyedLine("f3skeys_enable=", `f3skeys_enable="YES"`),
			WithKeyedLine("zfskeys_enable=", `zfskeys_enable="YES"`),
			WithKeyedLine("zfskeys_datasets=", `zfskeys_datasets="`+strings.Join(k.Datasets, " ")+`"`),
			Perm(0o644, Root),
			WithName("rc-conf-zfskeys"))
	})
}

// DescSinkKeyLocation returns the description for the sink keylocations.
func (ZfsKeys) DescSinkKeyLocation() string {
	return "zfs set keylocation=file:///keys/... on raw-received zrepl sinks (boot-time zfskeys can load them)"
}

// SinkKeyLocation sets the keylocation of each raw-received sink to the
// sender's key file. A raw receive creates the sink's encryption root with
// keylocation=prompt, and that is what f1's zdata/sink/f0/zdata/enc/nfsdata
// carried after its full re-seed on 2026-08-07 (task hk2): zfskeys then
// skips the file loader for it and runs "zfs load-key" against the console,
// and f3s-load-zfs-keys refuses it, so only carpcontrol.sh's explicit
// "load-key -L file://..." unlocked it. Setting keylocation changes neither
// the key nor the wrapping key, and later incremental raw receives leave the
// local property alone (f2's freebsd sink has kept its file:// location
// since 2026-05). The guard re-applies it after a future re-seed, and never
// points a dataset at a key file missing from the stick (/keys unmounted).
func (ZfsKeys) SinkKeyLocation() {
	EachHost(func(k KeyDatasets) {
		for _, sink := range k.SinkKeys {
			location := "file://" + sink.KeyFile
			Command("zfs", List("set", "keylocation="+location, sink.Dataset),
				OnlyIf("sh", List("-c", sinkKeyLocationGuard(sink.Dataset, sink.KeyFile, location))),
				WithName("keylocation-"+sink.Dataset))
		}
	})
}

// sinkKeyLocationGuard is true when the key file is on the stick and the
// dataset exists with a different keylocation (grep -v selects the one
// output line unless it is the wanted location; a missing dataset prints
// nothing, so the guard is false).
func sinkKeyLocationGuard(dataset, keyFile, location string) string {
	return `[ -s '` + keyFile + `' ] && zfs get -H -o value keylocation '` + dataset +
		`' 2>/dev/null | grep -qvx '` + location + `'`
}
