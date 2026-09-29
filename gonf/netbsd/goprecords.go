package netbsd

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/goprecords"
	"github.com/snonux/conf/gonf/paths"
)

// Goprecords manages uptimed and the hourly upload to goprecords on pi0/pi1.
// Uptimed and the upload client belong together: the cron job uploads the
// daemon's records file.
type Goprecords struct {
	RequiresRoot
}

const (
	uptimedConf = "/etc/uptimed.conf"
	uptimedRcD  = "/etc/rc.d/uptimed"
	// Same minute the hand-made paul crontab used; now root's crontab so the
	// root-owned /etc/goprecords-upload.token is readable.
	goprecordsSchedule = "17 * * * *"
)

// Uptimed installs /etc/uptimed.conf (LOG_MAXIMUM_ENTRIES=0), the custom
// rc.d script, and enables the daemon.
//
// Uptimed does not install the binary: no aarch64 pkgsrc package exists, so
// /usr/pkg/sbin/uptimed stays the hand-built install from the NetBSD Pi
// bootstrap (f3s-raspberry-pi skill). This task only converges conf, rc.d
// and the service enable.
func (Goprecords) Uptimed() {
	conf := InstallFile(uptimedConf, paths.GonfAsset("goprecords", "uptimed.conf"), RootOwned)
	rcd := InstallFile(uptimedRcD, paths.GonfAsset("netbsd", "uptimed.rc"), RootExec)
	Service("uptimed", WithRestart, OnChange(conf), DependsOn(conf, rcd))
}

// OptsUpload records uptimed first: the client uploads its records file.
func (Goprecords) OptsUpload() TaskOptions {
	return TaskOptions{Needs(Goprecords.Uptimed)}
}

// Upload installs the goprecords upload client, token and hourly root cron
// (output to syslog). Leaves any former paul-user crontab line alone; root's
// job is the managed one.
func (Goprecords) Upload() {
	EachHost(func(c goprecords.ClientHost) {
		host := c.Host
		client, ok := goprecords.Client(goprecords.PiToken(host))
		if !ok {
			return
		}
		CronAt("goprecords-upload", goprecordsSchedule, goprecords.CronCommand(host),
			DependsOn(client))
	})
}
