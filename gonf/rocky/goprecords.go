package rocky

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/goprecords"
	"github.com/snonux/conf/gonf/paths"
)

// Goprecords manages uptimed and the hourly upload to goprecords on pi2/pi3.
// Uptimed and the upload client belong together: the timer uploads the
// daemon's records file.
type Goprecords struct {
	RequiresRoot
}

const (
	uptimedConf       = "/etc/uptimed.conf"
	uptimedDropInDir  = "/etc/systemd/system/uptimed.service.d"
	uptimedDropInPath = uptimedDropInDir + "/time-sync.conf"
)

// Uptimed installs the uptimed package, conf (LOG_MAXIMUM_ENTRIES=0), the
// chronyc waitsync drop-in (no RTC on these Pis), and enables the daemon.
//
// Uptimed owns the daemon the upload timer depends on. The drop-in keeps
// uptimed from recording a stale boot time before chronyd has synced; see
// the f3s-workloads goprecords-uptimed skill.
func (Goprecords) Uptimed() {
	pkg := Package("uptimed")
	conf := InstallFile(uptimedConf, paths.GonfAsset("goprecords", "uptimed.conf"),
		RootOwned, DependsOn(pkg))
	EnsureDir(uptimedDropInDir, RootOwned)
	dropIn := InstallFile(uptimedDropInPath, paths.GonfAsset("rocky", "uptimed-time-sync.conf"),
		RootOwned, DependsOn(pkg))
	DaemonReload(OnChange(dropIn))
	Service("uptimed", WithRestart, OnChange(conf, dropIn), DependsOn(pkg, conf, dropIn))
}

// OptsUpload records uptimed first: the client uploads its records file.
func (Goprecords) OptsUpload() TaskOptions {
	return TaskOptions{Needs(Goprecords.Uptimed)}
}

// Upload installs the goprecords upload client, token, env file and hourly
// system timer (oneshot + timer), replacing the hand-made units of the same
// name when present.
func (Goprecords) Upload() {
	EachHost(func(c goprecords.ClientHost) {
		host := c.Host
		client, ok := goprecords.Client(goprecords.PiToken(host))
		if !ok {
			return
		}
		env := File("/etc/goprecords-upload.env",
			WithContent("GOPRECORDS_HOST="+host+"\n"),
			RootOwned, DependsOn(client))
		SystemdTimer("goprecords-upload",
			WithCommand("/usr/bin/env GOPRECORDS_HOST="+host+" "+goprecords.ClientScript),
			WithOnCalendar("hourly"),
			WithOnBootSec("90s"),
			WithPersistent,
			WithDescription("Hourly uptimed upload to goprecords"),
			WithServiceDescription("Upload uptimed stats to goprecords"),
			WithAfter("network-online.target"),
			WithWants("network-online.target"),
			DependsOn(client, env),
		)
	})
}
