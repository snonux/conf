// Package debian declares Debian 13 (trixie) host tasks for the conf
// repository: the Raspberry Pis pi2/pi3 after their planned move off Rocky
// Linux (f3s/docs/pi2-pi3-os-replacement.md, stage 0 of §7; task l82).
// unattended.go carries the partner-gated unattended upgrades, base.go the
// host base the Rocky Pis get from firewalld/dnf today (nftables, Docker CE,
// uptimed, SD-card write limits).
//
// Every task body runs inside onDebian: a destination-side guard on the
// cluster hostnames AND on /etc/debian_version, so pushing these tasks to a
// Pi that still runs Rocky (pi2/pi3 until tasks n82/o82 migrate them)
// records its ops but applies none of them.
package debian

import (
	"strings"

	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"
)

// debianMarker exists on Debian (and derivatives) only; Rocky has
// /etc/redhat-release instead. It is the destination-side OS guard.
const debianMarker = "/etc/debian_version"

// aptInstalledScript exits 0 when every package named in its arguments is
// installed (dpkg status "installed"); it is the idempotence guard of
// aptPackages.
const aptInstalledScript = `for p; do
  [ "$(dpkg-query -W -f='${db:Status-Status}' "$p" 2>/dev/null)" = installed ] || exit 1
done`

// aptInstallScript refreshes the package lists and installs its arguments.
// The refresh runs only when something is missing (the Unless guard), so a
// source added in the same run (Docker CE) is known before its packages
// are installed, and a failed earlier install retries cleanly. confdef /
// confold answer any conffile prompt (keep a locally changed file, take
// the default otherwise), so a prompt cannot fail the install.
const aptInstallScript = `apt-get -q update
apt-get install -y -q --no-install-recommends \
  -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold "$@"`

// onDebian runs fn on the current cluster's hosts, but only where the
// destination is Debian (see the package comment).
func onDebian(fn func()) {
	WhenHostname(ClusterHosts(), func() { WhenPathExists(debianMarker, fn) })
}

// aptPackages installs Debian packages; opts add options such as DependsOn
// (an apt source that must exist first). gonf v0.15.0's Package resource
// has no apt backend (dnf, pkg, pkgin and pkg_add only), so this is a
// guarded Command until one exists in the gonf library.
func aptPackages(names []string, opts ...CommandOption) Resource {
	all := append([]CommandOption{
		WithEnv(map[string]string{"DEBIAN_FRONTEND": "noninteractive"}),
		Unless("sh", append(List("-c", aptInstalledScript, "sh"), names...)),
		WithName("apt-install-" + strings.Join(names, "-")),
	}, opts...)
	return Command("sh", append(List("-ceu", aptInstallScript, "sh"), names...), all...)
}

// rootDir ensures a root-owned 0755 directory.
func rootDir(path string) Resource {
	return EnsureDir(path, WithMode(0o755), WithOwner("root"), WithGroup("root"))
}
