// Package migrationaddress validates opt-in triggerfish candidate addresses.
// It never changes the active host inventory.
package migrationaddress

import (
	"fmt"
	"net/netip"
)

// Lab maps a retained old LAN address to its reserved lab suffix. Callers
// supply existing typed host inventory; no parallel hostname map is kept.
func Lab(address string) (string, error) {
	ip, err := netip.ParseAddr(address)
	if err != nil || !netip.MustParsePrefix("192.168.1.0/24").Contains(ip) {
		return "", fmt.Errorf("candidate requires an old 192.168.1.0/24 address: %q", address)
	}
	octets := ip.As4()
	if octets[3] == 0 || octets[3] == 255 {
		return "", fmt.Errorf("candidate requires a host address: %q", address)
	}
	return fmt.Sprintf("10.20.0.%d", octets[3]), nil
}
