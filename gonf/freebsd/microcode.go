package freebsd

import (
	. "github.com/snonux/gonf/api"
)

// Microcode installs the Intel CPU microcode for early loading by the boot
// loader on the f-hosts (Intel N100, CPUID 0xb06e0).
//
// Added on 2026-09-25 as part of the ZFS-taskq kernel panic fix (task zj2,
// f3s skill references/kernel-panics.md): no microcode update was loaded on
// f0-f3, and the N100 is an Alder Lake-N part with TLB/PCID errata. The
// package ships /boot/firmware/intel-ucode.bin (plus split per-CPU files in
// /usr/local/share/cpucontrol for late loading via cpucontrol(8), unused
// here); its pkg-message and the FreeBSD Handbook ("Updating CPU microcode")
// name exactly that file for cpu_microcode_name in loader.conf.
//
// The loader.conf lines live in Loader, the one recipe that edits
// /boot/loader.conf; Loader.OptsConf needs this task, so the firmware file
// exists before the loader is told to load it.
//
// Register with OnCluster(cluster.NameFreeBSD).
type Microcode struct {
	RequiresRoot
}

// Package installs cpu-microcode-intel (/boot/firmware/intel-ucode.bin for
// early loading).
func (Microcode) Package() {
	Packages("cpu-microcode-intel")
}
