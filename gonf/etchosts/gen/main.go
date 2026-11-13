// Command gen writes the fleet records that the frontends recipes read as
// files (an import of etchosts from frontends would be a cycle): the hosts
// rows of frontends.Maintenance.Hosts and the NSD records that
// frontends.MailDNS.NSD merges into the buetow.org zone. Run it through
// go generate in gonf/etchosts. It only writes the two given files.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/snonux/conf/gonf/etchosts"
)

func main() {
	hosts := flag.String("hosts", "", "output file for the frontends' fleet hosts rows")
	nsd := flag.String("nsd", "", "output file for the authoritative LAN records")
	flag.Parse()
	if *hosts == "" || *nsd == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: gen -hosts FILE -nsd FILE")
		os.Exit(2)
	}
	for path, lines := range map[string][]string{
		*hosts: etchosts.FrontendHostsLines(),
		*nsd:   etchosts.NSDRecords(),
	} {
		if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
