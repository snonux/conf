package frontends

import (
	"fmt"

	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/wireguard"
)

// WireGuard owns the WireGuard bits of the OpenBSD frontends that
// ~/git/wireguardmeshgenerator does NOT own (user decision 2026-09-25, tasks
// qk2/zk2). The generator stays the single owner of the mesh: its --install
// copies wg0.conf into /etc/wireguard (mkdir, chmod 700 on the directory,
// chown root:wheel + chmod 600 on wg0.conf) and reloads with
// "sh /etc/netstart wg0". It never writes /etc/hostname.wg0, which its
// reload relies on. gonf never writes wg0.conf nor any key and never runs
// netstart; wireguard_mesh_install (gonf/wireguard) is the opt-in, by-name
// way to run the generator.
//
// What is left for gonf:
//   - Perms: the directory mode, wg0.conf's mode, and no group/other bits on
//     any non-.pub file in the directory. Both frontends kept hand-made or
//     backup copies there (wg0.conf.bak, wg0.conf.bak.20260621-*, the old
//     <fqdn>.priv) and several of them were 0644 until 2026-09-25.
//   - Service: wireguard-tools and /etc/hostname.wg0, the OpenBSD
//     enablement of the interface (addresses from WireGuardAddresses).
//
// The stale backup files themselves are listed in task zk2 and kept: the
// user decides about their removal.
//
// Register with OnCluster(cluster.NameFrontends).
type WireGuard struct {
	RequiresRoot
}

const (
	wireguardDir  = "/etc/wireguard"
	wireguardConf = wireguardDir + "/wg0.conf"
	hostnameWg0   = "/etc/hostname.wg0"
)

// DescPerms returns the description for the WireGuard permission task.
func (WireGuard) DescPerms() string {
	return "WireGuard: " + wireguardDir + " 0700, wg0.conf 0600 root:wheel, no group/other bits on other non-.pub files (content stays wireguardmeshgenerator's)"
}

// Perms converges only modes. Every resource waits for the path to exist,
// so gonf never creates the directory or an empty wg0.conf ahead of the
// generator, and EnsureFile keeps the file's bytes. The dated backup names
// differ per host and run, so wireguard.StripGroupOther sweeps
// group/other bits off every regular file but the public keys instead of
// naming them; it only runs while such a file exists. The guards are sequential rather than nested: each When fragment is its
// own recipe scope.
func (WireGuard) Perms() {
	WhenPathExists(wireguardDir, func() {
		Dir(wireguardDir, Perm(0o700, Root))
	})
	WhenPathExists(wireguardConf, func() {
		EnsureFile(wireguardConf, Perm(0o600, Root))
	})
	WhenPathExists(wireguardDir, func() {
		wireguard.StripGroupOther(wireguardDir)
	})
}

// DescService returns the description for the WireGuard service task.
func (WireGuard) DescService() string {
	return "WireGuard: wireguard-tools and /etc/hostname.wg0 (0640 root:wheel; never runs netstart)"
}

// Service installs wireguard-tools and keeps /etc/hostname.wg0, which
// brings wg0 up at boot and on the generator's "sh /etc/netstart wg0". The
// content is the live file of 2026-09-25 (identical on both hosts but for
// the addresses); the mode is the 0640 netstart(8) asks for. There is no
// OnChange: a changed file takes effect on the next generator reload or
// boot, gonf never bounces a running tunnel. The file waits for wg0.conf,
// which its "wg setconf" line loads. A frontend missing from the WireGuard
// inventory is refused as a declaration error (fails the record).
func (WireGuard) Service() {
	Package("wireguard-tools")
	EachHost(func(server Server) {
		content, err := hostnameWg0Content(server.Name)
		if err != nil {
			Refuse("File", hostnameWg0, err)
			return
		}
		WhenPathExists(wireguardConf, func() {
			File(hostnameWg0, WithContent(content), Perm(0o640, Root))
		})
	})
}

// hostnameWg0Content renders /etc/hostname.wg0 for the frontend name from
// its WireGuardAddresses row: both addresses, up, and the wg setconf line
// that loads the generator's wg0.conf.
func hostnameWg0Content(name string) (string, error) {
	peer, err := wireGuardAddressFor(name)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("inet %s 255.255.255.0 NONE\ninet6 %s 64\nup\n!/usr/local/bin/wg setconf wg0 %s\n",
		peer.IPv4, peer.IPv6, wireguardConf), nil
}
