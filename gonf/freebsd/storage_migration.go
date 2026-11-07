package freebsd

import (
	"os"
	"strings"

	"github.com/snonux/conf/gonf/paths"
	"github.com/snonux/conf/gonf/storagemigration"
	. "github.com/snonux/gonf/api"
)

// OptsMigrationCandidate keeps CARP preparation outside ordinary applies.
func (Carp) OptsMigrationCandidate() TaskOptions { return TaskOptions{Operational()} }

// WhenMigrationCandidate limits CARP preparation to the existing owners.
func (Carp) WhenMigrationCandidate() TaskOption { return WhenHostnameIn(carpMembers...) }

// MigrationCandidate stages a CARP alias template without reading its password.
// This is deliberately not an rc.conf fragment that can be activated as-is.
// Guarded promotion must resolve MustSecret(CarpPassSecret()) through RcConf's
// existing ownership, retaining vhid, skew, key, old management and alias0.
func (Carp) MigrationCandidate() {
	EachHostWith(func(n CarpNode) {
		line := carpAliasLine(n, "@RESOLVE_EXISTING_CARP_SECRET_AT_PROMOTION@")
		line = strings.ReplaceAll(line, carpVIP, storagemigration.LabVIP+"/32")
		compat := strings.Replace(carpAliasLine(n, "@RESOLVE_EXISTING_CARP_SECRET_AT_PROMOTION@"), "alias0=", "alias1=", 1)
		File(rcConf+".carp"+storagemigration.Suffix,
			WithContent("# INACTIVE TEMPLATE; not a shell configuration.\n# Both addresses MUST share existing vhid1 on BOTH members.\n# Secret reference: "+CarpPassSecret()+"\n"+line+"\n"+compat+"\n"), RootOwned)
	})
}

// OptsMigrationCandidate keeps server preparation outside ordinary applies.
func (NFS) OptsMigrationCandidate() TaskOptions { return TaskOptions{Operational()} }

// WhenMigrationCandidate limits server preparation to the CARP pair.
func (NFS) WhenMigrationCandidate() TaskOption { return WhenHostnameIn(carpMembers...) }

// MigrationCandidate stages the new server listener using existing TLS paths.
func (NFS) MigrationCandidate() {
	storageCandidate(stunnelConf, paths.FHostAsset("nfs/stunnel.conf"), storagemigration.CompatibleServer)
}

// OptsMigrationCandidate keeps ingress preparation outside ordinary applies.
func (Relayd) OptsMigrationCandidate() TaskOptions { return TaskOptions{Operational()} }

// WhenMigrationCandidate limits ingress preparation to the CARP pair.
func (Relayd) WhenMigrationCandidate() TaskOption { return WhenHostnameIn(carpMembers...) }

// MigrationCandidate stages VIP listeners and LAN backends without reloading.
func (Relayd) MigrationCandidate() {
	storageCandidate(relaydConf, paths.FHostAsset("relayd/relayd.conf"), storagemigration.CompatibleRelayd)
}

func storageCandidate(destination, source string, render func(string) (string, error)) {
	raw, err := os.ReadFile(source)
	if err != nil {
		Refuse("storage candidate", destination, err)
		return
	}
	content, err := render(string(raw))
	if err != nil {
		Refuse("storage candidate", destination, err)
		return
	}
	File(destination+storagemigration.Suffix, WithContent(content), RootOwned)
}
