package netbsd

import (
	. "github.com/snonux/gonf/api"
)

// The network-migration helper (tasks 624, i24, j24) left a minute watchdog
// cron, an rc.d boot hook with its rc.conf.d switch, the helper, its config,
// NPF candidate, log and journal directory on pi0/pi1. Its last accepted mode
// ("retire lab") only added the lab ULA at boot, which Network.RcConf now
// writes to rc.conf. RetireMigration removes all of it (task b74); it is a
// one-shot task, deleted in a later commit once both hosts are clean.
//
// /var/db/gonf-{shelly-migration,p34-shelly,f54-f3sctl} and the endpoint
// helper (gonf-endpoint-migration) belong to other tasks and stay.
const migrationState = "/var/db/gonf-network-migration"

// OptsRetireMigration keeps the one-shot removal out of the pis_netbsd
// aggregate and orders it after rc.conf carries the full lab addressing.
func (Network) OptsRetireMigration() TaskOptions {
	return TaskOptions{Operational(), Needs(Network.RcConf, Network.NPF)}
}

// RetireMigration removes the network-migration watchdog, boot hook and state.
func (Network) RetireMigration() {
	// An unconfirmed transaction still owns a rollback snapshot: refuse
	// rather than drop the only way back to the prior addresses.
	idle := Command("test", List("!", "-d", migrationState+"/pending"),
		WithName("network-migration-idle"))
	cron := NoCron("netbsd-network-watchdog", DependsOn(idle))
	hook := NoFile(List("/etc/rc.conf.d/gonf_network_migration", "/etc/rc.d/gonf_network_migration"),
		DependsOn(cron))
	helper := NoFile(List(
		"/usr/local/sbin/gonf-network-migration",
		"/usr/local/etc/gonf-network-migration.conf",
		"/usr/local/etc/gonf-network-migration.npf",
		"/var/log/gonf-network-migration.log",
	), DependsOn(hook))
	NoDir(migrationState, WithPrune, DependsOn(helper))
}
