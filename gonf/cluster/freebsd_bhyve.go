package cluster

import "github.com/snonux/conf/gonf/freebsd"

// Per-host vm-bhyve guests of the f-hosts (task ek2), attached in
// registerFreeBSD. The values are the live ones of 2026-09-25: each host
// autostarts its "rocky" guest (the k3s nodes r0-r2 on f0-f2, the plain
// Rocky VM on f3). The blog's memory=14G for r0-r2 is outdated; they run with
// 12G. f3's guest keeps the 14G and graphics_wait="no" it was installed
// with. f0's stopped fedora VM and f3's stopped freebsd test VM are left
// unmanaged (and not autostarted); see freebsd.Bhyve.
var (
	k3sBhyve = freebsd.BhyveHost{Guests: []freebsd.BhyveGuest{
		{Name: "rocky", CPU: "4", Memory: "12G", Autostart: true},
	}}
	f3Bhyve = freebsd.BhyveHost{Guests: []freebsd.BhyveGuest{
		{Name: "rocky", CPU: "4", Memory: "14G", GraphicsWait: "no", Autostart: true},
	}}
)
