package main

import (
	"os"

	"codeberg.org/snonux/conf/gonf/cluster"
	"codeberg.org/snonux/conf/gonf/tasks"
	"github.com/snonux/gonf/cli"
)

func main() {
	cluster.Register()
	tasks.Register()
	os.Exit(cli.CLI())
}
