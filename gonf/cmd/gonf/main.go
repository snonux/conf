package main

import (
	"os"

	"codeberg.org/snonux/conf/gonf/internal/fleet"
	"codeberg.org/snonux/conf/gonf/internal/tasks"
	. "github.com/snonux/gonf/api"
	"github.com/snonux/gonf/cli"
)

func main() {
	fleet.Register()
	RegisterMethods(tasks.Frontends{}, WithPrefix("frontends_"))
	tasks.RegisterUnattended()
	Aggregate("frontends", "Install all frontends_* configuration", "^frontends_")
	os.Exit(cli.CLI())
}
