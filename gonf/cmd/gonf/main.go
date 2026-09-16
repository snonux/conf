package main

import (
	"os"

	"codeberg.org/snonux/conf/gonf/internal/fleet"
	"codeberg.org/snonux/conf/gonf/internal/frontends"
	. "github.com/snonux/gonf/api"
	"github.com/snonux/gonf/cli"
)

func main() {
	fleet.Register()
	RegisterMethods(frontends.Unattended{}, WithPrefix("frontends_"))
	Aggregate("frontends", "Install all frontends_* configuration", "^frontends_")
	os.Exit(cli.CLI())
}
