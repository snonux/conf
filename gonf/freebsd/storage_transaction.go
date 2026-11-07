package freebsd

import (
	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

const storageHelper = "/usr/local/sbin/gonf-storage-migration"

// StorageMigration prepares the CARP pair's guarded storage transaction.
// Coordinator gates attest pair-wide facts that cannot be inferred locally.
// Normal aggregates never invoke these operations.
type StorageMigration struct{ RequiresRoot }

func (StorageMigration) OptsPrepare() TaskOptions {
	return TaskOptions{Operational()}
}
func (StorageMigration) OptsArm() TaskOptions        { return TaskOptions{Operational()} }
func (StorageMigration) OptsHold() TaskOptions       { return TaskOptions{Operational()} }
func (StorageMigration) OptsStage() TaskOptions      { return TaskOptions{Operational()} }
func (StorageMigration) OptsEnable() TaskOptions     { return TaskOptions{Operational()} }
func (StorageMigration) OptsConfirm() TaskOptions    { return TaskOptions{Operational()} }
func (StorageMigration) OptsRollback() TaskOptions   { return TaskOptions{Operational()} }
func (StorageMigration) OptsReleaseOld() TaskOptions { return TaskOptions{Operational()} }
func (StorageMigration) OptsReconcile() TaskOptions  { return TaskOptions{Operational()} }
func (StorageMigration) OptsStatus() TaskOptions     { return TaskOptions{Operational()} }

func (StorageMigration) WhenPrepare() TaskOption    { return WhenHostnameIn(carpMembers...) }
func (StorageMigration) WhenArm() TaskOption        { return WhenHostnameIn(carpMembers...) }
func (StorageMigration) WhenHold() TaskOption       { return WhenHostnameIn(carpMembers...) }
func (StorageMigration) WhenStage() TaskOption      { return WhenHostnameIn(carpMembers...) }
func (StorageMigration) WhenEnable() TaskOption     { return WhenHostnameIn(carpMembers...) }
func (StorageMigration) WhenConfirm() TaskOption    { return WhenHostnameIn(carpMembers...) }
func (StorageMigration) WhenRollback() TaskOption   { return WhenHostnameIn(carpMembers...) }
func (StorageMigration) WhenReleaseOld() TaskOption { return WhenHostnameIn(carpMembers...) }
func (StorageMigration) WhenReconcile() TaskOption  { return WhenHostnameIn(carpMembers...) }
func (StorageMigration) WhenStatus() TaskOption     { return WhenHostnameIn(carpMembers...) }

// Prepare installs dormant native deadline and pre-netif boot recovery.
func (StorageMigration) Prepare() {
	Carp{}.MigrationCandidate()
	NFS{}.MigrationCandidate()
	Relayd{}.MigrationCandidate()
	check := Command("test", List("!", "-e", "/var/db/gonf-storage-migration/pending"))
	pfCheck := Command("test", List("!", "-e", "/var/db/gonf-storage-migration/pf-pending"))
	python := Command("test", List("-x", "/usr/local/bin/python3.12"))
	EnsureDir("/var/db/gonf-storage-migration", RootPrivate, DependsOn(check))
	helper := InstallFile(storageHelper, paths.FHostAsset("storage-migration/migrate.py"), RootExec, DependsOn(check, pfCheck, python))
	InstallFile("/etc/rc.d/gonf_storage_rollback", paths.FHostAsset("storage-migration/rc-rollback"), RootExec, DependsOn(helper))
	InstallFile("/etc/rc.d/gonf_storage_hold", paths.FHostAsset("storage-migration/rc-hold"), RootExec, DependsOn(helper))
	CronAt("gonf-storage-rollback", "* * * * *", storageHelper+" watchdog", DependsOn(helper))
}

// Arm saves the existing owned configuration with a fifteen-minute deadline.
func (StorageMigration) Arm() { Sh(storageHelper + " arm 900") }

// Hold fences CARP and stops storage hooks and listeners without stopping cron.
func (StorageMigration) Hold() { Sh(storageHelper + " hold") }

// Stage installs dual endpoints only after both members are attested fenced.
func (StorageMigration) Stage() { Sh(storageHelper + " stage") }

// Enable advertises the shared address set after a fresh matching-pair audit.
func (StorageMigration) Enable() { Sh(storageHelper + " enable") }

// Confirm archives the transaction after native acceptance on both members.
func (StorageMigration) Confirm() { Sh(storageHelper + " confirm") }

// Rollback restores old owned configuration while retaining the CARP fence.
func (StorageMigration) Rollback() { Sh(storageHelper + " rollback") }

// ReleaseOld enables restored CARP only after both old address sets match.
func (StorageMigration) ReleaseOld() { Sh(storageHelper + " release-old") }

// Reconcile applies the existing CARP hook after the election settles.
func (StorageMigration) Reconcile() { Sh(storageHelper + " reconcile") }

// Status reports pending and fenced state without reading secret values.
func (StorageMigration) Status() { Sh(storageHelper + " status") }
