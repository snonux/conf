package main

import (
	"os"

	"codeberg.org/snonux/conf/gonf/internal/fleet"
	"codeberg.org/snonux/conf/gonf/internal/frontends/openbsd"
	"codeberg.org/snonux/conf/gonf/internal/pis/netbsd"
	"codeberg.org/snonux/conf/gonf/internal/pis/rocky"
	. "github.com/snonux/gonf/api"
	"github.com/snonux/gonf/cli"
)

func main() {
	fleet.Register()
	RegisterMethods(openbsd.Unattended{}, WithPrefix("frontends_"), WithFleet(fleet.NameFrontends))
	RegisterMethods(netbsd.Unattended{}, WithPrefix("pis_netbsd_"), WithFleet(fleet.NameNetBSDPis))
	RegisterMethods(rocky.Unattended{}, WithPrefix("pis_rocky_"), WithFleet(fleet.NameRockyAll))
	Aggregate("frontends", "Install all frontends_* configuration", "^frontends_")
	Aggregate("pis_netbsd", "Install all pis_netbsd_* configuration", "^pis_netbsd_")
	Aggregate("pis_rocky", "Install all pis_rocky_* configuration", "^pis_rocky_")
	os.Exit(cli.CLI())
}
