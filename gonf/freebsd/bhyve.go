package freebsd

import (
	"strings"

	. "github.com/snonux/gonf/api"
)

// Bhyve manages vm-bhyve on the f-hosts: its packages, its rc.conf keys, the
// settings of the managed guests' <vm>.conf, and the loopback VNC binding of
// every graphical guest (f3s blog part 4 plus the NVMe switch; values are
// the live ones of 2026-09-25, task ek2 -- the blog's memory=14G for r0-r2 is
// outdated, they run with 12G).
//
// Restart policy: nothing here stops, starts or restarts a VM. vm-bhyve reads
// <vm>.conf once when "vm start" launches the guest (vm-run loads it before
// its run loop), and rc.conf at boot, so every change takes effect on the next
// full stop/start of a guest -- in practice the nightly f3sctl power cycle --
// not on a guest-internal reboot. The r0-r2 k3s VMs and f3's rocky VM are
// deliberately never bounced from gonf.
//
// Deliberately NOT managed (bootstrap or per-VM identity, see the f3s skill
// references bootstrap-rocky-bhyve.md and rocky-linux-vms.md):
//   - uuid and network0_mac: per-VM identity; a new MAC would break the
//     guest's DHCP/NIC config.
//   - disk images, ISOs, VM creation and OS install.
//   - the vm switch "public" on re0 (/zroot/bhyve/.config/system.conf).
//   - the /bhyve -> /zroot/bhyve symlink (f3's target carries a trailing
//     slash; both work, the link only exists for the blog's command lines).
//   - guests not listed in BhyveHost.Guests: the stopped "fedora" VM on f0
//     (left for the user to decide on) and the stopped bhyveload "freebsd"
//     test VM on f3. Their configs are untouched except for the VNC binding
//     (VNCListen covers every graphical guest), and they are not autostarted.
//
// Register with OnCluster(cluster.NameFreeBSD); every f-host carries a
// BhyveHost.
type Bhyve struct {
	RequiresRoot
}

// BhyveHost is an f-host's vm-bhyve setup (host data, see gonf/cluster).
type BhyveHost struct {
	// Guests are the guests whose <vm>.conf settings gonf owns. vm_list (the
	// autostart order) is the names of those with Autostart set, so a guest
	// can only be autostarted when it is managed here.
	Guests []BhyveGuest
}

// BhyveGuest is one managed vm-bhyve guest. Every managed guest is a UEFI
// Linux guest with one NVMe disk on the "public" switch (bhyveGuestKeys);
// only the fields below differ between them.
type BhyveGuest struct {
	// Name is the vm-bhyve name: the config is /zroot/bhyve/<Name>/<Name>.conf.
	Name string
	// CPU and Memory are vm-bhyve's cpu= and memory= values ("4", "12G").
	CPU    string
	Memory string
	// GraphicsWait is graphics_wait= ("no" on f3's rocky, which set it at
	// install time); "" leaves the key unmanaged, so vm-bhyve's default
	// (wait only when installing) applies where it is absent.
	GraphicsWait string
	// Autostart lists the guest in vm_list.
	Autostart bool
}

// bhyveVNCListen is the address vm-bhyve's VNC framebuffer binds.
const bhyveVNCListen = "127.0.0.1"

// bhyveVMDir is where vm_dir="zfs:zroot/bhyve" mounts the guests.
const bhyveVMDir = "/zroot/bhyve"

// bhyveVMDelay is vm_delay, the seconds between two autostarted guests.
const bhyveVMDelay = "5"

// bhyveGuestKeys are the <vm>.conf settings shared by every managed guest,
// in the live spelling (graphics_vga unquoted as vm-bhyve's template wrote
// it). disk0_type="nvme" replaced virtio-blk (blog NVMe update; the guest
// side, dracut/lvm, belongs to the r-node base recipe). graphics="yes" with
// graphics_vga=io gives the guest a UEFI framebuffer console over VNC.
var bhyveGuestKeys = []struct{ key, line string }{
	{"guest", `guest="linux"`},
	{"loader", `loader="uefi"`},
	{"uefi_vars", `uefi_vars="yes"`},
	{"network0_type", `network0_type="virtio-net"`},
	{"network0_switch", `network0_switch="public"`},
	{"disk0_type", `disk0_type="nvme"`},
	{"disk0_name", `disk0_name="disk0.img"`},
	{"graphics", `graphics="yes"`},
	{"graphics_vga", `graphics_vga=io`},
}

// DescPackages returns the description for the vm-bhyve packages.
func (Bhyve) DescPackages() string {
	return "pkg: vm-bhyve, bhyve-firmware (UEFI firmware for the guests)"
}

// Packages installs vm-bhyve and the UEFI firmware the guests boot from
// (bhyve-firmware pulls in edk2-bhyve).
func (Bhyve) Packages() {
	Packages("vm-bhyve", "bhyve-firmware")
}

// DescRcConf returns the description for the vm-bhyve rc.conf keys.
func (Bhyve) DescRcConf() string {
	return "rc.conf: vm_enable, vm_dir=zfs:zroot/bhyve, vm_list (autostart), vm_delay (next boot)"
}

// OptsRcConf orders the keys after the packages, so vm_enable never names a
// missing rc script.
func (Bhyve) OptsRcConf() TaskOptions {
	return TaskOptions{Needs("freebsd_bhyve_packages")}
}

// RcConf owns the four vm_* lines via WithKeyedLine (replaced in place,
// appended when missing); the rest of rc.conf belongs to other recipes or
// stays hand-managed. Read at boot only; nothing is restarted.
func (Bhyve) RcConf() {
	EachHost(func(h BhyveHost) {
		File(rcConf,
			rcConfKeyedLine("vm_enable", "YES"),
			rcConfKeyedLine("vm_dir", "zfs:zroot/bhyve"),
			rcConfKeyedLine("vm_list", strings.Join(h.autostart(), " ")),
			rcConfKeyedLine("vm_delay", bhyveVMDelay),
			Perm(0o644, Root),
			WithName("rc-conf-bhyve"))
	})
}

// DescGuests returns the description for the guest config settings.
func (Bhyve) DescGuests() string {
	return "vm-bhyve guest .conf: guest/loader/uefi, cpu, memory, nvme disk0, network0, graphics (next vm stop/start)"
}

// Guests owns the settings of each managed guest's config line by line, so
// uuid, network0_mac, graphics_listen (VNCListen) and any hand-added line are
// kept. A config that does not exist is skipped (WhenPathExists): a keyed
// file would otherwise create a half guest without uuid or MAC; creating a
// VM is bootstrap. Never restarts a guest; see the type comment.
func (Bhyve) Guests() {
	EachHost(func(h BhyveHost) {
		for _, g := range h.Guests {
			conf := bhyveGuestConf(g.Name)
			WhenPathExists(conf, func() {
				File(conf, append(g.keyedLines(), Perm(0o644, Root))...)
			})
		}
	})
}

// DescVNCListen returns the description for the VNC binding.
func (Bhyve) DescVNCListen() string {
	return "vm-bhyve guests: graphics_listen=" + bhyveVNCListen + " (VNC on loopback; applies on the next vm stop/start)"
}

// OptsVNCListen runs the binding after Guests, so the sed rewrite and the
// keyed-line file never interleave on the same <vm>.conf.
func (Bhyve) OptsVNCListen() TaskOptions {
	return TaskOptions{Needs("freebsd_bhyve_guests")}
}

// VNCListen sets graphics_listen in every graphical guest's config, managed
// or not. vm-bhyve binds a guest's UEFI framebuffer (VNC, no password) to
// graphics_listen, default 0.0.0.0, so until 2026-09-25 every rocky/r-VM
// console was open on port 5900 of every interface, wg0 included (audit item
// 15, f3s/docs/audit-2026-09-25.md). Loopback only; reach a console through
// an SSH tunnel instead:
//
//	ssh -p22 -L 5900:127.0.0.1:5900 f0.lan.buetow.org   # then vnc://localhost:5900
//
// It does not restart any guest; see the type comment.
func (Bhyve) VNCListen() {
	Command("sh", List("-c", bhyveVNCScript(true)),
		OnlyIf("sh", List("-c", bhyveVNCScript(false))))
}

// autostart returns the names of the guests in vm_list, in Guests order.
func (h BhyveHost) autostart() []string {
	var names []string
	for _, g := range h.Guests {
		if g.Autostart {
			names = append(names, g.Name)
		}
	}
	return names
}

// keyedLines returns the file options owning the guest's managed settings.
// Each key ends in "=" so no key is a prefix of another (graphics= does not
// match graphics_vga= or graphics_listen=).
func (g BhyveGuest) keyedLines() []FileOption {
	opts := make([]FileOption, 0, len(bhyveGuestKeys)+3)
	for _, k := range bhyveGuestKeys {
		opts = append(opts, WithKeyedLine(k.key+"=", k.line))
	}
	opts = append(opts,
		WithKeyedLine("cpu=", "cpu="+g.CPU),
		WithKeyedLine("memory=", "memory="+g.Memory))
	if g.GraphicsWait != "" {
		opts = append(opts, WithKeyedLine("graphics_wait=", `graphics_wait="`+g.GraphicsWait+`"`))
	}
	return opts
}

// bhyveGuestConf returns the path of guest name's vm-bhyve config.
func bhyveGuestConf(name string) string {
	return bhyveVMDir + "/" + name + "/" + name + ".conf"
}

// bhyveVNCScript returns a sh loop over the vm-bhyve guest configs under
// bhyveVMDir that visits each <vm>/<vm>.conf with graphics enabled whose
// graphics_listen is not bhyveVNCListen. With fix set it rewrites the line
// (drop any old one, append the wanted one); without it the loop exits 0 on
// the first such config and 1 if there is none, which makes it the OnlyIf
// guard of the fix. A failed rewrite aborts the fix loop with status 1.
// Guests without graphics (bhyveload guests such as f3's freebsd VM) are
// left alone; a guest config that disappears (a deleted VM) is simply no
// longer visited, so nothing is ever recreated.
func bhyveVNCScript(fix bool) string {
	want := `graphics_listen="` + bhyveVNCListen + `"`
	action := `exit 0`
	tail := `; exit 1`
	if fix {
		action = `{ sed -i '' '/^graphics_listen=/d' "$c" && echo '` + want + `' >> "$c"; } || exit 1`
		tail = ``
	}
	return `for d in ` + bhyveVMDir + `/*/; do n=$(basename "$d"); c="$d$n.conf"; ` +
		`[ -f "$c" ] || continue; ` +
		`grep -Eq '^graphics="?yes"?$' "$c" || continue; ` +
		`grep -qx '` + want + `' "$c" && continue; ` +
		action + `; done` + tail
}
