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
	goprecordsSchedule     = "17 * * * *"
	legacyGoprecordsClient = "/usr/pkg/bin/goprecords-upload-client.sh"
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
// (output to syslog) and retires the duplicate legacy root uploader.
// Leaves any former paul-user crontab line alone; root's job is managed.
func (Goprecords) Upload() {
	EachHost(func(c goprecords.ClientHost) {
		host := c.Host
		client, ok := goprecords.Client(goprecords.PiToken(host))
		if !ok {
			return
		}
		cron := CronAt("goprecords-upload", goprecordsSchedule, goprecords.CronCommand(host),
			WithLegacyCommand("env GOPRECORDS_HOST="+host+" "+legacyGoprecordsClient+" >/dev/null 2>&1"),
			DependsOn(client))
		// Keep the old script if adoption skipped it or another job uses it.
		Command("rm", List("-f", "--", legacyGoprecordsClient), DependsOn(cron),
			WithName("retired-goprecords-upload-client"),
			OnlyIf("sh", List("-c", `
[ -e "`+legacyGoprecordsClient+`" ] || [ -L "`+legacyGoprecordsClient+`" ] || exit 1
tab=$(crontab -l) || exit 1
case "$tab" in
  *"`+legacyGoprecordsClient+`"*) exit 1 ;;
esac
exit 0`)))
	})
}
