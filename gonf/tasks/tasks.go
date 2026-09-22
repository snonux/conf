// Package tasks wires recipe groups into the gonf CLI composition root.
package tasks

import (
	"codeberg.org/snonux/conf/gonf/cluster"
	"codeberg.org/snonux/conf/gonf/freebsd"
	"codeberg.org/snonux/conf/gonf/frontends"
	"codeberg.org/snonux/conf/gonf/garage"
	"codeberg.org/snonux/conf/gonf/netbsd"
	"codeberg.org/snonux/conf/gonf/openbsd"
	"codeberg.org/snonux/conf/gonf/rnodes"
	"codeberg.org/snonux/conf/gonf/rocky"
	. "github.com/snonux/gonf/api"
)

// frontendSetupTasks returns the explicit membership of the frontends setup
// aggregate, in recording order (alphabetical, the order the former regex
// recorded it in, so the aggregate's plan is unchanged). It deliberately
// leaves out the operational actions frontends_acme_invoke (requesting
// certificates) and frontends_irc_bouncer (the existing ZNC deployment): both
// stay explicit, by-name actions. A new frontends_* setup task must be added
// here to join the aggregate; AggregateTasks fails the record on a name that
// is not registered, so a typo cannot silently shrink a setup run.
func frontendSetupTasks() []string {
	return []string{
		"frontends_acme",
		"frontends_base",
		"frontends_cron",
		"frontends_d_tail",
		"frontends_dns_failover",
		"frontends_foostats",
		"frontends_gemtexter",
		"frontends_gogios",
		"frontends_goprecords",
		"frontends_httpd",
		"frontends_inetd",
		"frontends_myname",
		"frontends_newsyslog",
		"frontends_nsd",
		"frontends_pf",
		"frontends_ping",
		"frontends_pkg_repo",
		"frontends_relayd",
		"frontends_rsync",
		"frontends_script",
		"frontends_service_accounts",
		"frontends_services",
		"frontends_smtpd",
		"frontends_uptimed",
		"frontends_wire_guard_hosts",
	}
}

// Register makes every current recipe group and its deployment aggregate
// available to the CLI. Future Rex ports join this composition root rather
// than extending main directly.
func Register() {
	RegisterMethods(openbsd.Unattended{}, WithPrefix("frontends_"), WithCluster(cluster.NameFrontends))
	RegisterMethods(frontends.Maintenance{}, WithPrefix("frontends_"), WithCluster(cluster.NameFrontends))
	RegisterMethods(frontends.Monitoring{}, WithPrefix("frontends_"), WithCluster(cluster.NameFrontends))
	RegisterMethods(frontends.Web{}, WithPrefix("frontends_"), WithCluster(cluster.NameFrontends))
	RegisterMethods(frontends.MailDNS{}, WithPrefix("frontends_"), WithCluster(cluster.NameFrontends))
	RegisterMethods(netbsd.Unattended{}, WithPrefix("pis_netbsd_"), WithCluster(cluster.NameNetBSDPis))
	RegisterMethods(rocky.Unattended{}, WithPrefix("rocky_"), WithCluster(cluster.NameRockyAll))
	// Only the Pis boot the SIG AltArch kernel dnf updateinfo cannot audit.
	// Its rocky_kernel_audit_ prefix keeps it in the "rocky" aggregate;
	// rocky_kernel_audit_packages installs its own ksh interpreter, so it
	// also deploys on its own.
	RegisterMethods(rocky.KernelAudit{}, WithPrefix("rocky_kernel_audit_"), WithCluster(cluster.NameRockyPis))
	RegisterMethods(rnodes.Maintenance{}, WithPrefix("rnodes_"), WithCluster(cluster.NameRockyK3s))
	RegisterMethods(freebsd.Unattended{}, WithPrefix("freebsd_"), WithCluster(cluster.NameFreeBSD))
	RegisterMethods(garage.Deployment{}, WithPrefix("garage_"), WithCluster(cluster.NameGarage))

	// The frontends aggregate lists its setup members explicitly (see
	// frontendSetupTasks): ACME invocation is excluded because setup only
	// installs its config and daily hook, while requesting certificates
	// remains an explicit action; the IRC bouncer likewise stays by-name.
	AggregateTasks("frontends", "Install all frontend configuration except explicit ACME invocation", frontendSetupTasks()...)
	Aggregate("pis_netbsd", "Install all pis_netbsd_* configuration", "^pis_netbsd_")
	Aggregate("rocky", "Install all rocky_* configuration", "^rocky_")
	Aggregate("rocky_kernel_audit", "Install the Rocky Pi kernel CVE audit", "^rocky_kernel_audit_")
	Aggregate("rnodes", "Install all rnodes_* configuration", "^rnodes_")
	Aggregate("freebsd", "Install all freebsd_* configuration", "^freebsd_")
	Aggregate("garage", "Install all Garage configuration", "^garage_")
}
