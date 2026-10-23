package frontends

import (
	"fmt"

	. "github.com/snonux/gonf/api"
)

// addPeerCertificateChecks covers the four ro3 pairs, including the primary
// Gogios pair's www SAN, on this frontend's own IPv4 and IPv6. DNS-selected
// checks miss the inactive copies after owner-only ACME renewal. Local keeps
// these checks running on the passive checker too, without ping dependencies.
// check_curl -D verifies hostname and trust; -C warns at 20 days, critical at 7.
func addPeerCertificateChecks(checks map[string]gogiosCheck, server Server) {
	for _, name := range List("standby.ecat.buetow.org", "standby.alt.irregular.ninja",
		"standby.gogios.buetow.org", "gogios.buetow.org", "www.gogios.buetow.org") {
		for _, family := range []int{4, 6} {
			address := server.IPv4
			if family == 6 {
				address = server.IPv6
			}
			checks[fmt.Sprintf("Check Peer HTTPS IPv%d %s on %s", family, name, server.Name)] = gogiosCheck{
				Plugin: gogiosPluginDir + "/check_curl",
				Args: List("-S", "-D", "--sni", "-H", name, "-I", address,
					fmt.Sprintf("-%d", family), "-C", "20,7", "--continue-after-certificate",
					"-e", "HTTP/1.1 200", "-t", "10"),
				RandomSpread: 10, RunInterval: 300, Local: true,
			}
		}
	}
}
