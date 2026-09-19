// Package tasks wires recipe groups into the gonf CLI composition root.
package tasks

import (
	"codeberg.org/snonux/conf/gonf/cluster"
	"codeberg.org/snonux/conf/gonf/freebsd"
	"codeberg.org/snonux/conf/gonf/netbsd"
	"codeberg.org/snonux/conf/gonf/openbsd"
	"codeberg.org/snonux/conf/gonf/rocky"
	. "github.com/snonux/gonf/api"
)

// Register makes every current recipe group and its deployment aggregate
// available to the CLI. Future Rex ports join this composition root rather
// than extending main directly.
func Register() {
	RegisterMethods(openbsd.Unattended{}, WithPrefix("frontends_"), WithCluster(cluster.NameFrontends))
	RegisterMethods(netbsd.Unattended{}, WithPrefix("pis_netbsd_"), WithCluster(cluster.NameNetBSDPis))
	RegisterMethods(rocky.Unattended{}, WithPrefix("rocky_"), WithCluster(cluster.NameRockyAll))
	RegisterMethods(freebsd.Unattended{}, WithPrefix("freebsd_"), WithCluster(cluster.NameFreeBSD))

	Aggregate("frontends", "Install all frontends_* configuration", "^frontends_")
	Aggregate("pis_netbsd", "Install all pis_netbsd_* configuration", "^pis_netbsd_")
	Aggregate("rocky", "Install all rocky_* configuration", "^rocky_")
	Aggregate("freebsd", "Install all freebsd_* configuration", "^freebsd_")
}
