package netbsd

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// ShellyTLS installs the hourly check of the Shelly plugs' HTTPS
// certificates on pi0/pi1 (task s44). The probe does a real TLS handshake
// with each plug, verifies the presented leaf against the pinned f3s Shelly
// CA and pushes notAfter dates and results to the in-cluster Pushgateway
// (the Local NodePort pushgateway-lan, 30091 on r0-r2). Prometheus alerts on
// them (f3s/prometheus/manifests/shelly-tls-alerts.yaml) and Gogios shows
// the firing alerts like every other Prometheus alert.
//
// The probe trusts exactly the CA file f3sctl uses,
// /usr/local/etc/f3sctl/shelly-ca.pem, so it also notices a missing or
// replaced controller trust anchor. That file belongs to
// Controller.Config, which applies only through the controller endpoint
// lease, so it is deliberately not a Needs prerequisite here: a missing CA
// makes the probe report failure instead of blocking this deployment.
type ShellyTLS struct{ RequiresRoot }

const (
	shellyTLSProbe    = "/usr/local/sbin/shelly-tls-probe"
	shellyTLSStateDir = "/var/db/shelly-tls-probe"
)

// ShellyTLSProbeTime is a Pi's hourly probe minute (host data, see
// gonf/cluster). pi0 and pi1 use different minutes so they never open TLS
// sessions to the same plug at the same moment.
type ShellyTLSProbeTime struct {
	Minute string
}

// Packages installs curl, which pushes the results to the Pushgateway.
func (ShellyTLS) Packages() {
	Package("curl")
}

// OptsScript records curl before the script that needs it.
func (ShellyTLS) OptsScript() TaskOptions {
	return TaskOptions{Needs(ShellyTLS.Packages)}
}

// Script installs /usr/local/sbin/shelly-tls-probe (0755 root:wheel).
func (ShellyTLS) Script() {
	EnsureDir("/usr/local/sbin", RootOwned)
	InstallFile(shellyTLSProbe, paths.GonfAsset("netbsd", "shelly-tls-probe"), RootExec)
}

// StateDir creates /var/db/shelly-tls-probe for the per-plug last-success
// records.
//
// The records let Prometheus tell "failing for more than 2h" without
// relying on alert state, which the nightly cluster power-off resets.
func (ShellyTLS) StateDir() {
	EnsureDir(shellyTLSStateDir, RootPrivate)
}

// OptsCron records the script and its state directory before the job.
func (ShellyTLS) OptsCron() TaskOptions {
	return TaskOptions{Needs(ShellyTLS.Script, ShellyTLS.StateDir)}
}

// Cron runs shelly-tls-probe hourly at the host's minute, diagnostics to
// syslog.
//
// The probe writes its diagnostics to stderr, which goes to syslog (tag
// shelly-tls-probe); stdout is dropped, so cron mails nothing to root's
// unread local mailbox (task 2k2).
func (ShellyTLS) Cron() {
	EachHost(func(at ShellyTLSProbeTime) {
		Cron("shelly-tls-probe",
			WithCommand(shellyTLSProbe+" 2>&1 >/dev/null | logger -t shelly-tls-probe"),
			WithMinute(at.Minute))
	})
}
