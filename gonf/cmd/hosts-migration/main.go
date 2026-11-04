// hosts-migration prints inactive hosts/DNS fragments as JSON. It has no
// installation, SSH, secret-provider or service-control capability.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/snonux/conf/gonf/etchosts"
)

func main() {
	profile := flag.String("profile", "staging", "staging, target (AP held old), or ap-target (after d24)")
	view := flag.String("router-view", "upstream", "canonical router name: upstream, lab, or wifi")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(2)
	}
	candidate, err := etchosts.RenderMigration(*profile, *view)
	if err == nil {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(candidate)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
