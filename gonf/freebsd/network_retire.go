package freebsd

import (
	. "github.com/snonux/gonf/api"
)

// NetworkRetirement removes the host artifacts of the two FreeBSD
// network-migration helpers (task d74, items 3 and 3a of IQ
// openbsdrouter-plan/CLEANUP.md):
//   - gonf-network-migration (the triggerfish address migration, tasks
//     Y14..j24) on f0-f3 and the standalone guest "freebsd";
//   - gonf-bridge-migration (the task 764 move onto vm-public) on f0-f3.
//
// Each left a minute root cron watchdog, an rc.d boot hook before netif, the
// helper, its config and a journal directory under /var/db. With no pending
// transaction the hooks only re-read rc.conf at boot, which already carries
// the final layout (bridge vm-public with the lab IPv4 and ULA, the CARP VIP
// on f0/f1, defaultrouter 10.20.0.1), so removing them changes no address.
//
// One-shot: once every host ran it, a later commit deletes this file and its
// registrations. It is Operational, so no aggregate picks it up: run it by
// name, one host at a time.
type NetworkRetirement struct{ RequiresRoot }

const (
	networkMigrationState = "/var/db/gonf-network-migration"
	bridgeMigrationState  = "/var/db/gonf-bridge-migration"
)

func (NetworkRetirement) OptsRetire() TaskOptions { return TaskOptions{Operational()} }

// Retire removes the network- and bridge-migration watchdogs, boot hooks, helpers and journals.
func (NetworkRetirement) Retire() {
	// A pending transaction (or the network helper's confirmed-boot guard)
	// still owns a rollback snapshot, and a tagged "unset" line in rc.conf
	// is only removed by the helper's next watchdog run: refuse in both
	// cases rather than drop the only owner. The check runs only when one
	// of them is present, so an idle host previews no change for it.
	busy := `for p in ` + networkMigrationState + `/pending ` + networkMigrationState + `/confirmed-boot-guard ` +
		bridgeMigrationState + `/pending; do [ -e "$p" ] && exit 0; done
grep -Eq 'gonf-(network|bridge)-migration absent keys' ` + rcConf
	idle := Command("sh", List("-c", "echo 'a network or bridge migration transaction is pending' >&2; exit 1"),
		OnlyIf("sh", List("-c", busy)), WithName("network-migration-idle"))
	networkCron := NoCron("gonf-network-rollback", DependsOn(idle))
	bridgeCron := NoCron("gonf-bridge-rollback", DependsOn(idle))
	hooks := NoFile(List("/etc/rc.d/gonf_network_rollback", "/etc/rc.d/gonf_bridge_rollback"),
		DependsOn(networkCron, bridgeCron))
	helpers := NoFile(List(
		"/usr/local/sbin/gonf-network-migration",
		"/usr/local/sbin/gonf-bridge-migration",
		"/usr/local/etc/gonf-network-migration.conf",
		"/usr/local/etc/gonf-bridge-migration.conf",
	), DependsOn(hooks))
	NoDir(List(networkMigrationState, bridgeMigrationState), WithPrune, DependsOn(helpers))
}
