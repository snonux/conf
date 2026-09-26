package rnodes

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/wireguard"
)

// WireGuard owns the WireGuard bits of r0-r2 that
// ~/git/wireguardmeshgenerator does NOT own (user decision 2026-09-25, tasks
// qk2/zk2). The generator stays the single owner of the mesh: its --install
// copies wg0.conf into /etc/wireguard (mkdir, chmod 700 on the directory,
// chown root:root + chmod 600 + restorecon on wg0.conf) and runs
// "systemctl reload wg-quick@wg0.service". It neither installs
// wireguard-tools nor enables the unit. gonf never writes wg0.conf nor any
// key and never reloads or restarts the tunnel; wireguard_mesh_install
// (gonf/wireguard) is the opt-in, by-name way to run the generator.
//
// What is left for gonf:
//   - Perms: the directory mode, wg0.conf's mode, and no group/other bits on
//     any other non-.pub file there (the wg0.conf.bak.20260621-* copies,
//     0600 on 2026-09-25).
//   - Service: wireguard-tools and wg-quick@wg0 enabled and active.
//
// Mode changes keep the SELinux label (chmod does not relabel), so nothing
// here needs restorecon. The stale backup files are listed in task zk2 and
// kept: the user decides about their removal. SELinux wireguard_t is
// Base.SelinuxWireguard's.
//
// Register with OnCluster(cluster.NameRockyK3s); nothing here is per host.
type WireGuard struct {
	RequiresRoot
}

const (
	wireguardDir  = "/etc/wireguard"
	wireguardConf = wireguardDir + "/wg0.conf"
)

// DescPerms returns the description for the WireGuard permission task.
func (WireGuard) DescPerms() string {
	return "WireGuard: " + wireguardDir + " 0700, wg0.conf 0600 root:root, no group/other bits on other non-.pub files (content stays wireguardmeshgenerator's)"
}

// Perms converges only modes. Every resource waits for the path to exist,
// so gonf never creates the directory or an empty wg0.conf ahead of the
// generator, and EnsureFile keeps the file's bytes. The dated backup names
// differ per host, so wireguard.StripGroupOther sweeps group/other bits off
// every regular file but the public keys instead of naming them; it only
// runs while such a file exists. The guards are sequential rather than
// nested: each When fragment is its own recipe scope.
func (WireGuard) Perms() {
	WhenPathExists(wireguardDir, func() {
		Dir(wireguardDir, RootPrivate)
	})
	WhenPathExists(wireguardConf, func() {
		EnsureFile(wireguardConf, RootPrivate)
	})
	WhenPathExists(wireguardDir, func() {
		wireguard.StripGroupOther(wireguardDir)
	})
}

// DescService returns the description for the WireGuard service task.
func (WireGuard) DescService() string {
	return "WireGuard: wireguard-tools, wg-quick@wg0 enabled and active (never restarted)"
}

// Service installs wireguard-tools and, once the generator has installed
// wg0.conf, keeps wg-quick@wg0 enabled and active. There is no WithRestart
// and no OnChange: a running tunnel is never bounced by gonf (a reload is
// the generator's job). Without wg0.conf the unit would fail to start, so
// it waits for it. The package is declared outside the When fragment (a
// fragment is its own recipe scope); plan order installs it first.
func (WireGuard) Service() {
	Package("wireguard-tools")
	WhenPathExists(wireguardConf, func() {
		Service("wg-quick@wg0")
	})
}
