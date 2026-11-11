package freebsd

import (
	"fmt"
	"net/netip"

	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

// BridgeMigration moves an f-host's lab IPv4, lab ULA and (on the storage
// pair) the CARP VIP from the physical NIC re0 onto the vm-bhyve bridge
// vm-public, of which re0 stays a plain member (task 764). With the
// addresses on the member a bhyve guest cannot reach its own host over
// native IPv6: its neighbour solicitation arrives on the guest's tap and is
// dropped there, because ND answers only for addresses of the receiving
// interface. FreeBSD 15 also deprecates addresses on bridge members.
//
// The helper (f3s/freebsd-hosts/network-migration/bridge.sh, BRIDGE.md)
// journals the scoped rc.conf and vm-bhyve system.conf keys, moves the
// running addresses, and restores both when the move is not confirmed in
// time (root cron), at the next boot before netif, or on request. Every
// method is Operational: the routine freebsd aggregate never moves
// addresses. Prepare refuses while a transaction is pending, so tooling is
// never replaced under an armed journal.
type BridgeMigration struct{ RequiresRoot }

const (
	bridgeHelper = "/usr/local/sbin/gonf-bridge-migration"
	bridgeState  = "/var/db/gonf-bridge-migration"
	// LabBridge is the vm-bhyve bridge of switch "public" that carries the
	// host addresses after the move; CarpNode.Interface names it then.
	LabBridge = "vm-public"
	// labMember is the physical NIC that stays a bridge member (and the
	// Wake-on-LAN NIC).
	labMember = "re0"
	// labGateway is the lab router (triggerfish igc1), the hosts' default.
	labGateway = "10.20.0.1"
)

func (BridgeMigration) OptsPrepare() TaskOptions   { return TaskOptions{Operational()} }
func (BridgeMigration) OptsMove() TaskOptions      { return TaskOptions{Operational()} }
func (BridgeMigration) OptsBootTrial() TaskOptions { return TaskOptions{Operational()} }
func (BridgeMigration) OptsConfirm() TaskOptions   { return TaskOptions{Operational()} }
func (BridgeMigration) OptsRollback() TaskOptions  { return TaskOptions{Operational()} }
func (BridgeMigration) OptsStatus() TaskOptions    { return TaskOptions{Operational()} }

// Prepare installs the dormant bridge-move helper, its boot hook and watchdog.
// The host's lab addresses come from its MigrationHost (the reviewed
// inventory of the earlier address migration); the move itself needs a
// root-only gate file, see BRIDGE.md.
func (BridgeMigration) Prepare() {
	EachHost(func(h MigrationHost) {
		if err := bridgeHostValid(h); err != nil {
			Refuse("BridgeMigration", bridgeHelper, err)
			return
		}
		idle := Command("test", List("!", "-d", bridgeState+"/pending"))
		EnsureDir("/usr/local/sbin", RootOwned)
		EnsureDir("/usr/local/etc", RootOwned)
		EnsureDir(bridgeState, RootPrivate)
		conf := File("/usr/local/etc/gonf-bridge-migration.conf", WithContent(fmt.Sprintf(
			"member='%s'\nbridge='%s'\nswitch='public'\nlab='%s'\nula='%s'\ngateway='%s'\n",
			labMember, LabBridge, h.LabIPv4, h.ULA, labGateway)), RootPrivate, DependsOn(idle))
		helper := InstallFile(bridgeHelper, paths.FHostAsset("network-migration/bridge.sh"), RootExec, DependsOn(conf))
		// System rc.d, as for gonf_network_rollback: it must run before netif.
		InstallFile("/etc/rc.d/gonf_bridge_rollback", paths.FHostAsset("network-migration/rc-bridge-rollback"), RootExec, DependsOn(helper))
		CronAt("gonf-bridge-rollback", "* * * * *", bridgeHelper+" watchdog", DependsOn(helper))
	})
}

// bridgeHostValid checks the inventory row the helper configuration is
// rendered from: an f-host on re0 with a lab IPv4 and a private ULA.
func bridgeHostValid(h MigrationHost) error {
	if h.Standalone || h.Interface != labMember {
		return fmt.Errorf("bridge move is for f-hosts on %s only", labMember)
	}
	lab, err := netip.ParseAddr(h.LabIPv4)
	if err != nil || !lab.Is4() || !netip.MustParsePrefix(labGateway+"/24").Contains(lab) {
		return fmt.Errorf("lab IPv4 %q is not in %s/24", h.LabIPv4, labGateway)
	}
	ula, err := netip.ParseAddr(h.ULA)
	if err != nil || !ula.Is6() || !ula.IsPrivate() {
		return fmt.Errorf("lab ULA %q is not a private IPv6 address", h.ULA)
	}
	return nil
}

// Move arms a 900-second journal and moves the addresses onto the bridge.
func (BridgeMigration) Move() { Sh(bridgeHelper + " move 900") }

// BootTrial lets the next boot keep the pending move once, within 1200 s.
func (BridgeMigration) BootTrial() { Sh(bridgeHelper + " boot-trial 1200") }

// Confirm accepts a pending move after separate fresh access and service checks.
func (BridgeMigration) Confirm() { Sh(bridgeHelper + " confirm") }

// Rollback restores a pending move to re0 without restarting netif or guests.
func (BridgeMigration) Rollback() { Sh(bridgeHelper + " rollback") }

// Status reports a pending move and its deadline.
func (BridgeMigration) Status() { Sh(bridgeHelper + " status") }
