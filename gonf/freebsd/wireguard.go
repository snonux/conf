package freebsd

import (
	. "github.com/snonux/gonf/api"
)

// WireGuard owns the WireGuard scaffolding on the f-hosts that
// ~/git/wireguardmeshgenerator does NOT own (user decision 2026-09-25,
// task qk2). The generator stays the single owner of the mesh itself: its
// --generate writes wg0.conf (keys, PSKs, peers, addresses) under dist/, and
// its --install copies wg0.conf into /usr/local/etc/wireguard (mkdir, chmod
// 700 on the directory, chown root:wheel + chmod 600 on wg0.conf) and runs
// "service wireguard reload". gonf never writes wg0.conf nor any key, and never
// reloads or restarts the tunnel; wireguard.Mesh (gonf/wireguard) is the
// opt-in, by-name way to run the generator through gonf.
//
// What is left for gonf:
//   - Perms: the directory mode and the modes of the secret-bearing files
//     that exist there, including stray ones the generator never touches
//     (f3's hand-made wg0.key and wg0.conf.bak were 0644 until 2026-09-25).
//   - Service: wireguard-tools, wireguard_interfaces="wg0" and the rc
//     service being enabled and running (the generator assumes all three).
//
// Stale wg0.conf.bak* files are listed in task qk2 and deliberately kept: the
// user decides about their removal. The /etc/hosts wg0 rows belong to
// Base (rendered from gonf/etchosts).
//
// Register with OnCluster(cluster.NameFreeBSD); nothing here is per host.
type WireGuard struct {
	RequiresRoot
}

const (
	wireguardDir  = "/usr/local/etc/wireguard"
	wireguardConf = wireguardDir + "/wg0.conf"
)

// wireguardSecretFiles are the files under wireguardDir that hold a private
// key or PSK and must be 0600 root:wheel when present. wg0.conf is the
// generator's (same mode it installs, so this only repairs drift); wg0.key
// and wg0.conf.bak are f3's hand-made leftovers. The dated
// wg0.conf.bak.YYYYMMDD-* copies on f0-f2 cannot be named statically; they
// are already 0600 and sit in the 0700 directory. wg0.pub is public and may
// stay 0644.
var wireguardSecretFiles = []string{
	wireguardConf,
	wireguardDir + "/wg0.key",
	wireguardDir + "/wg0.conf.bak",
}

// DescPerms returns the description for the WireGuard permission task.
func (WireGuard) DescPerms() string {
	return "WireGuard: " + wireguardDir + " 0700 and wg0.conf/wg0.key/wg0.conf.bak 0600 root:wheel when present (content stays wireguardmeshgenerator's)"
}

// Perms converges only modes and ownership. Every resource is guarded by
// WhenPathExists, so gonf never creates the directory or an empty wg0.conf
// ahead of the generator, and EnsureFile keeps each file's bytes. The guards
// are sequential rather than nested: each When fragment is its own recipe
// scope.
func (WireGuard) Perms() {
	WhenPathExists(wireguardDir, func() {
		Dir(wireguardDir, Perm(0o700, Root))
	})
	for _, path := range wireguardSecretFiles {
		WhenPathExists(path, func() {
			EnsureFile(path, Perm(0o600, Root))
		})
	}
}

// DescService returns the description for the WireGuard service task.
func (WireGuard) DescService() string {
	return "WireGuard: wireguard-tools, rc.conf wireguard_interfaces=wg0, wireguard enabled and running (never restarted)"
}

// Service installs wireguard-tools and, once the generator has installed
// wg0.conf, keeps rc.conf's wireguard_interfaces line and the wireguard rc
// service enabled (wireguard_enable="YES", written by the Service backend
// via sysrc) and running. There is no WithRestart and no OnChange: a
// running tunnel is never bounced by gonf (a reload/restart is the
// generator's job, see the f3s skill's wireguard.md). Without wg0.conf the
// service would fail to start, so the rc bits wait for it. The package is
// declared first, outside the When fragment (a fragment is its own recipe
// scope, so no DependsOn across it); plan order installs it before the
// service is probed.
func (WireGuard) Service() {
	Package("wireguard-tools")
	WhenPathExists(wireguardConf, func() {
		rc := File(rcConf, rcConfKeyedLine("wireguard_interfaces", "wg0"),
			Perm(0o644, Root), WithName("rc-conf-wireguard"))
		Service("wireguard", DependsOn(rc))
	})
}
