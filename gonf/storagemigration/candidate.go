// Package storagemigration renders inactive storage endpoint candidates.
// It never reads credentials or applies network or service state.
package storagemigration

import (
	"fmt"
	"strings"

	"github.com/snonux/conf/gonf/migrationaddress"
)

const (
	// OldVIP remains the active storage endpoint until coordinated acceptance.
	OldVIP = "192.168.1.138"
	// LabVIP is the reserved replacement storage and ingress endpoint.
	LabVIP = "10.20.0.138"
	// Suffix keeps candidates outside service configuration discovery paths.
	Suffix = ".triggerfish-candidate"
)

// Server preserves every server setting except the exact TLS listener.
func Server(source string) (string, error) {
	return replace(source, "accept = "+OldVIP+":2323", "accept = "+LabVIP+":2323", 1)
}

// Client preserves the client identity, loopback listener and TLS policy.
func Client(source string) (string, error) {
	return replace(source, "connect = "+OldVIP+":2323", "connect = "+LabVIP+":2323", 1)
}

// Relayd changes only the two VIP listeners and the existing backend table.
// The current table supplies the addresses; there is no parallel host map.
func Relayd(source string) (string, error) {
	const prefix = "table <k3s_nodes> { "
	var table string
	for _, line := range strings.Split(source, "\n") {
		if strings.HasPrefix(line, prefix) {
			if table != "" {
				return "", fmt.Errorf("multiple k3s backend tables")
			}
			table = line
		}
	}
	if !strings.HasSuffix(table, " }") {
		return "", fmt.Errorf("missing canonical k3s backend table")
	}
	old := strings.Fields(strings.TrimSuffix(strings.TrimPrefix(table, prefix), " }"))
	if len(old) != 3 {
		return "", fmt.Errorf("expected three k3s backends")
	}
	next := make([]string, len(old))
	seen := make(map[string]bool)
	for i, address := range old {
		mapped, err := migrationaddress.Lab(address)
		if err != nil {
			return "", err
		}
		if seen[mapped] {
			return "", fmt.Errorf("duplicate k3s backend")
		}
		seen[mapped] = true
		next[i] = mapped
	}
	result, err := replace(source, table, prefix+strings.Join(next, " ")+" }", 1)
	if err != nil {
		return "", err
	}
	return replace(result, "listen on "+OldVIP+" port ", "listen on "+LabVIP+" port ", 2)
}

func replace(source, old, next string, count int) (string, error) {
	if strings.Count(source, old) != count {
		return "", fmt.Errorf("unexpected storage source; expected %d occurrences of %q", count, old)
	}
	return strings.ReplaceAll(source, old, next), nil
}
