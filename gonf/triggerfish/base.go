// Package triggerfish manages the LAN OpenBSD host baseline.
package triggerfish

import (
	"fmt"
	"github.com/snonux/conf/gonf/etchosts"
	"github.com/snonux/conf/gonf/fleetfiles"

	"github.com/snonux/conf/gonf/goprecords"
	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

//go:generate go run github.com/snonux/gonf/cmd/gonf-desc

// Base manages privileges, uptime collection and uploads.
type Base struct{ RequiresRoot }

// Doas enables passwordless administration for wheel members.
func (Base) Doas() {
	File("/etc/doas.conf", WithContent("permit nopass :wheel\n"), RootPrivate,
		WithValidation("/usr/bin/doas", List("-C", CandidatePath)))
}

// Hostname sets the persistent and running LAN hostname.
func (Base) Hostname() {
	const fqdn = "triggerfish.lan.buetow.org"
	conf := File("/etc/myname", WithContent(fqdn+"\n"), RootOwned)
	Sh("/bin/hostname "+fqdn, OnChange(conf))
}

// Hosts merges the fleet while retaining the router's upstream canonical address.
func (Base) Hosts() { fleetfiles.MergeHosts(etchosts.EarthHostsBlock()) }

// Uptimed installs uprecords and enables persistent uptime collection.
func (Base) Uptimed() {
	pkg := Package("uptimed")
	conf := InstallFile("/etc/uptimed.conf", paths.GonfAsset("goprecords", "uptimed.conf"), RootOwned, DependsOn(pkg))
	Service("uptimed", WithRestart, OnChange(conf), DependsOn(pkg, conf))
}

// OptsUpload keeps uptime collection paired with uploads.
func (Base) OptsUpload() TaskOptions {
	return TaskOptions{Needs(Base.Uptimed)}
}

// Upload installs the API token, upload client and hourly root cron job.
func (Base) Upload() {
	tokenRef := paths.FrontendSecret("etc/goprecords/triggerfish.token")
	// This host must have its key provisioned before a deployment succeeds.
	MustSecret(tokenRef)
	client, ok := goprecords.Client(tokenRef)
	if !ok {
		Refuse("File", goprecords.TokenFile, fmt.Errorf("triggerfish goprecords token is empty"))
		return
	}
	CronAt("goprecords-upload", "19 * * * *", goprecords.CronCommand("triggerfish"), DependsOn(client))
}
