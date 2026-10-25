package frontends

import . "github.com/snonux/gonf/api"

// OptsWgZNC keeps the private ZNC migration a deliberate by-name operation.
func (Web) OptsWgZNC() TaskOptions { return TaskOptions{Operational()} }

// WhenWgZNC limits the bouncer migration to its existing host.
func (Web) WhenWgZNC() TaskOption { return WhenHostnameIn(Master) }

// WgZNC binds the existing TLS IRC bouncer to WireGuard without exporting secrets.
func (Web) WgZNC() {
	peer, err := wireGuardAddressFor(Master)
	if err != nil {
		Refuse("Command", "znc-wireguard", err)
		return
	}
	EnsureDir("/usr/local/libexec", RootOwned)
	EnsureDir("/usr/local/sbin", RootOwned)
	parser := InstallFile("/usr/local/libexec/znc-wg-listener.awk",
		legacyFrontendAsset("scripts/znc-wg-listener.awk"), RootOwned)
	script := InstallFile("/usr/local/sbin/znc-wireguard.sh",
		legacyFrontendAsset("scripts/znc-wireguard.sh"), RootExec)
	Command("/usr/local/sbin/znc-wireguard.sh", List("apply", peer.IPv4),
		DependsOn(parser, script),
		Unless("/usr/local/sbin/znc-wireguard.sh", List("check", peer.IPv4)))
}
