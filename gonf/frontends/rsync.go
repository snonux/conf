package frontends

import (
	"fmt"
	"strings"

	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

type rsyncConfigData struct {
	Publishers []string
	Readers    []string
	StatsPeers []string
}

func installRsyncConfig() {
	password := strings.TrimSuffix(MustSecret(paths.FrontendSecret("etc/rsync/shuriken.password")), "\n")
	if password == "" || strings.ContainsAny(password, "\r\n\x00") || strings.TrimSpace(password) != password {
		Refuse("File", "/etc/rsyncd.secrets", fmt.Errorf("rsync password must be a nonempty single line without surrounding whitespace"))
		return
	}
	data := rsyncConfigData{Publishers: List("127.0.0.1", "::1"), Readers: List("127.0.0.1", "::1"), StatsPeers: List("127.0.0.1", "::1")}
	for _, host := range List("r0", "r1", "r2", "blowfish", "fishfinger") {
		peer, err := wireGuardAddressFor(host)
		if err != nil {
			Refuse("File", "/etc/rsyncd.conf", err)
			return
		}
		data.Publishers = append(data.Publishers, peer.IPv4, peer.IPv6)
		if host == "fishfinger" {
			data.Readers = append(data.Readers, peer.IPv4, peer.IPv6)
		}
		if host == "blowfish" || host == "fishfinger" {
			data.StatsPeers = append(data.StatsPeers, peer.IPv4, peer.IPv6)
		}
	}
	config, err := renderControllerTemplate(frontendAsset("rsyncd.conf.tmpl"), data)
	if err != nil {
		Refuse("File", "/etc/rsyncd.conf", err)
		return
	}
	credentials := File("/etc/rsyncd.secrets", WithContent("shuriken:"+password+"\n"), RootPrivate, WithSensitive)
	// inetd starts a new daemon per connection; rsync rereads its config.
	// Credentials must exist before authenticated modules become active.
	File("/etc/rsyncd.conf", WithContent(config), RootOwned, DependsOn(credentials))
}
