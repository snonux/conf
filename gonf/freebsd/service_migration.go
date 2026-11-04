package freebsd

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/snonux/conf/gonf/migrationaddress"
	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

// OptsMigrationCandidate keeps UPS candidate preparation out of aggregates.
func (Apcupsd) OptsMigrationCandidate() TaskOptions { return TaskOptions{Operational()} }

// MigrationCandidate stages UPS configuration without rebinding or restarting.
// The master must be promoted and accepted before any network client changes.
func (Apcupsd) MigrationCandidate() {
	EachHostWith(func(s UPSServer) {
		address, err := migrationaddress.Lab(s.NISIP)
		if err != nil {
			Refuse("UPS candidate", apcupsdConf, err)
			return
		}
		s.NISIP = address
		upsCandidate(serverConfData(s))
	})
	EachHostWith(func(c UPSClient) {
		host, port, err := net.SplitHostPort(c.Server)
		if err != nil || port != "3551" {
			Refuse("UPS candidate", apcupsdConf, fmt.Errorf("expected UPS server host:3551"))
			return
		}
		address, err := migrationaddress.Lab(host)
		if err != nil {
			Refuse("UPS candidate", apcupsdConf, err)
			return
		}
		c.Server = net.JoinHostPort(address, port)
		upsCandidate(clientConfData(c))
	})
}

// OptsMigrationCandidate keeps the fan override out of normal aggregates.
func (ShellyFans) OptsMigrationCandidate() TaskOptions { return TaskOptions{Operational()} }

// MigrationCandidate stages the boot fan address override without running it.
// Promotion needs device recovery acceptance and the exact TCP80 router path.
func (ShellyFans) MigrationCandidate() {
	EnsureDir("/usr/local/etc/rc.conf.d", RootOwned)
	File("/usr/local/etc/rc.conf.d/shellyfans.triggerfish-candidate",
		WithContent("# Inactive: merge after Shelly address/recovery acceptance.\nshellyfans_ip=\"10.30.0.28\"\n"), RootOwned)
}

func upsCandidate(data apcupsdConfData) {
	InstallFile(apcupsdConf+".triggerfish-candidate", paths.FHostAsset("apcupsd/apcupsd.conf.tmpl"),
		RootOwned, WithTemplateData(data))
}

// OptsMigrationCandidate keeps restricted-key candidates out of aggregates.
func (F3sctl) OptsMigrationCandidate() TaskOptions { return TaskOptions{Operational()} }

// MigrationCandidate stages additive Pi source pins, retaining old LAN and wg0.
func (F3sctl) MigrationCandidate() {
	raw, err := os.ReadFile(paths.FHostAsset("f3sctl/authorized_keys"))
	if err != nil {
		Refuse("f3sctl candidate", f3sctlKeyFile, err)
		return
	}
	const old = `from="192.168.1.125,192.168.1.126,192.168.2.203,192.168.2.204"`
	if strings.Count(string(raw), old) != 1 {
		Refuse("f3sctl candidate", f3sctlKeyFile, fmt.Errorf("unexpected source pins; review key before migration"))
		return
	}
	content := strings.Replace(string(raw), old,
		`from="192.168.1.125,192.168.1.126,10.20.0.125,10.20.0.126,192.168.2.203,192.168.2.204"`, 1)
	File(f3sctlKeyFile+".triggerfish-candidate", WithContent(content), RootOwned)
}
