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
	"github.com/snonux/conf/gonf/wireguard"
	. "github.com/snonux/gonf/api"
)

// Register makes every current recipe group and its deployment aggregate
// available to the CLI. New recipe groups (Rex, the former engine, was
// retired in conf task v42) join this composition root rather than
// extending main directly.
//
// OnCluster binds each group to its cluster (for EachHost) and guards every
// task to that cluster's hosts, so the task bodies need no hostname wrapper.
// RegisterOnCluster does the same for structs whose default prefix
// (package_type_) is their task prefix; RegisterMethods with WithPrefix
// keeps the prefixes that differ from it.
func Register() {
	RegisterMethods(openbsd.Unattended{}, WithPrefix("frontends_"), OnCluster(cluster.NameFrontends))
	RegisterMethods(frontends.Maintenance{}, WithPrefix("frontends_"), OnCluster(cluster.NameFrontends))
	RegisterMethods(frontends.Monitoring{}, WithPrefix("frontends_"), OnCluster(cluster.NameFrontends))
	RegisterMethods(frontends.Web{}, WithPrefix("frontends_"), OnCluster(cluster.NameFrontends))
	RegisterMethods(frontends.MailDNS{}, WithPrefix("frontends_"), OnCluster(cluster.NameFrontends))
	RegisterOnCluster(cluster.NameFrontends, frontends.WireGuard{})

	RegisterMethods(netbsd.Unattended{}, WithPrefix("pis_netbsd_"), OnCluster(cluster.NameNetBSDPis))
	RegisterMethods(netbsd.Periodic{}, WithPrefix("pis_netbsd_periodic_"), OnCluster(cluster.NameNetBSDPis))

	RegisterMethods(rocky.Unattended{}, WithPrefix("rocky_"), OnCluster(cluster.NameRockyAll))
	// Only the Pis boot the SIG AltArch kernel dnf updateinfo cannot audit.
	// Its rocky_kernel_audit_ prefix keeps it in the "rocky" aggregate;
	// rocky_kernel_audit_packages installs its own ksh interpreter, so it
	// also deploys on its own.
	RegisterOnCluster(cluster.NameRockyPis, rocky.KernelAudit{})
	RegisterMethods(pihole.Deployment{}, WithPrefix("pihole_"), OnCluster(cluster.NameRockyPis))

	RegisterMethods(rnodes.Maintenance{}, WithPrefix("rnodes_"), OnCluster(cluster.NameRockyK3s))
	RegisterOnCluster(cluster.NameRockyK3s, rnodes.Base{}, rnodes.K3s{}, rnodes.WireGuard{})

	RegisterMethods(freebsd.Unattended{}, WithPrefix("freebsd_"), OnCluster(cluster.NameFreeBSD))
	RegisterMethods(freebsd.ZfsKeys{}, WithPrefix("freebsd_zfskeys_"), OnCluster(cluster.NameFreeBSD))
	RegisterMethods(freebsd.ShellyFans{}, WithPrefix("freebsd_shellyfans_"), OnCluster(cluster.NameFreeBSD))
	RegisterMethods(freebsd.RootMail{}, WithPrefix("freebsd_rootmail_"), OnCluster(cluster.NameFreeBSD))
	// Carp, NFS and Relayd serve the CARP pair: their f0/f1-only tasks
	// narrow further with WhenHostnameIn companions.
	RegisterOnCluster(cluster.NameFreeBSD,
		freebsd.Carp{}, freebsd.NFS{}, freebsd.Relayd{},
		freebsd.Periodic{}, freebsd.Zrepl{}, freebsd.Zusb{}, freebsd.Apcupsd{},
		freebsd.Bhyve{}, freebsd.Debug{}, freebsd.Microcode{}, freebsd.Loader{},
		freebsd.Base{}, freebsd.Baseline{}, freebsd.Goprecords{},
		freebsd.Monitoring{}, freebsd.F3sctl{}, freebsd.WireGuard{})

	RegisterMethods(garage.Deployment{}, WithPrefix("garage_"), OnCluster(cluster.NameGarage))
	// Controller-local and Operational: wireguard_mesh_install runs
	// ~/git/wireguardmeshgenerator on the laptop, which pushes to the whole
	// mesh itself. No OnCluster (it targets no gonf cluster) and no aggregate
	// matches ^wireguard_.
	RegisterMethods(wireguard.Mesh{})

	// A pattern Aggregate never picks up an Operational() task, so the
	// frontends aggregate leaves out the explicit, by-name actions:
	// frontends_acme_invoke (requesting certificates; setup only installs
	// the ACME config and daily hook), frontends_irc_bouncer (the existing
	// ZNC deployment) and frontends_ping (the push-pipeline diagnostic).
	// A new frontends_* task joins the aggregate unless it is Operational.
	AggregatePrefix("frontends", "Install all frontend configuration except explicit ACME invocation")
	AggregatePrefix("pis_netbsd")
	AggregatePrefix("rocky")
	AggregatePrefix("rocky_kernel_audit", "Install the Rocky Pi kernel CVE audit")
	AggregatePrefix("pihole", "Install the Pi-hole Docker deployment on pi2/pi3")
	AggregatePrefix("rnodes")
	AggregatePrefix("freebsd")
	AggregatePrefix("garage", "Install all Garage configuration")
}
