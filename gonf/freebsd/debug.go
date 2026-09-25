package freebsd

import (
	. "github.com/snonux/gonf/api"
)

// Debug installs the tools used to read a kernel crash dump on the f-hosts.
//
// Added on 2026-09-25 while investigating fleet-wide ZFS-taskq kernel panics
// on f0-f3 (FreeBSD 15.1): the gdb package provides /usr/local/bin/kgdb,
// which reads /var/crash/vmcore.N after savecore(8) (see the f3s skill,
// references/kernel-panics.md). It was first installed by hand on all four;
// this recipe keeps it across reinstalls. The kernel debug symbols
// (kernel-dbg) are deliberately not installed.
//
// The matching loader tunable (kern.msgbuf_show_timestamp) lives in Loader,
// the one recipe that edits /boot/loader.conf.
//
// Register with OnCluster(cluster.NameFreeBSD).
type Debug struct {
	RequiresRoot
}

// DescGdb returns the description for the gdb package.
func (Debug) DescGdb() string {
	return "Install gdb (kgdb for /var/crash/vmcore.N)"
}

// Gdb installs the gdb package.
func (Debug) Gdb() {
	Packages("gdb")
}
