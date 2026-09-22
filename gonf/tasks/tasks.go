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

const frontendAggregatePattern = `^frontends_(acme|base|cron|dns_failover|d_tail|foostats|gemtexter|gogios|goprecords|httpd|inetd|myname|newsyslog|nsd|pf|ping|pkg_repo|relayd|rsync|script|service_accounts|services|smtpd|uptimed|wire_guard_hosts)$`

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
	// Its rocky_kernel_audit_ prefix keeps it in the "rocky" aggregate; the
	// ksh interpreter it runs under comes from rocky_packages.
	RegisterMethods(rocky.KernelAudit{}, WithPrefix("rocky_kernel_audit_"), WithCluster(cluster.NameRockyPis))
	RegisterMethods(rnodes.Maintenance{}, WithPrefix("rnodes_"), WithCluster(cluster.NameRockyK3s))
	RegisterMethods(freebsd.Unattended{}, WithPrefix("freebsd_"), WithCluster(cluster.NameFreeBSD))
	RegisterMethods(garage.Deployment{}, WithPrefix("garage_"), WithCluster(cluster.NameGarage))

	// ACME invocation is deliberately excluded: setup only installs its config
	// and daily hook, while requesting certificates remains an explicit action.
	Aggregate("frontends", "Install all frontend configuration except explicit ACME invocation", frontendAggregatePattern)
	Aggregate("pis_netbsd", "Install all pis_netbsd_* configuration", "^pis_netbsd_")
	Aggregate("rocky", "Install all rocky_* configuration", "^rocky_")
	Aggregate("rocky_kernel_audit", "Install the Rocky Pi kernel CVE audit", "^rocky_kernel_audit_")
	Aggregate("rnodes", "Install all rnodes_* configuration", "^rnodes_")
	Aggregate("freebsd", "Install all freebsd_* configuration", "^freebsd_")
	Aggregate("garage", "Install all Garage configuration", "^garage_")
}
