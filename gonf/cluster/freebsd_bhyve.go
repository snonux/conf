package cluster

import "github.com/snonux/conf/gonf/freebsd"

// Per-host vm-bhyve guests of the f-hosts (task ek2), attached in
// registerFreeBSD. The values are the live ones of 2026-09-25, except
// autostart: f3 autostarts its plain Rocky VM, but since f3sctl v0.12.0
// (task gl4) the k3s hosts f0-f2 do not autostart theirs (r0-r2). f3sctl
// starts them with its agent's vm-start verb on every wake, except
// `f3sctl power hosts on`, which brings the hosts up without Kubernetes;
// `f3sctl power vms on` starts them later. A k3s host rebooted by hand
// therefore comes back with its guest off. The blog's memory=14G for r0-r2 is outdated; they run with
// 12G. f3's guest was installed with 14G and graphics_wait="no"; it keeps
// graphics_wait, but its memory was later reduced by hand to 12G, and the
// inventory follows that live value (task k74, 2026-11-13) instead of
// growing the guest back on its next vm stop/start. f0's stopped fedora VM
// and f3's stopped freebsd test VM are left unmanaged (and not
// autostarted); see freebsd.Bhyve.
var (
	k3sBhyve = freebsd.BhyveHost{Guests: []freebsd.BhyveGuest{
		{Name: "rocky", CPU: "4", Memory: "12G", Autostart: false},
	}}
	f3Bhyve = freebsd.BhyveHost{Guests: []freebsd.BhyveGuest{
		{Name: "rocky", CPU: "4", Memory: "12G", GraphicsWait: "no", Autostart: true},
	}}
)
