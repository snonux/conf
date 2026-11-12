package freebsd

import (
	. "github.com/snonux/gonf/api"
)

// Uplink owns the rc.conf line of the f-hosts' physical lab NIC re0, which
// since task 764 carries no address and is a plain member of the bridge
// vm-public (the bridge holds the lab IPv4, the ULA and, on f0/f1, the CARP
// VIP).
//
// Why -txcsum -tso: if_bridge mutes TXCSUM/TSO on every member while a member
// without them (the bhyve guest's tap) is attached, and restores them when it
// leaves. re(4) re-initialises and renegotiates its link on each capability
// change, so every guest start and stop reset re0 for about 4 s: the host and
// its guest lost the lab for that long. netif configures re0 (interface-index
// order) before clone_up's vm-public runs "addm re0", so re0 joins the bridge
// with the capabilities already off and no later tap join or leave changes
// them. While a guest runs the bridge mutes exactly these anyway; with no
// guest the host only loses TX checksum and TSO4 offload, which is
// negligible at 1 Gbit/s. TXCSUM and TSO4 are the only bridge-controlled
// capabilities re0 has. On f0/f1 the CARP boot gate (Carp.BootGate)
// pre-mutes them before netif as well.
type Uplink struct{ RequiresRoot }

// uplinkRcConfValue is ifconfig_re0 after the bridge move. The flags come
// before "up": ifconfig applies its arguments in order, so the capability
// change hits a down NIC and costs no extra re(4) re-init at boot.
const uplinkRcConfValue = "-txcsum -tso up"

// RcConf keeps re0 an address-less bridge member with TXCSUM/TSO off in rc.conf (next boot).
//
// The guard fails the task (with a message) on a host that is not on
// vm-public yet or while a bridge-move transaction is pending, whose journal
// owns the same key (f3s/freebsd-hosts/network-migration/bridge.sh writes
// ifconfig_re0="up" at a move; this task corrects it after the confirm).
// Read at boot; nothing is restarted. While a guest runs, the bridge has
// already muted the same capabilities, so the running re0 needs no change.
func (Uplink) RcConf() {
	moved := Command("sh", List("-c", `test ! -d `+bridgeState+`/pending && grep -q '^ifconfig_vm_public=' `+rcConf+
		` || { echo 'not on vm-public yet, or a bridge move is pending' >&2; exit 1; }`),
		WithName("uplink-moved-guard"))
	File(rcConf, WithShellVar("ifconfig_"+labMember, uplinkRcConfValue),
		RootOwned, WithName("rc-conf-uplink"), DependsOn(moved))
}
