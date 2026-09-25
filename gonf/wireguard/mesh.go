// Package wireguard lets gonf trigger ~/git/wireguardmeshgenerator, the sole
// owner of the WireGuard mesh configuration (wg0.conf/tun0.conf, keys, PSKs,
// peers, the NetBSD rc.d script with its routes). gonf never renders any of
// that itself; see freebsd.WireGuard, rnodes.WireGuard and
// frontends.WireGuard for the host-side bits gonf does own, and
// StripGroupOther (perms.go) for the permission sweep they share.
package wireguard

import (
	"path/filepath"

	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// generatorScript is the generator's entry point inside its checkout.
const generatorScript = "wireguardmeshgenerator.rb"

// Mesh runs the generator on the controller. Register it WITHOUT OnCluster
// and run it by name on the laptop:
//
//	./gonf.sh -n wireguard_mesh_install   # preview only
//	./gonf.sh wireguard_mesh_install
//
// The generator itself pushes to every host of its YAML over SSH (scp +
// doas/sudo) and reloads each tunnel, so the task needs neither root nor a
// cluster. It is not Privileged: it runs as the invoking user, whose SSH
// keys the generator uses.
type Mesh struct{}

// DescInstall returns the description for the generator invocation.
func (Mesh) DescInstall() string {
	return "Run wireguardmeshgenerator --generate --install for the whole mesh (controller-local, explicit only)"
}

// OptsInstall marks the invocation Operational, like frontends_acme_invoke:
// no pattern aggregate ever picks it up, so a routine deploy never rewrites
// or reloads a tunnel. Without Privileged() it runs as the invoking user.
func (Mesh) OptsInstall() TaskOptions { return TaskOptions{Operational()} }

// Install regenerates every host's config under the checkout's dist/ (keys
// and PSKs under keys/ are only created when missing) and installs it on
// every host with an ssh section, full mesh at once: the generator's
// peer exclusions must stay symmetric, so installing a subset could leave a
// peer that the other side no longer knows. For a single host run the
// generator by hand: ruby wireguardmeshgenerator.rb --generate --install
// --hosts=f3.
//
// The WhenPathExists guard is evaluated where the plan applies: it holds on
// the controller with the checkout and skips the command on any pushed
// destination, so the task cannot run on a mesh host by accident.
func (Mesh) Install() {
	script := filepath.Join(paths.WireGuardMeshGenerator, generatorScript)
	WhenPathExists(script, func() {
		Command("ruby", List(generatorScript, "--generate", "--install"),
			WithDir(paths.WireGuardMeshGenerator))
	})
}
