package triggerfish

import (
	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

// OptsNetwork keeps legacy network changes out of routine host maintenance.
func (Base) OptsNetwork() TaskOptions { return TaskOptions{Operational()} }

// Network configures the temporary LAN alias, IPv4 forwarding and upstream NAT.
func (Base) Network() {
	// Migration recovery owns the current-cabling profile after its first arm.
	// A destination-side prerequisite prevents an explicit legacy invocation
	// from replacing that profile; ordinary aggregates exclude this action.
	legacy := Command("test", List("!", "-e", "/etc/gonf/router-owner"), WithName("legacy-router-owner-check"))
	// TEMPORARY migration setup until all workloads have moved over or been
	// renumbered. Keep the old 192.168.1.0/24 upstream reachable while
	// introducing 192.168.11.0/24 on the same physical interface.
	// Later move the alias and connected /24 route from igc0 to igc1;
	// do not assign the same connected subnet to both interfaces.
	iface := File("/etc/hostname.igc0", WithContent("inet 192.168.1.2 255.255.255.0\ninet alias 192.168.11.2 255.255.255.0\ninet6 autoconf\n"), RootPrivate, DependsOn(legacy))
	Sh("/sbin/ifconfig igc0 inet alias 192.168.11.2 netmask 255.255.255.0",
		DependsOn(iface), Unless("sh", List("-c", "/sbin/ifconfig igc0 | grep -q 'inet 192[.]168[.]11[.]2 netmask 0xffffff00'")))
	// Prepare the second port without assigning the downstream address yet.
	second := File("/etc/hostname.igc1", WithContent("up\n"), RootPrivate, DependsOn(legacy))
	Sh("/sbin/ifconfig igc1 up", OnChange(second), DependsOn(legacy))
	File("/etc/mygate", WithContent("192.168.1.1\n"), RootOwned, DependsOn(legacy))
	forwarding := File("/etc/sysctl.conf", WithKeyedLine("net.inet.ip.forwarding=", "net.inet.ip.forwarding=1"), RootOwned, DependsOn(legacy))
	pf := InstallFile("/etc/pf.conf", paths.GonfAsset("triggerfish", "pf.conf"), RootPrivate,
		WithValidation("/sbin/pfctl", List("-n", "-f", CandidatePath)), DependsOn(legacy))
	reload := Sh("/sbin/pfctl -f /etc/pf.conf", OnChange(pf), DependsOn(legacy))
	// Enable forwarding only after the validated firewall is loaded.
	Sh("/sbin/sysctl net.inet.ip.forwarding=1", DependsOn(forwarding, reload),
		Unless("sh", List("-c", `test "$(/sbin/sysctl -n net.inet.ip.forwarding)" = 1`)))
}
