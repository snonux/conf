package freebsd

import (
	. "github.com/snonux/gonf/api"
)

// StorageRetirement removes the storage-migration helper's host artifacts
// from the CARP pair f0/f1 (task 874, item 4 of IQ
// openbsdrouter-plan/CLEANUP.md): the every-minute root watchdog cron, the
// pre-netif restore hook gonf_storage_rollback, the late boot fence
// gonf_storage_hold, the helper, its transaction journals in
// /var/db/gonf-storage-migration and the inactive CARP, stunnel and relayd
// candidates that freebsd_{carp,nfs,relayd}_migration_candidate staged.
// The move to the storage VIP 10.20.0.138 was accepted on 2026-11-07; rc.conf,
// stunnel.conf and relayd.conf carry it (freebsd_carp_rc_conf,
// freebsd_nfs_stunnel_conf, freebsd_relayd_daemon). With no pending
// transaction and no "held" fence both rc.d hooks did nothing at boot.
//
// One-shot: once f0 and f1 ran freebsd_storage_migration_retire, a later
// commit deletes this file and its registration. It is Operational, so the
// freebsd aggregate never picks it up: run it by name, one host at a time.
type StorageRetirement struct{ RequiresRoot }

const (
	storageHelper = "/usr/local/sbin/gonf-storage-migration"
	storageState  = "/var/db/gonf-storage-migration"
	// storageCandidateSuffix is the suffix the candidates were staged under.
	storageCandidateSuffix = ".triggerfish-candidate"
)

func (StorageRetirement) OptsRetire() TaskOptions { return TaskOptions{Operational()} }

// WhenRetire limits the retirement to the CARP pair, where Prepare ran.
func (StorageRetirement) WhenRetire() TaskOption { return WhenHostnameIn(carpMembers...) }

// Retire removes the storage-migration watchdog, boot hooks, helper and candidates.
func (StorageRetirement) Retire() {
	// Refuse while a transaction is pending or CARP is fenced: the rollback
	// and release-old paths still need the helper. Skipped when clean.
	open := `for f in pending pf-pending held; do [ -e "$1/$f" ] && exit 0; done; exit 1`
	refuse := Command("sh", List("-c", "echo 'pending or held storage-migration transaction' >&2; exit 1"),
		OnlyIf("sh", List("-c", open, "open", storageState)), WithName("storage-migration-idle"))
	cron := NoCron("gonf-storage-rollback", DependsOn(refuse))
	// The hooks have no rcvar: they run at every boot while present, so
	// deleting the files is what disables them.
	hooks := NoFile(List("/etc/rc.d/gonf_storage_rollback", "/etc/rc.d/gonf_storage_hold"), DependsOn(refuse))
	NoFile(storageHelper, DependsOn(cron, hooks))
	NoDir(storageState, WithPrune, DependsOn(cron, hooks))
	NoFile(List(rcConf+".carp"+storageCandidateSuffix, stunnelConf+storageCandidateSuffix,
		relaydConf+storageCandidateSuffix), DependsOn(refuse))
}
