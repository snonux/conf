package freebsd

import (
	"strconv"

	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"

	"github.com/snonux/conf/gonf/paths"
)

// Apcupsd manages apcupsd on the f-hosts: f0 is wired to the APC BX750MI over
// USB and serves its status on the LAN (NIS, port 3551); f1-f3 are network
// clients of f0. Until 2026-09-25 the four apcupsd.conf files were hand-edited
// copies of the sample (see the f3s skill, references/ups-power.md).
//
// Findings behind this recipe (audit item 12, f3s/docs/audit-2026-09-25.md):
//   - The ~900 "Communications with UPS lost/restored" mails on f2 (similar
//     counts on f1 and f3) were no network flapping: every lost/restored pair
//     lines up with f0 going down or coming up (nightly power-off, morning
//     power-on with the clients polling before f0's apcupsd is up, daytime f0
//     reboots), and a client that stays up while f0 is off repeats "lost"
//     every 10 minutes. The clients' commfailure/commok scripts therefore stop
//     mailing (ClientEvents); the events stay in syslog and apcupsd.events.
//   - f0's NIS server listened on 0.0.0.0 (all interfaces, including wg0), and
//     every client ran a NIS server on 0.0.0.0 that nothing queries. f0 now
//     binds its LAN address and the clients bind loopback for their own
//     apcaccess.
//   - The clients reached f0 by name; the name comes from /etc/hosts, but the
//     IP removes the resolver from the path entirely.
//
// Register with OnCluster(cluster.NameFreeBSD); every f-host carries a UPS.
type Apcupsd struct {
	RequiresRoot
}

// UPS is a FreeBSD host's apcupsd role (host data, see gonf/cluster).
type UPS struct {
	// Server is the NIS server ("ip:port") a network client polls for the UPS
	// status. It is empty on the host the UPS is wired to over USB.
	Server string
	// NISIP is the address this host's own NIS server binds: the LAN address
	// on the USB host (the clients poll it), loopback on a client (only its
	// local apcaccess reads it).
	NISIP string
}

// Shutdown thresholds per role. The clients shut down first (more battery and
// runtime left) so they still receive f0's UPS status while they go down; f0,
// which owns the USB link, goes last. Deliberate, see ups-power.md.
const (
	usbBatteryLevel = 5
	usbMinutes      = 3
	netBatteryLevel = 10
	netMinutes      = 6
)

const (
	apcupsdDir  = "/usr/local/etc/apcupsd"
	apcupsdConf = apcupsdDir + "/apcupsd.conf"
)

// apcupsdConfData is apcupsd.conf.tmpl's root. It stays JSON-compatible
// because WithTemplateData serializes it with the plan.
type apcupsdConfData struct {
	Cable        string
	Type         string
	Device       string
	NISIP        string
	BatteryLevel string
	Minutes      string
}

// confData maps a host's UPS role onto the template fields: an empty Server
// is the USB host (DEVICE empty = autodetect the USB UPS), anything else is a
// network client of that server.
func confData(u UPS) apcupsdConfData {
	if u.Server == "" {
		return apcupsdConfData{
			Cable: "usb", Type: "usb", Device: "", NISIP: u.NISIP,
			BatteryLevel: strconv.Itoa(usbBatteryLevel), Minutes: strconv.Itoa(usbMinutes),
		}
	}
	return apcupsdConfData{
		Cable: "ether", Type: "net", Device: u.Server, NISIP: u.NISIP,
		BatteryLevel: strconv.Itoa(netBatteryLevel), Minutes: strconv.Itoa(netMinutes),
	}
}

// DescClientEvents returns the description for the quiet client scripts.
func (Apcupsd) DescClientEvents() string {
	return "apcupsd net clients: commfailure/commok log only, no root mail (f0 keeps the stock scripts)"
}

// ClientEvents replaces commfailure and commok on the network clients with a
// script that exits 99 (no mail, no wall); see the asset's header for why.
// The USB host keeps the stock scripts: there a lost link is a real fault.
func (Apcupsd) ClientEvents() {
	EachHost(func(u UPS) {
		if u.Server == "" {
			return
		}
		src := paths.FHostAsset("apcupsd/net-client-event")
		InstallFile(apcupsdDir+"/commfailure", src, Perm(0o744, Root))
		InstallFile(apcupsdDir+"/commok", src, Perm(0o744, Root))
	})
}

// DescConfig returns the description for apcupsd.conf.
func (Apcupsd) DescConfig() string {
	return "Render " + apcupsdConf + " per host (f0 USB + LAN NIS, f1-f3 net clients) and restart apcupsd on change"
}

// OptsConfig installs the quiet client scripts first, so the restart of a
// client (which briefly loses f0) does not mail.
func (Apcupsd) OptsConfig() TaskOptions {
	return TaskOptions{Privileged(), Needs("client_events")}
}

// Config installs the package, renders each host's apcupsd.conf and restarts
// apcupsd only when the file changed; the service converges to enabled and
// running on every apply.
func (Apcupsd) Config() {
	EachHost(func(u UPS) {
		pkg := Package("apcupsd")
		config := InstallFile(apcupsdConf, paths.FHostAsset("apcupsd/apcupsd.conf.tmpl"),
			Perm(0o644, Root), WithTemplateData(confData(u)), DependsOn(pkg))
		Service("apcupsd", WithRestart, OnChange(config))
	})
}
