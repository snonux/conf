package rocky

import (
	"fmt"
	"net/netip"

	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

const migrationHelper = "/usr/local/sbin/gonf-network-migration"

// NetworkMigration prepares guarded physical-interface address transactions.
// Register on reviewed host scopes; every action is excluded from aggregates.
type NetworkMigration struct{ RequiresRoot }

// MigrationHost supplies reserved physical addresses without changing wg0.
type MigrationHost struct {
	Interface, OldIPv4, LabIPv4, RecoveryIPv4, ULA, SharedGateway string
	// Standalone omits every k3s operation; IPv6Policy must explicitly be disabled.
	Standalone bool
	IPv6Policy string
}

func (NetworkMigration) OptsPrepare() TaskOptions  { return TaskOptions{Operational()} }
func (NetworkMigration) OptsStage() TaskOptions    { return TaskOptions{Operational()} }
func (NetworkMigration) OptsPromote() TaskOptions  { return TaskOptions{Operational()} }
func (NetworkMigration) OptsConfirm() TaskOptions  { return TaskOptions{Operational()} }
func (NetworkMigration) OptsRollback() TaskOptions { return TaskOptions{Operational()} }
func (NetworkMigration) OptsStatus() TaskOptions   { return TaskOptions{Operational()} }

// Prepare installs dormant NetworkManager transaction and PID1 rollback tools.
func (NetworkMigration) Prepare() {
	EachHost(func(h MigrationHost) {
		role, ipv6Policy := "k3s", "ula"
		if h.Standalone {
			role, ipv6Policy = "standalone", h.IPv6Policy
			if ipv6Policy != "disabled" || h.ULA != "" {
				panic("standalone migration requires explicit disabled physical IPv6 and no ULA")
			}
		} else if addr := netip.MustParseAddr(h.ULA); !addr.Is6() || !addr.IsPrivate() {
			panic("k3s migration requires native ULA")
		}
		if h.Interface != "enp0s5" {
			panic("migration requires reviewed enp0s5 interface")
		}
		for _, a := range List(h.OldIPv4, h.LabIPv4, h.RecoveryIPv4, h.SharedGateway) {
			if !netip.MustParseAddr(a).Is4() {
				panic("migration requires IPv4")
			}
		}
		check := Command("test", List("!", "-d", "/var/lib/gonf-network-migration/pending"))
		EnsureDir("/usr/local/sbin", RootOwned)
		EnsureDir("/var/lib/gonf-network-migration", RootPrivate)
		conf := File("/etc/gonf-network-migration.conf", WithContent(fmt.Sprintf(
			"iface='%s'\nold='%s'\nlab='%s'\nrecovery='%s'\nula='%s'\nshared_gateway='%s'\nrole='%s'\nipv6_policy='%s'\n",
			h.Interface, h.OldIPv4, h.LabIPv4, h.RecoveryIPv4, h.ULA, h.SharedGateway, role, ipv6Policy)), RootPrivate, DependsOn(check))
		script := InstallFile(migrationHelper, paths.GonfAsset("rocky", "network-migration/migrate.sh"), RootExec, DependsOn(conf))
		for _, unit := range List("gonf-network-rollback.service", "gonf-network-rollback.timer", "gonf-network-rollback-boot.service", "gonf-network-rollback-runtime.service") {
			asset := unit
			if h.Standalone && unit == "gonf-network-rollback-runtime.service" {
				asset = "standalone-runtime.service"
			}
			InstallFile("/etc/systemd/system/"+unit, paths.GonfAsset("rocky", "network-migration/"+asset), RootOwned, DependsOn(script))
		}
		// install only: even daemon-reload/enable occurs later under explicit init.
	})
}

// Stage requests a guarded PID1-owned shared-wire dual-address transaction.
func (NetworkMigration) Stage() { Sh(migrationHelper + " request stage shared") }

// Promote selects the lab primary, refreshing k3s only for k3s hosts.
func (NetworkMigration) Promote() { Sh(migrationHelper + " request promote shared") }

// Confirm accepts independently verified fresh management and service checks.
func (NetworkMigration) Confirm() { Sh(migrationHelper + " confirm") }

// Rollback requests restoration through PID1 independently of the SSH session.
func (NetworkMigration) Rollback() { Sh(migrationHelper + " request rollback") }

// Status reports pending state without exposing NetworkManager credentials.
func (NetworkMigration) Status() { Sh(migrationHelper + " status") }
