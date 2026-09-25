package freebsd

import (
	. "github.com/snonux/gonf/api"
)

// Loader owns individual lines of /boot/loader.conf on the f-hosts. The file
// itself stays hand-managed: each host keeps its own kern.geom.label.*,
// cryptodev/zfs/carp/coretemp *_load lines, so this recipe never renders the
// whole file. Each managed setting is a WithKeyedLine: the line starting with
// the setting's name is replaced in place (a changed value or comment
// converges, a duplicate is dropped), and appended when missing.
//
// loader.conf is read only at boot, so a change takes effect on the next
// reboot; nothing is reloaded here.
//
// Register with OnCluster(cluster.NameFreeBSD).
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

// DescConf returns the description for the managed loader.conf lines.
func (Loader) DescConf() string {
	return "loader.conf lines: kern.msgbuf_show_timestamp=1, efi_max_resolution=1080p (next boot)"
}

// Conf manages the two lines in place; the rest of loader.conf is untouched.
func (Loader) Conf() {
	File(loaderConf,
		WithKeyedLine(`kern.msgbuf_show_timestamp=`, msgbufTimestampLine),
		WithKeyedLine(`efi_max_resolution=`, efiResolutionLine),
		Perm(0o644, Root))
}
