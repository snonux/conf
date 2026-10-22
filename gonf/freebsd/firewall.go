package freebsd

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// Firewall manages inbound host filtering on every f-host, independently of
// relayd. The common rules trust LAN IPv4/ULA/link-local and authenticated
// WireGuard traffic; public IPv6 services are denied. See
// f3s/freebsd-hosts/pf/README.md for the approval, rollback and rollout checks.
//
// Bridge forwarding must bypass PF: re0 and tap interfaces carry the bhyve
// guests, whose firewall is managed separately. Disabling bridge/member hooks
// does not disable filtering of packets delivered to the host's IP stack.
// Never use "service pf start/restart": its start flushes states. Load the
// candidate before enabling a previously disabled PF; reloads preserve states.
type Firewall struct {
	RequiresRoot
}

const (
	pfConf      = "/etc/pf.conf"
	pfLoadedSum = "/var/db/gonf-pf.sha256"
)

// Config installs and activates validated default-deny inbound PF on f0-f3.
func (Firewall) Config() {
	bridgeKeys := List("net.link.bridge.pfil_member", "0", "net.link.bridge.pfil_bridge", "0")
	// Loader tunables protect guests from the moment if_bridge loads; sysctl
	// persists/reconciles the same settings on running hosts and later at boot.
	boot := File(loaderConf, WithShellVars(bridgeKeys...), RootOwned,
		WithName("loader-conf-firewall"))
	bridge := Sysctls(sysctlConf, bridgeKeys, RootOwned,
		WithName("sysctl-conf-firewall"), DependsOn(boot))
	// FreeBSD 15 pfctl -nf needs PF's netlink family even for a parse-only
	// check. Loading the module does not enable PF or load a ruleset.
	module := Sh("kldload pf", Unless("kldstat", List("-q", "-m", "pf")), DependsOn(bridge))
	conf := InstallFile(pfConf, paths.FHostAsset("pf/pf.conf"), RootOwned,
		WithValidation("pfctl", List("-nf", CandidatePath)), DependsOn(module))
	// Record the checksum only after a successful load. OnChange alone loses
	// a failed load when the file has already been published: the next apply
	// would otherwise skip it and leave an enabled PF on its old rules.
	reload := Command("sh", List("-ec", `digest=$(sha256 -q `+pfConf+`)
pfctl -f `+pfConf+`
printf '%s\n' "$digest" > `+pfLoadedSum), WithName("pf-load"), DependsOn(conf),
		Unless("sh", List("-ec", `test "$(sha256 -q `+pfConf+`)" = "$(cat `+pfLoadedSum+` 2>/dev/null)"`)))
	// A disabled PF may have an unrelated or empty ruleset even when the
	// managed file is unchanged. Load it again before enabling, without -F.
	enable := Command("sh", List("-ec", "pfctl -f "+pfConf+" && pfctl -e"),
		Unless("pfctl", List("-s", "Running")), DependsOn(reload))
	rc := File(rcConf, WithShellVars("pf_enable", "YES", "pf_rules", pfConf),
		RootOwned, WithName("rc-conf-firewall"), DependsOn(enable))
	// The service backend owns pflog_enable; do not also write that key.
	Service("pflog", DependsOn(rc))
}
