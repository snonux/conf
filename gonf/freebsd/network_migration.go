package freebsd

import (
	"fmt"
	"net/netip"

	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

const migrationHelper = "/usr/local/sbin/gonf-network-migration"

// NetworkMigration prepares explicit, reversible f-host address changes.
// Every method is operational: ordinary freebsd applies never stage addresses.
type NetworkMigration struct{ RequiresRoot }

// MigrationHost carries the physical interface and reserved host addresses.
// SharedGateway must be reachable on the unchanged production cable.
type MigrationHost struct {
	Interface, OldIPv4, LabIPv4, RecoveryIPv4, ULA, SharedGateway string
}

func (NetworkMigration) OptsPrepare() TaskOptions  { return TaskOptions{Operational()} }
func (NetworkMigration) OptsStage() TaskOptions    { return TaskOptions{Operational()} }
func (NetworkMigration) OptsPromote() TaskOptions  { return TaskOptions{Operational()} }
func (NetworkMigration) OptsConfirm() TaskOptions  { return TaskOptions{Operational()} }
func (NetworkMigration) OptsRollback() TaskOptions { return TaskOptions{Operational()} }
func (NetworkMigration) OptsStatus() TaskOptions   { return TaskOptions{Operational()} }

// Prepare installs dormant rollback tooling without changing network state.
func (NetworkMigration) Prepare() {
	EachHost(func(h MigrationHost) {
		// Reserve alias200..202; Carp owns alias0, Bhyve owns vm_*.
		for _, value := range []string{h.OldIPv4, h.LabIPv4, h.RecoveryIPv4, h.SharedGateway} {
			if !netip.MustParseAddr(value).Is4() {
				panic("migration requires IPv4")
			}
		}
		if addr := netip.MustParseAddr(h.ULA); !addr.Is6() || !addr.IsPrivate() || h.Interface != "re0" {
			panic("migration requires re0 and IPv6 ULA")
		}
		check := Command("test", List("!", "-d", "/var/db/gonf-network-migration/pending"))
		EnsureDir("/usr/local/sbin", RootOwned)
		EnsureDir("/usr/local/etc", RootOwned)
		EnsureDir("/var/db/gonf-network-migration", RootPrivate)
		conf := File("/usr/local/etc/gonf-network-migration.conf", WithContent(fmt.Sprintf(
			"iface='%s'\nold='%s'\nlab='%s'\nrecovery='%s'\nula='%s'\nshared_gateway='%s'\n",
			h.Interface, h.OldIPv4, h.LabIPv4, h.RecoveryIPv4, h.ULA, h.SharedGateway)), RootPrivate, DependsOn(check))
		helper := InstallFile(migrationHelper, paths.FHostAsset("network-migration/migrate.sh"), RootExec, DependsOn(conf))
		// System rc.d is deliberate: local_startup scripts may be discovered
		// only after netif. Restore before netif consumes an unconfirmed rc.conf.
		InstallFile("/etc/rc.d/gonf_network_rollback", paths.FHostAsset("network-migration/rc-rollback"), RootExec, DependsOn(helper))
		CronAt("gonf-network-rollback", "* * * * *", migrationHelper+" watchdog", DependsOn(helper))
	})
}

// Stage adds the lab address and local ULA on the unchanged shared wire.
func (NetworkMigration) Stage() { Sh(migrationHelper + " stage shared 900") }

// Promote selects the lab primary and gateway while retaining old management.
func (NetworkMigration) Promote() { Sh(migrationHelper + " promote shared 900") }

// Confirm accepts a transaction after separate fresh management/service checks.
func (NetworkMigration) Confirm() { Sh(migrationHelper + " confirm") }

// Rollback restores the pending transaction without restarting netif or guests.
func (NetworkMigration) Rollback() { Sh(migrationHelper + " rollback") }

// Status reports the persistent rollback deadline and current transaction.
func (NetworkMigration) Status() { Sh(migrationHelper + " status") }
