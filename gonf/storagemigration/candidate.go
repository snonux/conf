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
	return endpoint(source, "accept")
}

// CompatibleServer retains the old TLS endpoint on the same CARP owner.
// Both services use the existing certificates and client verification policy.
func CompatibleServer(source string) (string, error) {
	if strings.Contains(source, "[nfs-tls-compat]") {
		global, _, _ := strings.Cut(source, "[nfs-tls]")
		_, service, _ := strings.Cut(source, "[nfs-tls-compat]")
		original := global + "[nfs-tls]" + service
		if strings.Count(original, "[") != 1 {
			return "", fmt.Errorf("unexpected compatibility services")
		}
		expected, err := CompatibleServer(original)
		if err != nil || expected != source {
			return "", fmt.Errorf("compatibility TLS policy drift")
		}
		return source, nil
	}
	primary, err := Server(source)
	if err != nil {
		return "", err
	}
	const section = "[nfs-tls]"
	if strings.Count(source, section) != 1 || strings.Count(source, "[") != 1 {
		return "", fmt.Errorf("expected exactly one NFS TLS service")
	}
	_, service, _ := strings.Cut(source, section)
	return strings.TrimRight(primary, "\n") + "\n\n[nfs-tls-compat]" + service, nil
}

// Client preserves the client identity, loopback listener and TLS policy.
func Client(source string) (string, error) {
	return endpoint(source, "connect")
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
	// Replace the complete active table line, never matching a comment.
	lines := strings.SplitAfter(source, "\n")
	for i, line := range lines {
		if strings.TrimSuffix(line, "\n") == table {
			lines[i] = strings.Replace(line, table, prefix+strings.Join(next, " ")+" }", 1)
		}
	}
	return listeners(strings.Join(lines, ""))
}

// CompatibleRelayd retains old ingress listeners using the new backend table.
// Both VIPs must belong to the same vhid on both existing CARP members.
func CompatibleRelayd(source string) (string, error) {
	const marker = "\n\n# Retained until old clients migrate.\n"
	if strings.Contains(source, marker) {
		if strings.Count(source, marker) != 1 {
			return "", fmt.Errorf("duplicate compatibility relay blocks")
		}
		primary, _, _ := strings.Cut(source, marker)
		trailing := len(source) - len(strings.TrimRight(source, "\n"))
		original := strings.ReplaceAll(primary, "10.20.0.", "192.168.1.") + strings.Repeat("\n", trailing)
		expected, err := CompatibleRelayd(original)
		if err != nil || expected != source {
			return "", fmt.Errorf("compatibility relay policy drift")
		}
		return source, nil
	}
	primary, err := Relayd(source)
	if err != nil {
		return "", err
	}
	start := strings.Index(primary, `relay "lan_http" {`)
	if start < 0 || strings.Count(primary, "relay ") != 2 ||
		strings.Count(primary, `relay "lan_https" {`) != 1 {
		return "", fmt.Errorf("expected canonical HTTP and HTTPS relay blocks")
	}
	compat := strings.ReplaceAll(primary[start:], LabVIP, OldVIP)
	compat = strings.ReplaceAll(compat, `"lan_http"`, `"lan_http_compat"`)
	compat = strings.ReplaceAll(compat, `"lan_https"`, `"lan_https_compat"`)
	return strings.TrimRight(primary, "\n") + marker + compat, nil
}

// endpoint requires exactly one complete active directive. It deliberately
// refuses extra services or inline comments rather than guessing their intent.
func endpoint(source, directive string) (string, error) {
	lines := strings.SplitAfter(source, "\n")
	count := 0
	for i, line := range lines {
		active := strings.TrimSpace(line)
		if strings.HasPrefix(active, "#") || strings.HasPrefix(active, ";") {
			continue
		}
		key, value, found := strings.Cut(active, "=")
		if !found || !strings.EqualFold(strings.TrimSpace(key), directive) {
			continue
		}
		count++
		if strings.TrimSpace(value) != OldVIP+":2323" || count != 1 {
			return "", fmt.Errorf("unexpected or duplicate active stunnel %s endpoint", directive)
		}
		lines[i] = strings.Replace(line, OldVIP+":2323", LabVIP+":2323", 1)
	}
	if count != 1 {
		return "", fmt.Errorf("missing active stunnel %s endpoint", directive)
	}
	return strings.Join(lines, ""), nil
}

func listeners(source string) (string, error) {
	lines := strings.SplitAfter(source, "\n")
	seen := make(map[string]bool)
	for i, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "listen" {
			continue
		}
		if len(fields) != 5 || fields[1] != "on" || fields[2] != OldVIP || fields[3] != "port" ||
			(fields[4] != "80" && fields[4] != "443") || seen[fields[4]] {
			return "", fmt.Errorf("unexpected or duplicate active relayd listener")
		}
		seen[fields[4]] = true
		lines[i] = strings.Replace(line, OldVIP, LabVIP, 1)
	}
	if len(seen) != 2 {
		return "", fmt.Errorf("missing active relayd listeners")
	}
	return strings.Join(lines, ""), nil
}
