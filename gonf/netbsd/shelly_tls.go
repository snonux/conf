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
//
// The job runs as the unprivileged account _shellytls (s44 review N1): it
// parses certificates received from the network on hosts that also serve a
// public site. It needs only read access to the world-readable CA file and
// its own state directory.
type ShellyTLS struct{ RequiresRoot }

const (
	shellyTLSProbe    = "/usr/local/sbin/shelly-tls-probe"
	shellyTLSStateDir = "/var/db/shelly-tls-probe"
	shellyTLSUser     = "_shellytls"
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

// Account creates the unprivileged _shellytls account (no login) that runs
// the probe, with its state directory as home.
//
// The home must exist: NetBSD cron chdirs to $HOME before running a job
// and skips the job when that fails (task n54). The first version used
// /var/empty, which NetBSD (unlike OpenBSD and FreeBSD) does not have, so
// every scheduled run was skipped. The state directory is the account's
// only writable place and StateDir creates it in the same plan.
// WithManageHome also points an existing account's passwd home field there
// (usermod -d, which neither moves nor chowns anything).
func (ShellyTLS) Account() {
	User(shellyTLSUser, WithPrimaryGroup(shellyTLSUser),
		WithHome(shellyTLSStateDir), WithManageHome, WithShell("/sbin/nologin"))
}

// OptsStateDir records the account before the directory it owns.
func (ShellyTLS) OptsStateDir() TaskOptions {
	return TaskOptions{Needs(ShellyTLS.Account)}
}

// StateDir creates /var/db/shelly-tls-probe (0700 _shellytls), the
// account's home, for the per-plug last-success records.
//
// The records let Prometheus tell "failing for more than 2h" without
// relying on alert state, which the nightly cluster power-off resets.
// Dir, not EnsureDir: an existing directory (root-owned from the first
// deployment) must converge to the new owner, and EnsureDir skips existing
// directories. Records a former root run left behind are replaced by
// rename, which the directory owner may do.
func (ShellyTLS) StateDir() {
	Dir(shellyTLSStateDir, Perm(0o700, shellyTLSUser+":"+shellyTLSUser))
}

// OptsCron records the script, the account and its state directory before
// the job.
func (ShellyTLS) OptsCron() TaskOptions {
	return TaskOptions{Needs(ShellyTLS.Script, ShellyTLS.Account, ShellyTLS.StateDir)}
}

// Cron runs shelly-tls-probe hourly as _shellytls at the host's minute,
// diagnostics to syslog.
//
// The probe writes its diagnostics to stderr, which goes to syslog (tag
// shelly-tls-probe); stdout is dropped, so cron mails nothing to an unread
// local mailbox (task 2k2). The first deployment ran the job in root's
// crontab; that entry is removed once the unprivileged one exists.
//
// cron chdirs to the account's home before the job (see Account); a manual
// "doas -u _shellytls" run does not, so it cannot show that a scheduled run
// would start. Check /var/log/cron after the host's minute instead.
func (ShellyTLS) Cron() {
	EachHost(func(at ShellyTLSProbeTime) {
		job := Cron("shelly-tls-probe",
			WithCommand(shellyTLSProbe+" 2>&1 >/dev/null | logger -t shelly-tls-probe"),
			WithMinute(at.Minute), WithCronUser(shellyTLSUser))
		NoCron("shelly-tls-probe", DependsOn(job))
	})
}
