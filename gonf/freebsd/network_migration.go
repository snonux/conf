package freebsd

import (
	"fmt"
	"net/netip"

	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

const migrationHelper = "/usr/local/sbin/gonf-network-migration"

// NetworkMigration prepares explicit, reversible host address changes.
// Every method is operational: ordinary freebsd applies never stage addresses.
type NetworkMigration struct{ RequiresRoot }

// MigrationHost carries the physical interface and reserved host addresses.
// SharedGateway must be reachable on the unchanged production cable.
type MigrationHost struct {
	Interface, OldIPv4, LabIPv4, RecoveryIPv4, ULA, SharedGateway string
	// Standalone is a reviewed vtnet0 guest with no allocated native ULA.
	Standalone bool
	IPv6Policy string
}

func (NetworkMigration) OptsPrepare() TaskOptions  { return TaskOptions{Operational()} }
func (NetworkMigration) OptsStage() TaskOptions    { return TaskOptions{Operational()} }
func (NetworkMigration) OptsPromote() TaskOptions  { return TaskOptions{Operational()} }
func (NetworkMigration) OptsConfirm() TaskOptions  { return TaskOptions{Operational()} }
func (NetworkMigration) OptsRollback() TaskOptions { return TaskOptions{Operational()} }
func (NetworkMigration) OptsStatus() TaskOptions   { return TaskOptions{Operational()} }
func (NetworkMigration) OptsRetire() TaskOptions   { return TaskOptions{Operational()} }
func (NetworkMigration) OptsRetireRecovery() TaskOptions {
	return TaskOptions{Operational()}
}

// Prepare installs dormant rollback tooling without changing network state.
func (NetworkMigration) Prepare() {
	EachHost(func(h MigrationHost) {
		// Reserve alias200..202; Carp owns alias0, Bhyve owns vm_*.
		for _, value := range List(h.OldIPv4, h.LabIPv4, h.RecoveryIPv4, h.SharedGateway) {
			if !netip.MustParseAddr(value).Is4() {
				panic("migration requires IPv4")
			}
		}
		ipv6Policy := "ula"
		if h.Standalone {
			ipv6Policy = h.IPv6Policy
			if h.Interface != "vtnet0" || ipv6Policy != "disabled" || h.ULA != "" {
				panic("guest migration requires reviewed vtnet0 and disabled native IPv6 without ULA")
			}
		} else if addr := netip.MustParseAddr(h.ULA); !addr.Is6() || !addr.IsPrivate() || h.Interface != "re0" {
			panic("migration requires re0 and IPv6 ULA")
		}
		check := Command("test", List("!", "-d", "/var/db/gonf-network-migration/pending"))
		guard := Command("test", List("!", "-e", "/var/db/gonf-network-migration/confirmed-boot-guard"))
		EnsureDir("/usr/local/sbin", RootOwned)
		EnsureDir("/usr/local/etc", RootOwned)
		EnsureDir("/var/db/gonf-network-migration", RootPrivate)
		conf := File("/usr/local/etc/gonf-network-migration.conf", WithContent(fmt.Sprintf(
			"iface='%s'\nold='%s'\nlab='%s'\nrecovery='%s'\nula='%s'\nshared_gateway='%s'\nipv6_policy='%s'\n",
			h.Interface, h.OldIPv4, h.LabIPv4, h.RecoveryIPv4, h.ULA, h.SharedGateway, ipv6Policy)), RootPrivate, DependsOn(check, guard))
		helper := InstallFile(migrationHelper, paths.FHostAsset("network-migration/migrate.sh"), RootExec, DependsOn(conf))
		// System rc.d is deliberate: local_startup scripts may be discovered
		// only after netif. Restore before netif consumes an unconfirmed rc.conf.
		InstallFile("/etc/rc.d/gonf_network_rollback", paths.FHostAsset("network-migration/rc-rollback"), RootExec, DependsOn(helper))
		CronAt("gonf-network-rollback", "* * * * *", migrationHelper+" watchdog", DependsOn(helper))
	})
}

// Stage adds the lab address and applies the reviewed native IPv6 policy.
func (NetworkMigration) Stage() { Sh(migrationHelper + " stage shared 900") }

// Promote selects the lab primary and gateway while retaining old management.
func (NetworkMigration) Promote() { Sh(migrationHelper + " promote shared 900") }

// Retire removes the retained old IPv4 (alias200) under a 900-second journal.
// It needs the accepted isolated promote, the activation, igc1 and single-use
// retire gates, and no CARP VIP left in the old /24 (task i24).
func (NetworkMigration) Retire() { Sh(migrationHelper + " retire 900") }

// RetireRecovery removes the igc1 recovery alias202 and the 10.99.0.2 return
// route via 10.98.0.1 under a 900-second journal, so wg2 replies follow the
// 10.20.0.1 default (task j24). It needs the retired old IPv4 (i24) and the
// activation, igc1 and single-use retire-recovery gates.
func (NetworkMigration) RetireRecovery() { Sh(migrationHelper + " retire-recovery 900") }

// Confirm accepts a transaction after separate fresh management/service checks.
func (NetworkMigration) Confirm() { Sh(migrationHelper + " confirm") }

// Rollback restores the pending transaction without restarting netif or guests.
func (NetworkMigration) Rollback() { Sh(migrationHelper + " rollback") }

// Status reports the persistent rollback deadline and current transaction.
func (NetworkMigration) Status() { Sh(migrationHelper + " status") }
