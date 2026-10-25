// Package hostfirewall stages the source firewall shared by Rocky nodes and Pis.
package hostfirewall

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// Profile describes the common ingress path of one host group.
// EnableWireGuard is explicit because serialized template numbers retain
// their text representation; numeric zero is not a boolean false there.
type Profile struct {
	Interface       string
	EnableWireGuard bool
	WireGuardPort   uint16
}

// Stage installs candidates and the guarded activation helper. It neither loads
// rules nor enables the boot service: approval and one-host verification come
// first, as described in f3s/docs/linux-source-firewall.md.
func Stage(profile Profile) {
	EnsureDir("/etc/f3s-firewall", RootPrivate)
	EnsureDir("/usr/local/sbin", RootOwned)
	rules := InstallFile("/etc/f3s-firewall/candidate.nft",
		paths.GonfAsset("hostfirewall", "candidate.nft.tmpl"),
		RootPrivate, WithTemplateData(profile),
		WithValidation("/usr/sbin/nft", List("-c", "-f", CandidatePath)))
	helper := InstallFile("/usr/local/sbin/f3s-firewall",
		paths.GonfAsset("hostfirewall", "f3s-firewall"), RootExec,
		WithValidation("/usr/bin/bash", List("-n", CandidatePath)))
	unit := InstallFile("/etc/systemd/system/f3s-firewall.service",
		paths.GonfAsset("hostfirewall", "f3s-firewall.service"), RootOwned)
	Sh("restorecon -RF /etc/f3s-firewall /usr/local/sbin/f3s-firewall /etc/systemd/system/f3s-firewall.service",
		OnChange(rules, helper, unit))
	Sh("systemctl daemon-reload", OnChange(unit))
}
