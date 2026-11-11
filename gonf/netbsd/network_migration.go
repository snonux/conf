package netbsd

import (
	"fmt"
	"net/netip"

	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

// NetworkMigration owns the NetBSD controller address/NPF transaction.
type NetworkMigration struct{ RequiresRoot }

// MigrationHost declares reserved mue0 addresses; tun0 is never reconfigured.
type MigrationHost struct {
	Interface, OldIPv4, LabIPv4, RecoveryIPv4, ULA, SharedGateway string
}

func (NetworkMigration) OptsPrepare() TaskOptions  { return TaskOptions{Operational()} }
func (NetworkMigration) OptsStage() TaskOptions    { return TaskOptions{Operational()} }
func (NetworkMigration) OptsPromote() TaskOptions  { return TaskOptions{Operational()} }
func (NetworkMigration) OptsConfirm() TaskOptions  { return TaskOptions{Operational()} }
func (NetworkMigration) OptsRollback() TaskOptions { return TaskOptions{Operational()} }
func (NetworkMigration) OptsStatus() TaskOptions   { return TaskOptions{Operational()} }
func (NetworkMigration) OptsRetire() TaskOptions   { return TaskOptions{Operational()} }

// Prepare installs dormant transactions, a no-op watchdog and ordered boot recovery.
func (NetworkMigration) Prepare() {
	EachHostNamed(func(host string, h MigrationHost) {
		if h.Interface != "mue0" {
			panic("NetBSD migration requires the reviewed mue0 interface")
		}
		for _, address := range List(h.OldIPv4, h.LabIPv4, h.RecoveryIPv4, h.SharedGateway) {
			if !netip.MustParseAddr(address).Is4() {
				panic("NetBSD migration requires IPv4 reservations")
			}
		}
		if address := netip.MustParseAddr(h.ULA); !address.Is6() || !address.IsPrivate() {
			panic("NetBSD migration requires native ULA")
		}
		check := Command("test", List("!", "-d", "/var/db/gonf-network-migration/pending"))
		EnsureDir("/usr/local/etc", RootOwned)
		EnsureDir("/usr/local/sbin", RootOwned)
		EnsureDir("/var/db/gonf-network-migration", RootPrivate)
		conf := File("/usr/local/etc/gonf-network-migration.conf", WithContent(fmt.Sprintf(
			"host='%s'\niface='%s'\nold='%s'\nlab='%s'\nrecovery='%s'\nula='%s'\nshared_gateway='%s'\n",
			host, h.Interface, h.OldIPv4, h.LabIPv4, h.RecoveryIPv4, h.ULA, h.SharedGateway)), RootPrivate, DependsOn(check))
		candidate := InstallFile("/usr/local/etc/gonf-network-migration.npf",
			paths.GonfAsset("netbsd", "network-migration/npf.conf"), RootPrivate,
			WithTemplateData(h), WithValidation("/sbin/npfctl", List("validate", CandidatePath)), DependsOn(conf))
		script := InstallFile("/usr/local/sbin/gonf-network-migration",
			paths.GonfAsset("netbsd", "network-migration/migrate.sh"), RootExec, DependsOn(candidate),
			WithValidation("/bin/sh", List("-n", CandidatePath)))
		InstallFile("/etc/rc.d/gonf_network_migration",
			paths.GonfAsset("netbsd", "network-migration/rc.d"), RootExec, DependsOn(script))
		EnsureDir("/etc/rc.conf.d", RootOwned)
		File("/etc/rc.conf.d/gonf_network_migration", WithContent("gonf_network_migration=YES\n"), RootPrivate)
		log := Command("/bin/sh", List("-ec",
			"umask 077; : >> /var/log/gonf-network-migration.log; chmod 600 /var/log/gonf-network-migration.log"),
			WithName("network-migration-private-log"))
		CronAt("netbsd-network-watchdog", "* * * * *",
			"/usr/local/sbin/gonf-network-migration watchdog >>/var/log/gonf-network-migration.log 2>&1", DependsOn(script, log))
	})
}

// Stage dispatches a guarded shared-wire dual-address transaction.
func (NetworkMigration) Stage() { Sh("/usr/local/sbin/gonf-network-migration request stage shared") }

// Promote dispatches NEW primary selection while retaining OLD access.
func (NetworkMigration) Promote() {
	Sh("/usr/local/sbin/gonf-network-migration request promote shared")
}

// Retire dispatches the guarded removal of the retained OLD IPv4 (task i24).
// It needs the accepted promote isolated profile and the activation, igc1 and
// single-use retire gates; it rewrites rc.conf's OLD primary/defaultroute to
// the lab primary and 10.20.0.1 under the same 900-second journal.
func (NetworkMigration) Retire() {
	Sh("/usr/local/sbin/gonf-network-migration request retire isolated")
}

// Confirm accepts independently verified network and service state.
func (NetworkMigration) Confirm() { Sh("/usr/local/sbin/gonf-network-migration confirm") }

// Rollback dispatches restoration independently of the SSH origin.
func (NetworkMigration) Rollback() { Sh("/usr/local/sbin/gonf-network-migration request rollback") }

// Status reports pending and accepted modes without exposing WireGuard keys.
func (NetworkMigration) Status() { Sh("/usr/local/sbin/gonf-network-migration status") }
