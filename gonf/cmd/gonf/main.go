package main

import (
	"os"

	"codeberg.org/snonux/conf/gonf/internal/fleet"
	"codeberg.org/snonux/conf/gonf/internal/frontends"
	"codeberg.org/snonux/conf/gonf/internal/pis"
	. "github.com/snonux/gonf/api"
	"github.com/snonux/gonf/cli"
)

func main() {
	fleet.Register()
	RegisterMethods(frontends.Unattended{}, WithPrefix("frontends_"))
	RegisterMethods(pis.NetBSD{}, WithPrefix("pis_netbsd_"))
	RegisterMethods(pis.Rocky{}, WithPrefix("pis_rocky_"))
	Aggregate("frontends", "Install all frontends_* configuration", "^frontends_")
	Aggregate("pis_netbsd", "Install all pis_netbsd_* configuration", "^pis_netbsd_")
	Aggregate("pis_rocky", "Install all pis_rocky_* configuration", "^pis_rocky_")
	os.Exit(cli.CLI())
}
