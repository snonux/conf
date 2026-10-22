package freebsd

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// Relayd manages the LAN ingress of the k3s cluster on the CARP pair f0/f1
// (f3s-k3s skill, references/ingress.md; blog part 7): relayd listens on the
// CARP VIP 192.168.1.138 ports 80 and 443 and forwards plain TCP to Traefik
// on r0-r2. It is pure TCP passthrough, Traefik terminates TLS for
// *.f3s.lan.buetow.org, so relayd holds no certificate. PF is managed by
// Firewall on all f-hosts; relayd depends on its validated ruleset.
//
// Both hosts carry the same ingress config on purpose: after a CARP failover
// f1 must serve the VIP exactly like f0. On the BACKUP relayd runs too (the
// VIP address stays configured on re0, CARP only stops answering for it), so
// a failover needs no service start.
//
// relayd.conf is validated before publication and relayd is restarted only
// when its configuration changed. A reload (SIGHUP) keeps state such as TLS
// keypairs; see f3s/freebsd-hosts/pf/README.md for firewall rollout safety.
//
// The backend table lists r0-r2's LAN addresses literally. The r-node
// inventory (rnodes.K3sNode) only carries their wg0 addresses, and relayd
// must reach the nodes on the LAN, so deriving the table would mean a second
// per-host field used only here; the table changes only with the k3s nodes.
//
// Register with OnCluster(cluster.NameFreeBSD); the bodies narrow to the CARP
// members with WhenHostname, since f2/f3 have no VIP and run no relayd.
type Relayd struct {
	RequiresRoot
}

const relaydConf = "/usr/local/etc/relayd.conf"

// DescDaemon returns the description for relayd.
func (Relayd) DescDaemon() string {
	return "relayd on f0/f1: package, " + relaydConf + " (VIP :80/:443 -> r0-r2, TCP passthrough), validated with relayd -n; enabled, running, restarted only when the config changed"
}

// OptsDaemon records PF first: relayd depends on PF being enabled.
func (Relayd) OptsDaemon() TaskOptions {
	return TaskOptions{Needs(Firewall.Config)}
}

// WhenDaemon narrows the task to the CARP members.
func (Relayd) WhenDaemon() TaskOption { return WhenHostnameIn(carpMembers...) }

// Daemon installs relayd and its config after a relayd -n check of the
// candidate, keeps the service enabled (relayd_enable="YES") and running,
// and restarts it only when relayd.conf changed (see the Relayd comment for
// why restart and not reload).
func (Relayd) Daemon() {
	pkg := Package("relayd")
	conf := InstallFile(relaydConf, paths.FHostAsset("relayd/relayd.conf"), RootOwned,
		WithValidation("relayd", List("-n", "-f", CandidatePath)), DependsOn(pkg))
	Service("relayd", WithRestart, OnChange(conf))
}
