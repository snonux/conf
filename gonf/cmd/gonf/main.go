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
	Aggregate("frontends", "Install all frontends_* configuration", "^frontends_")
	Aggregate("pis_netbsd", "Install all pis_netbsd_* configuration", "^pis_netbsd_")
	os.Exit(cli.CLI())
}
