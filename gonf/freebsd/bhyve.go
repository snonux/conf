package freebsd

import (
	. "github.com/snonux/gonf/api"
)

// Bhyve hardens the vm-bhyve guests on the f-hosts.
//
// vm-bhyve binds a guest's UEFI framebuffer (VNC, no password) to
// graphics_listen, default 0.0.0.0, so until 2026-09-25 every rocky/r-VM
// console was open on port 5900 of every interface, wg0 included (audit item
// 15, f3s/docs/audit-2026-09-25.md). VNCListen pins it to loopback; reach a
// console through an SSH tunnel instead:
//
//	ssh -p22 -L 5900:127.0.0.1:5900 f0.lan.buetow.org   # then vnc://localhost:5900
//
// vm-bhyve reads <vm>.conf once when "vm start" launches the guest (vm-run
// loads it before its run loop), so the change takes effect only on the next
// full stop/start of a guest -- not on a guest-internal reboot, and running
// k3s VMs are deliberately not restarted for it.
//
// Register with OnCluster(cluster.NameFreeBSD). No host data: every f-host
// gets the same loopback binding.
type Bhyve struct {
	RequiresRoot
}

// bhyveVNCListen is the address vm-bhyve's VNC framebuffer binds.
const bhyveVNCListen = "127.0.0.1"

// bhyveVNCScript returns a sh loop over the vm-bhyve guest configs under
// /zroot/bhyve (vm_dir="zfs:zroot/bhyve" on every f-host) that visits each
// <vm>/<vm>.conf with graphics enabled whose graphics_listen is not
// bhyveVNCListen. With fix set it rewrites the line (drop any old one,
// append the wanted one); without it the loop exits 0 on the first such
// config and 1 if there is none, which makes it the OnlyIf guard of the fix.
// A failed rewrite aborts the fix loop with status 1.
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
	return `for d in /zroot/bhyve/*/; do n=$(basename "$d"); c="$d$n.conf"; ` +
		`[ -f "$c" ] || continue; ` +
		`grep -Eq '^graphics="?yes"?$' "$c" || continue; ` +
		`grep -qx '` + want + `' "$c" && continue; ` +
		action + `; done` + tail
}

// DescVNCListen returns the description for the VNC binding.
func (Bhyve) DescVNCListen() string {
	return "vm-bhyve guests: graphics_listen=" + bhyveVNCListen + " (VNC on loopback; applies on the next vm stop/start)"
}

// VNCListen sets graphics_listen in every graphical guest's config. It does
// not restart any guest; see the type comment.
func (Bhyve) VNCListen() {
	Command("sh", List("-c", bhyveVNCScript(true)),
		OnlyIf("sh", List("-c", bhyveVNCScript(false))))
}
