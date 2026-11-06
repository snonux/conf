package recovery

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/netip"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
)

const readinessHandler = "/usr/share/udhcpc/default.script"
const readinessLeasePath = "/run/udhcpc.eth0.info"
const readinessHandlerSHA = "03cfb359fc6b1b16df905fa411be527bf0a142b8a23647f0ee16b103268a4797"
const readinessLeaseKeys = "ip siaddr sname boot_file subnet timezone router timesvr namesvr dns logsvr cookiesvr lprsvr hostname bootsize domain swapsvr rootpath ipttl mtu broadcast ntpsrv wins lease dhcptype serverid message tftp bootfile"

// ReadinessFile records identity and provenance, without file contents or clocks.
type ReadinessFile struct {
	Hash                 string
	Mode                 uint32
	UID, GID             uint32
	Links, Inode, Device uint64
	Regular              bool
}

// ReadinessOwner identifies the actual daemon, not the app's cached PID.
type ReadinessOwner struct {
	PID, PPID, Group, Session int
	Birth                     uint64
	State                     byte
	Argv                      []string
	Executable                ReadinessFile
}

// ReadinessProcesses includes all direct children, including zombies.
type ReadinessProcesses struct {
	Owner                         ReadinessOwner
	Candidates, Children, Unknown int
}

// ReadinessLease is emitted only after parsing the complete vendor file schema.
type ReadinessLease struct {
	Complete     bool
	Address      netip.Prefix
	Routers, DNS []netip.Addr
	Server       netip.Addr
	Seconds      uint64
	File         ReadinessFile
}

// ReadinessMount binds the canonical lease to its covering current mount.
type ReadinessMount struct {
	ID         int
	Path, Type string
	Device     uint64
}

// ReadinessRoute describes an observed main-table default, never a requested edit.
type ReadinessRoute struct {
	Gateway   netip.Addr
	Interface string
}

// ParentReadinessSnapshot is readonly evidence; it grants no canary authorization.
// Begin/End bracket collection without locking or stopping the vendor owner.
type ParentReadinessSnapshot struct {
	Boot, CanonicalLease string
	Parent               Link
	Begin, End           ReadinessProcesses
	Handler, Busybox     ReadinessFile
	HooksKnown           bool
	ExecutableHooks      int
	Lease                ReadinessLease
	Mount                ReadinessMount
	Defaults             []ReadinessRoute
	DNS                  []netip.Addr
}

// ValidateParentReadiness checks two stable post-vendor observations. It neither
// claims a boot lease nor proves future watcher timing or freezes natural renews.
func ValidateParentReadiness(boot, factoryMAC string, first, second ParentReadinessSnapshot) error {
	mac, err := net.ParseMAC(factoryMAC)
	if boot == "" || err != nil || len(mac) != 6 || mac[0]&1 != 0 {
		return errors.New("invalid parent readiness expectation")
	}
	for _, s := range []ParentReadinessSnapshot{first, second} {
		if err := validateReadinessSnapshot(boot, mac.String(), s); err != nil {
			return err
		}
	}
	if !reflect.DeepEqual(first, second) {
		return errors.New("parent readiness observations changed")
	}
	return nil
}

func validateReadinessSnapshot(boot, mac string, s ParentReadinessSnapshot) error {
	if s.Boot != boot || !s.Parent.Exists || s.Parent.Index <= 0 || !s.Parent.Up || s.Parent.MAC != mac {
		return errors.New("parent boot, identity or UP not ready")
	}
	if !s.HooksKnown || s.ExecutableHooks != 0 || !readyFile(s.Handler, true) || s.Handler.Hash != readinessHandlerSHA || !readyFile(s.Busybox, true) {
		return errors.New("DHCP handler provenance not ready")
	}
	for _, p := range []ReadinessProcesses{s.Begin, s.End} {
		if err := validateReadinessOwner(p, s.Busybox); err != nil {
			return err
		}
	}
	if !reflect.DeepEqual(s.Begin, s.End) {
		return errors.New("DHCP owner changed during collection")
	}
	if !s.Lease.Complete || !readyFile(s.Lease.File, false) || s.Lease.Seconds == 0 || !s.Lease.Address.IsValid() || !readyIPv4(s.Lease.Address.Addr()) || s.Lease.Address.Bits() <= 0 || !readyIPv4(s.Lease.Server) || s.Lease.Seconds > 0xffffffff {
		return errors.New("complete applied DHCP lease required")
	}
	if !filepath.IsAbs(s.CanonicalLease) || filepath.Clean(s.CanonicalLease) != s.CanonicalLease || filepath.Base(s.CanonicalLease) != "udhcpc.eth0.info" || s.Mount.ID <= 0 || s.Mount.Type != "tmpfs" || !mountContains(s.Mount.Path, s.CanonicalLease) || s.Mount.Device != s.Lease.File.Device {
		return errors.New("current-boot tmpfs lease provenance required")
	}
	applied := false
	for _, address := range s.Parent.Addresses {
		if address == s.Lease.Address.String() {
			applied = true
		}
	}
	if !applied || !uniqueReadyAddresses(s.Lease.Routers) || !uniqueReadyAddresses(s.Lease.DNS) || !sameAddresses(s.Lease.DNS, s.DNS) {
		return errors.New("lease address or resolver state not applied")
	}
	if len(s.Defaults) != len(s.Lease.Routers) {
		return errors.New("lease default routes not applied")
	}
	for _, gateway := range s.Lease.Routers {
		found := false
		for _, route := range s.Defaults {
			if gateway.Is4() && route.Gateway == gateway && route.Interface == "eth0" {
				found = true
			}
		}
		if !found {
			return errors.New("lease default route not applied")
		}
	}
	return nil
}

func validateReadinessOwner(p ReadinessProcesses, busybox ReadinessFile) error {
	o := p.Owner
	if p.Candidates != 1 || p.Children != 0 || p.Unknown != 0 || o.PID <= 1 || o.Birth == 0 || o.PPID != 1 || o.Session != o.PID || o.Group != o.PID || (o.State != 'S' && o.State != 'R') || !readyFile(o.Executable, true) || o.Executable.Hash != busybox.Hash {
		return errors.New("stable completed DHCP daemon required")
	}
	return readinessArgv(o.Argv)
}

func readyFile(f ReadinessFile, executable bool) bool {
	decoded, err := hex.DecodeString(f.Hash)
	if err != nil || len(decoded) != sha256.Size || !f.Regular || f.UID != 0 || f.GID != 0 || f.Links == 0 || f.Inode == 0 || f.Device == 0 {
		return false
	}
	if executable {
		return f.Mode&0111 != 0 && f.Mode&^uint32(0755) == 0
	}
	return f.Links == 1 && (f.Mode == 0600 || f.Mode == 0644)
}

// The implicit handler is admitted only for the pinned BusyBox 1.36.1 vendor
// build: its CONFIG_UDHCPC_DEFAULT_SCRIPT is readinessHandler. The executable
// comparison observes the installed BusyBox; it is not binary attestation.
func readinessArgv(args []string) error {
	if len(args) < 2 || args[0] != "udhcpc" {
		return errors.New("unreviewed DHCP argv")
	}
	seen := map[string]bool{}
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if len(arg) < 2 || arg[0] != '-' || !strings.Contains("ixs", arg[1:2]) {
			return errors.New("unreviewed DHCP option")
		}
		flag, value := arg[1:2], arg[2:]
		if seen[flag] {
			return errors.New("duplicate DHCP option")
		}
		seen[flag] = true
		if value == "" {
			i++
			if i >= len(args) {
				return errors.New("missing DHCP option value")
			}
			value = args[i]
		}
		switch flag {
		case "i":
			if value != "eth0" {
				return errors.New("wrong DHCP interface")
			}
		case "s":
			if value != readinessHandler {
				return errors.New("custom DHCP script")
			}
		case "x":
			host, ok := strings.CutPrefix(value, "hostname:")
			if !ok || len(host) < 1 || len(host) > 63 || !hostnameLabel(host) {
				return errors.New("unreviewed DHCP hostname")
			}
		}
	}
	if !seen["i"] {
		return errors.New("explicit DHCP interface required")
	}
	return nil
}

func hostnameLabel(s string) bool {
	for i, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			continue
		}
		if r != '-' || i == 0 || i == len(s)-1 {
			return false
		}
	}
	return true
}

func parseReadinessLease(data []byte, file ReadinessFile) (ReadinessLease, error) {
	result := ReadinessLease{File: file}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		return result, errors.New("truncated DHCP lease")
	}
	expected := map[string]bool{}
	for _, key := range strings.Fields(readinessLeaseKeys) {
		expected[key] = true
	}
	values := map[string]string{}
	last := ""
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		_, duplicate := values[key]
		if !ok || !expected[key] || duplicate {
			return result, errors.New("invalid DHCP lease schema")
		}
		values[key], last = value, key
	}
	if len(values) != len(expected) || last != "bootfile" {
		return result, errors.New("incomplete DHCP lease schema")
	}
	ip, err := netip.ParseAddr(values["ip"])
	mask := net.ParseIP(values["subnet"]).To4()
	if err != nil || !ip.Is4() || ip.IsUnspecified() || ip.IsMulticast() || mask == nil {
		return result, errors.New("invalid DHCP IPv4 lease")
	}
	bits, width := net.IPMask(mask).Size()
	if width != 32 || bits == 0 {
		return result, errors.New("invalid DHCP netmask")
	}
	result.Address = netip.PrefixFrom(ip, bits)
	result.Server, err = netip.ParseAddr(values["serverid"])
	if err != nil || !result.Server.Is4() || result.Server.IsUnspecified() || result.Server.IsMulticast() {
		return result, errors.New("invalid DHCP server")
	}
	result.Seconds, err = strconv.ParseUint(values["lease"], 10, 32)
	if err != nil || result.Seconds == 0 {
		return result, errors.New("invalid DHCP duration")
	}
	result.Routers, err = parseReadyAddresses(values["router"])
	if err != nil {
		return result, err
	}
	result.DNS, err = parseReadyAddresses(values["dns"])
	if err != nil {
		return result, err
	}
	result.Complete = true
	return result, nil
}

func parseReadyAddresses(value string) ([]netip.Addr, error) {
	var result []netip.Addr
	for _, text := range strings.Fields(value) {
		address, err := netip.ParseAddr(text)
		if err != nil || !address.Is4() || address.IsUnspecified() || address.IsMulticast() {
			return nil, errors.New("invalid DHCP address list")
		}
		for _, prior := range result {
			if prior == address {
				return nil, errors.New("duplicate DHCP address")
			}
		}
		result = append(result, address)
	}
	if len(result) == 0 {
		return nil, errors.New("empty DHCP address list")
	}
	return result, nil
}

func sameAddresses(a, b []netip.Addr) bool {
	if len(a) != len(b) {
		return false
	}
	for _, value := range a {
		count := 0
		for _, other := range b {
			if value == other {
				count++
			}
		}
		if count != 1 {
			return false
		}
	}
	return true
}

func mountContains(mount, path string) bool {
	return filepath.IsAbs(mount) && filepath.Clean(mount) == mount && (mount == "/" || path == mount || strings.HasPrefix(path, mount+"/"))
}

func readyIPv4(address netip.Addr) bool {
	return address.Is4() && !address.IsUnspecified() && !address.IsMulticast()
}

func uniqueReadyAddresses(addresses []netip.Addr) bool {
	if len(addresses) == 0 {
		return false
	}
	seen := make(map[netip.Addr]bool, len(addresses))
	for _, address := range addresses {
		if !readyIPv4(address) || seen[address] {
			return false
		}
		seen[address] = true
	}
	return true
}
