package cluster

import "github.com/snonux/conf/gonf/freebsd"

// Per-host zrepl jobs and ZFS key datasets of the f-hosts (task hk2),
// attached in registerFreeBSD. The values are the live ones of 2026-09-25
// (f3s-storage skill, references/zrepl.md and usb-keys.md): the blog's
// zrepl config is outdated (its f0_to_f1_freebsd job is gone; the freebsd VM
// moved to f3). Replication runs over the WireGuard mesh (192.168.2.13N).

// zreplLocalSnap is the local_zfs_snapshots filter entry every host starts
// with; the per-host lists append exclusions of datasets a push job or sink
// snapshots itself.
var zreplLocalSnap = freebsd.ZreplFilesystem{Pattern: "zroot<", Include: true}

// f0 pushes the NFS dataset to f1 every minute.
var (
	f0Zrepl = freebsd.ZreplJobs{
		Push: &freebsd.ZreplPush{
			Name:        "f0_to_f1_nfsdata",
			Peer:        "f1",
			Address:     "192.168.2.131:8888",
			Filesystems: []string{"zdata/enc/nfsdata"},
			Interval:    "1m",
			Grid:        "24x1h | 14x1d | 6x30d",
		},
		Snap: []freebsd.ZreplFilesystem{zreplLocalSnap},
	}
	f0Keys = freebsd.KeyDatasets{
		Datasets: []string{"zdata/enc", "zdata/enc/nfsdata", "zroot/bhyve", "zroot/garage"},
	}
)

// f1 receives f0's NFS dataset. The sink is an encryption root of its own
// (raw receive) and is unlocked with f0's key.
var (
	f1Zrepl = freebsd.ZreplJobs{
		Sink: &freebsd.ZreplSink{
			Listen: "192.168.2.131:8888", ClientIP: "192.168.2.130", ClientName: "f0",
			RootFS: "zdata/sink",
		},
		Snap: []freebsd.ZreplFilesystem{zreplLocalSnap},
	}
	f1Keys = freebsd.KeyDatasets{
		Datasets: []string{"zdata/enc", "zroot/bhyve", "zroot/garage", "zdata/sink/f0/zdata/enc/nfsdata"},
		SinkKeys: []freebsd.SinkKey{{
			Dataset: "zdata/sink/f0/zdata/enc/nfsdata",
			KeyFile: "/keys/f0.lan.buetow.org:zdata.key",
		}},
	}
)

// f2 receives f3's VM datasets below zroot/sink (no second disk) and also
// snapshots zdata locally. Only the freebsd VM's sink is unlocked at boot;
// zroot/sink/f3/zroot/bhyve/rocky stays locked (keylocation=prompt, not in
// zfskeys_datasets), as it was live.
var (
	f2Zrepl = freebsd.ZreplJobs{
		Sink: &freebsd.ZreplSink{
			Listen: "192.168.2.132:8888", ClientIP: "192.168.2.133", ClientName: "f3",
			RootFS: "zroot/sink",
		},
		Snap: []freebsd.ZreplFilesystem{
			zreplLocalSnap,
			{Pattern: "zroot/sink<", Include: false},
			{Pattern: "zdata<", Include: true},
		},
	}
	f2Keys = freebsd.KeyDatasets{
		Datasets: []string{"zdata/enc", "zroot/bhyve", "zroot/garage", "zroot/sink/f3/zroot/bhyve/freebsd"},
		SinkKeys: []freebsd.SinkKey{{
			Dataset: "zroot/sink/f3/zroot/bhyve/freebsd",
			KeyFile: "/keys/f3.lan.buetow.org:bhyve.key",
		}},
	}
)

// f3 pushes the freebsd and rocky VM datasets to f2 every 10 minutes.
var (
	f3Zrepl = freebsd.ZreplJobs{
		Push: &freebsd.ZreplPush{
			Name:        "f3_to_f2_freebsd",
			Peer:        "f2",
			Address:     "192.168.2.132:8888",
			Filesystems: []string{"zroot/bhyve/freebsd", "zroot/bhyve/rocky"},
			Interval:    "10m",
			Grid:        "24x1h | 14x1d",
		},
		Snap: []freebsd.ZreplFilesystem{
			zreplLocalSnap,
			{Pattern: "zroot/bhyve/freebsd", Include: false},
			{Pattern: "zroot/bhyve/rocky", Include: false},
		},
	}
	f3Keys = freebsd.KeyDatasets{Datasets: []string{"zroot/bhyve"}}
)
