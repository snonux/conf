package freebsd

import (
	. "github.com/snonux/gonf/api"
)

// Loader owns individual lines of /boot/loader.conf on the f-hosts. The file
// itself stays hand-managed: the install-time kern.geom.label.* and zfs_load
// lines stay as they are, and carp_load belongs to Carp.LoaderConf (f0/f1
// only), so this recipe never renders the whole file. Each managed setting is a
// WithKeyedLine: the line starting with the setting's name is replaced in
// place (a changed value or comment converges, a duplicate is dropped), and
// appended when missing.
//
// loader.conf is read only at boot, so a change takes effect on the next
// reboot (the nightly f3sctl power cycle); nothing is reloaded here.
//
// Register with OnCluster(cluster.NameFreeBSD); every f-host carries a
// BaselineHost (its Cryptodev field).
type Loader struct {
	RequiresRoot
}

const loaderConf = "/boot/loader.conf"

// msgbufTimestampLine prefixes every kernel and console message with
// [uptime-seconds] from early boot, so dmesg -a times rc stalls (the ~6.5 min
// f1 boot stall) and the lead-up to a panic. Set by hand on f0-f3 on
// 2026-09-25 during the ZFS-taskq panic investigation (f3s skill,
// references/kernel-panics.md); the text, comment included, is the line
// written then, so the hosts converge without a change.
const msgbufTimestampLine = `kern.msgbuf_show_timestamp="1"` + "\t" +
	`# [uptime-seconds] prefix on kernel+console msgs from early boot; to time rc stalls and panics (see f3s skill kernel-panics.md)`

// efiResolutionLine restores the 1080p efifb console: FreeBSD 15.1 defaulted
// the GOP mode to 1x1, which fell back to vga 640x480, and the JetKVM
// captures only the 1080p mode (720p and 640x480 show no signal). Set by hand
// on 2026-06-27. f0, f2 and f3 carried this exact comment; f1 carried a
// different comment with the same value and was aligned to this one.
const efiResolutionLine = `efi_max_resolution="1080p"` + "\t" +
	`# restore efifb 1080p HDMI console (FreeBSD 15.1 defaulted to 1x1 -> vga 640x480, broke JetKVM)`

// pcidDisabledLine turns off PCID in the amd64 pmap. Panic analysis (task
// zj2, 2026-09-25, f3s skill references/kernel-panics.md sections 5-6): f0-f2
// panicked in ~10 % of boot/shutdown cycles, always in a ZFS taskq thread at
// sched_ule_sswitch+0x888 (the retq of the context switch) or at IP 0/garbage,
// only while ZFS creates or destroys taskq kthreads in bulk (zfs mount -a,
// pool export). In several vmcores the dumped kernel stack holds a valid
// sched_switch frame while the CPU popped garbage from it: the TLB mapped
// the kstack VA to a different physical page than the page tables (a stale
// global TLB entry). The N100 (Alder Lake-N/Gracemont, CPUID 0xb06e0) has
// the INVLPG-does-not-flush-global-entries-with-PCID erratum, and Linux
// disables PCID on it. With PTI off, PCID buys almost nothing, so this is
// both the discriminating test and the probable fix: expect no new
// /var/crash dumps over ~20 power cycles per host.
const pcidDisabledLine = `vm.pmap.pcid_enabled="0"` + "\t" +
	`# N100 INVLPG/PCID erratum: stale kstack TLB entries -> ZFS taskq panics in sched_ule_sswitch (zj2, f3s skill kernel-panics.md)`

// microcodeLoadLine and microcodeNameLine make the loader apply the Intel
// microcode update early, before the kernel starts (none was loaded before).
// The file comes from cpu-microcode-intel (Microcode recipe); its pkg-message
// and the FreeBSD Handbook give exactly these two lines. The third knob,
// cpu_microcode_type="cpu_microcode", is already the default in
// /boot/defaults/loader.conf. Same panic investigation as pcidDisabledLine;
// microcode may carry the TLB erratum fix, PCID off works around it anyway.
const microcodeLoadLine = `cpu_microcode_load="YES"` + "\t" +
	`# early-load Intel microcode (cpu-microcode-intel; zj2 panic fix, f3s skill kernel-panics.md)`

const microcodeNameLine = `cpu_microcode_name="/boot/firmware/intel-ucode.bin"`

// coretempLoadLine loads coretemp(4), the per-core DTS temperatures
// (dev.cpu.N.temperature) node_exporter scrapes; without it the only reading
// is the constant ACPI tz0. In loader.conf on f0-f3 since 2026-05-17.
const coretempLoadLine = `coretemp_load="YES"`

// cryptodevLoadLine loads cryptodev(4) (/dev/crypto) on the hosts whose
// BaselineHost sets Cryptodev (f0-f2, as installed; f3 never had it).
const cryptodevLoadLine = `cryptodev_load="YES"`

// DescConf returns the description for the managed loader.conf lines.
func (Loader) DescConf() string {
	return "loader.conf lines: msgbuf timestamps, efi 1080p, vm.pmap.pcid_enabled=0, Intel microcode early load, coretemp/cryptodev (next boot)"
}

// OptsConf orders the loader lines after the microcode package, so
// cpu_microcode_name never points at a missing file.
func (Loader) OptsConf() TaskOptions {
	return TaskOptions{Needs("freebsd_microcode_package")}
}

// Conf manages the lines in place; the rest of loader.conf is untouched.
// Each key ends in "=" so no key is a prefix of another, and the full
// module names keep the *_load keys apart from the hand-kept zfs_load and
// Carp's carp_load. cryptodev_load is managed only where BaselineHost.Cryptodev is
// set; elsewhere the line is neither added nor removed.
func (Loader) Conf() {
	EachHost(func(h BaselineHost) {
		opts := []FileOption{
			WithKeyedLine(`kern.msgbuf_show_timestamp=`, msgbufTimestampLine),
			WithKeyedLine(`efi_max_resolution=`, efiResolutionLine),
			WithKeyedLine(`vm.pmap.pcid_enabled=`, pcidDisabledLine),
			WithKeyedLine(`cpu_microcode_load=`, microcodeLoadLine),
			WithKeyedLine(`cpu_microcode_name=`, microcodeNameLine),
			WithKeyedLine(`coretemp_load=`, coretempLoadLine),
			Perm(0o644, Root),
		}
		if h.Cryptodev {
			opts = append(opts, WithKeyedLine(`cryptodev_load=`, cryptodevLoadLine))
		}
		File(loaderConf, opts...)
	})
}
