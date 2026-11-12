package freebsd

// The f-hosts' lab network layout since task 764 (2026-11-12): the physical
// NIC re0 is an address-less member of the vm-bhyve bridge vm-public, which
// carries the lab IPv4 10.20.0.13N/24, the lab ULA fd02:ca0b:f1cb:20::13N/64
// and, on f0/f1, the CARP VIP. rc.conf alone builds it at boot
// (cloned_interfaces bridge0 renamed vm-public, ifconfig_vm_public*,
// ifconfig_re0 from Uplink.RcConf, defaultrouter 10.20.0.1); vm-bhyve adopts
// the bridge as the manual switch "public". The one-shot helpers that moved
// the addresses there were removed in task d74; IQ
// topics/infra/f3s/network/fhost-bridge.md keeps the why and the history.
const (
	// LabBridge is the bridge that carries the host addresses;
	// CarpNode.Interface names it on f0/f1.
	LabBridge = "vm-public"
	// labMember is the physical NIC: a plain member of LabBridge, the
	// Wake-on-LAN NIC, and the wire the CARP boot gate listens on.
	labMember = "re0"
)
