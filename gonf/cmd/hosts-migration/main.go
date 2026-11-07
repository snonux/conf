// hosts-migration prints inactive hosts/DNS fragments as JSON. It has no
// installation, SSH, secret-provider or service-control capability.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/snonux/conf/gonf/etchosts"
)

func main() {
	profile := flag.String("profile", "staging", "staging, accepted (DHCP/AP held), target, or ap-target (after d24)")
	view := flag.String("router-view", "upstream", "canonical router name: upstream, lab, or wifi")
	format := flag.String("format", "json", "json, hosts, authoritative-hosts, or dnsmasq-hosts")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(2)
	}
	candidate, err := etchosts.RenderMigration(*profile, *view)
	if err == nil {
		switch *format {
		case "json":
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			err = encoder.Encode(candidate)
		case "hosts":
			err = writeLines(append(candidate.HostsBlock, candidate.WireGuardBlock...))
		case "authoritative-hosts":
			err = writeLines(candidate.AuthoritativeLAN)
		case "dnsmasq-hosts":
			err = writeLines(candidate.PiHoleDNSMasq)
		default:
			err = fmt.Errorf("unknown output format %q", *format)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func writeLines(lines []string) error {
	for _, line := range lines {
		// Wildcard promotion has its own cache gate; these fragments contain
		// only explicit hosts. Comments from inactive views are not live claims.
		if strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") ||
			strings.HasPrefix(line, "*.f3s.lan.buetow.org.") ||
			strings.HasPrefix(line, "address=/.f3s.lan.buetow.org/") {
			continue
		}
		if _, err := fmt.Fprintln(os.Stdout, line); err != nil {
			return err
		}
	}
	return nil
}
