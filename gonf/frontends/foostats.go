package frontends

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

func installFoostats(server Server, source, odds string) {
	key := strings.TrimSuffix(MustSecret(paths.FrontendSecret("etc/foostats/ip-hmac.key")), "\n")
	if _, err := hex.DecodeString(key); err != nil || len(key) < 64 || len(key) > 4096 {
		Refuse("File", "/etc/foostats/ip-hmac.key", fmt.Errorf("Foostats HMAC key must encode at least 32 bytes as hex"))
		return
	}
	peerName := Master
	if server.Name == Master {
		peerName = Standby
	}
	peer, err := wireGuardAddressFor(peerName)
	if err != nil {
		Refuse("File", dailyLocal, err)
		return
	}
	deps := Packages("p5-Digest-SHA3", "p5-Digest-HMAC", "p5-PerlIO-gzip", "p5-JSON", "p5-String-Util", "p5-HTML-Parser")
	EnsureDir("/etc/foostats", RootPrivate)
	// httpd is chrooted under /var/www. Rsync's read-only peer module reads
	// these counters as www outside that chroot; www cannot write the tree.
	EnsureDir("/var/db/foostats", Perm(0o750, "root:www"))
	keyFile := File("/etc/foostats/ip-hmac.key", WithContent(key+"\n"), RootPrivate, WithSensitive)
	script := InstallFile("/usr/local/bin/foostats.pl", source, Perm(0o500, Root), DependsOn(deps))
	data := InstallFile("/etc/foostats/fooodds.txt", odds, RootOwned)
	Dir("/var/www/htdocs/gemtexter/stats.foo.zone", RootOwned)
	Dir("/var/gemini/stats.foo.zone", RootOwned)
	command := "perl /usr/local/bin/foostats.pl --parse-logs --replicate --report --partner-node " + peer.Name + "." + Domain + " --partner-address " + peer.IPv4
	File(dailyLocal, WithKeyedLine("perl /usr/local/bin/foostats.pl ", command), RootOwned,
		DependsOn(script, data, keyFile))
	InstallFile("/etc/newsyslog.conf", legacyFrontendAsset("etc/newsyslog.conf"), RootOwned)
}

func foostatsPrivateSource(path string) bool {
	content, err := os.ReadFile(path)
	if err == nil && strings.Contains(string(content), "package Foostats::Privacy") && strings.Contains(string(content), "hmac-sha3-512-v1") {
		return true
	}
	Refuse("File", "/usr/local/bin/foostats.pl", fmt.Errorf("Foostats source lacks private-statistics support; set GONF_FOOSTATS_ROOT to the reviewed an3 source"))
	return false
}
