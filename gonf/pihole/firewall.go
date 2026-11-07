package pihole

import (
	"github.com/snonux/conf/gonf/hostfirewall"
	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

// Firewall stages the Pi source guard alongside the existing firewalld policy.
type Firewall struct{ RequiresRoot }

// OptsStage excludes firewall staging from ordinary Pi-hole deployments.
func (Firewall) OptsStage() TaskOptions { return TaskOptions{Operational()} }

// Stage installs a candidate and guarded firewall activation tools without loading rules.
func (Firewall) Stage() {
	hostfirewall.Stage(hostfirewall.Profile{
		Interface: "eth0",
		Rules:     paths.GonfAsset("pihole", "network-migration/firewall.nft"),
	})
}

// OptsMigrationStage keeps resolver migration out of ordinary deployments.
func (Firewall) OptsMigrationStage() TaskOptions { return TaskOptions{Operational()} }

// MigrationStage installs dormant resolver source permissions without loading rules.
func (Firewall) MigrationStage() {
	EnsureDir("/etc/f3s-firewall", RootPrivate)
	InstallFile("/etc/f3s-firewall/migration.nft",
		paths.GonfAsset("pihole", "network-migration/firewall.nft"), RootPrivate,
		WithValidation("/usr/sbin/nft", List("-c", "-f", CandidatePath)))
}
