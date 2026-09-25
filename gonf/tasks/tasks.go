// Package tasks wires recipe groups into the gonf CLI composition root.
package tasks

import (
	"github.com/snonux/conf/gonf/cluster"
	"github.com/snonux/conf/gonf/freebsd"
	"github.com/snonux/conf/gonf/frontends"
	"github.com/snonux/conf/gonf/garage"
	"github.com/snonux/conf/gonf/netbsd"
	"github.com/snonux/conf/gonf/openbsd"
	"github.com/snonux/conf/gonf/pihole"
	"github.com/snonux/conf/gonf/rnodes"
	"github.com/snonux/conf/gonf/rocky"
	. "github.com/snonux/gonf/api"
)

// Register makes every current recipe group and its deployment aggregate
// available to the CLI. New recipe groups (Rex, the former engine, was
// retired in conf task v42) join this composition root rather than
// extending main directly.
//
// OnCluster binds each group to its cluster (for EachHost) and guards every
// task to that cluster's hosts, so the task bodies need no hostname wrapper.
func Register() {
	RegisterMethods(openbsd.Unattended{}, WithPrefix("frontends_"), OnCluster(cluster.NameFrontends))
	RegisterMethods(frontends.Maintenance{}, WithPrefix("frontends_"), OnCluster(cluster.NameFrontends))
	RegisterMethods(frontends.Monitoring{}, WithPrefix("frontends_"), OnCluster(cluster.NameFrontends))
	RegisterMethods(frontends.Web{}, WithPrefix("frontends_"), OnCluster(cluster.NameFrontends))
	RegisterMethods(frontends.MailDNS{}, WithPrefix("frontends_"), OnCluster(cluster.NameFrontends))
	RegisterMethods(netbsd.Unattended{}, WithPrefix("pis_netbsd_"), OnCluster(cluster.NameNetBSDPis))
	RegisterMethods(rocky.Unattended{}, WithPrefix("rocky_"), OnCluster(cluster.NameRockyAll))
	// Only the Pis boot the SIG AltArch kernel dnf updateinfo cannot audit.
	// Its rocky_kernel_audit_ prefix keeps it in the "rocky" aggregate;
	// rocky_kernel_audit_packages installs its own ksh interpreter, so it
	// also deploys on its own.
	RegisterMethods(rocky.KernelAudit{}, WithPrefix("rocky_kernel_audit_"), OnCluster(cluster.NameRockyPis))
	RegisterMethods(pihole.Deployment{}, WithPrefix("pihole_"), OnCluster(cluster.NameRockyPis))
	RegisterMethods(rnodes.Maintenance{}, WithPrefix("rnodes_"), OnCluster(cluster.NameRockyK3s))
	RegisterMethods(freebsd.Unattended{}, WithPrefix("freebsd_"), OnCluster(cluster.NameFreeBSD))
	// The CARP helpers narrow to f0/f1 inside the bodies (WhenHostname).
	RegisterMethods(freebsd.Carp{}, WithPrefix("freebsd_carp_"), OnCluster(cluster.NameFreeBSD))
	RegisterMethods(freebsd.Periodic{}, WithPrefix("freebsd_periodic_"), OnCluster(cluster.NameFreeBSD))
	RegisterMethods(freebsd.Apcupsd{}, WithPrefix("freebsd_apcupsd_"), OnCluster(cluster.NameFreeBSD))
	RegisterMethods(freebsd.Bhyve{}, WithPrefix("freebsd_bhyve_"), OnCluster(cluster.NameFreeBSD))
	RegisterMethods(freebsd.Debug{}, WithPrefix("freebsd_debug_"), OnCluster(cluster.NameFreeBSD))
	RegisterMethods(freebsd.Loader{}, WithPrefix("freebsd_loader_"), OnCluster(cluster.NameFreeBSD))
	RegisterMethods(garage.Deployment{}, WithPrefix("garage_"), OnCluster(cluster.NameGarage))

	// A pattern Aggregate never picks up an Operational() task, so the
	// frontends aggregate leaves out the explicit, by-name actions:
	// frontends_acme_invoke (requesting certificates; setup only installs
	// the ACME config and daily hook), frontends_irc_bouncer (the existing
	// ZNC deployment) and frontends_ping (the push-pipeline diagnostic).
	// A new frontends_* task joins the aggregate unless it is Operational.
	Aggregate("frontends", "Install all frontend configuration except explicit ACME invocation", "^frontends_")
	Aggregate("pis_netbsd", "Install all pis_netbsd_* configuration", "^pis_netbsd_")
	Aggregate("rocky", "Install all rocky_* configuration", "^rocky_")
	Aggregate("rocky_kernel_audit", "Install the Rocky Pi kernel CVE audit", "^rocky_kernel_audit_")
	Aggregate("pihole", "Install the Pi-hole Docker deployment on pi2/pi3", "^pihole_")
	Aggregate("rnodes", "Install all rnodes_* configuration", "^rnodes_")
	Aggregate("freebsd", "Install all freebsd_* configuration", "^freebsd_")
	Aggregate("garage", "Install all Garage configuration", "^garage_")
}
