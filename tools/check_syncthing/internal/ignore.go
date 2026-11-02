package internal

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// noLimit marks an ignore rule limit that was not set.
const noLimit int64 = -1

// IgnoreRule accepts an out-of-sync folder or a device as known. Folder
// matches the folder ID or label exactly. MaxItems/MaxBytes, when set,
// bound the accepted needTotalItems/needBytes: a folder that falls further
// behind than the recorded case alerts again. A device rule (Device set,
// Folder empty) keeps that device out of the freshness checks.
type IgnoreRule struct {
	Folder   string
	Device   string
	MaxItems int64
	MaxBytes int64
}

// LoadIgnoreFile reads ignore rules from path; an empty path means none.
func LoadIgnoreFile(path string) ([]IgnoreRule, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open ignore file: %w", err)
	}
	defer func() { _ = f.Close() }()
	rules, err := ParseIgnoreRules(f)
	if err != nil {
		return nil, fmt.Errorf("ignore file %s: %w", path, err)
	}
	return rules, nil
}

// ParseIgnoreRules parses one rule per line:
//
//	<folder ID or label>[; max-items=<n>][; max-bytes=<n>]
//	device: <device ID or name>
//
// Blank lines and lines starting with # are skipped.
func ParseIgnoreRules(r io.Reader) ([]IgnoreRule, error) {
	var rules []IgnoreRule
	scanner := bufio.NewScanner(r)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		rule, err := parseIgnoreLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
		rules = append(rules, rule)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read rules: %w", err)
	}
	return rules, nil
}

// Matches reports whether the rule covers folder in its current status.
func (r IgnoreRule) Matches(folder Folder, status FolderStatus) bool {
	if r.Folder == "" || (r.Folder != folder.ID && r.Folder != folder.Label) {
		return false
	}
	if r.MaxItems != noLimit && status.NeedTotalItems > r.MaxItems {
		return false
	}
	return r.MaxBytes == noLimit || status.NeedBytes <= r.MaxBytes
}

// MatchesDevice reports whether the rule ignores device.
func (r IgnoreRule) MatchesDevice(device Device) bool {
	return r.Device != "" && (r.Device == device.ID || r.Device == device.Name)
}

func parseIgnoreLine(line string) (IgnoreRule, error) {
	if rest, ok := strings.CutPrefix(line, "device:"); ok {
		rule := IgnoreRule{Device: strings.TrimSpace(rest), MaxItems: noLimit, MaxBytes: noLimit}
		if rule.Device == "" || strings.Contains(rule.Device, ";") {
			return rule, fmt.Errorf("device rule needs one device ID or name and no conditions")
		}
		return rule, nil
	}
	parts := strings.Split(line, ";")
	rule := IgnoreRule{Folder: strings.TrimSpace(parts[0]), MaxItems: noLimit, MaxBytes: noLimit}
	if rule.Folder == "" {
		return rule, fmt.Errorf("missing folder ID or label")
	}
	for _, part := range parts[1:] {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			return rule, fmt.Errorf("condition %q is not key=value", strings.TrimSpace(part))
		}
		n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil || n < 0 {
			return rule, fmt.Errorf("condition %q needs a non-negative integer", key)
		}
		switch strings.TrimSpace(key) {
		case "max-items":
			rule.MaxItems = n
		case "max-bytes":
			rule.MaxBytes = n
		default:
			return rule, fmt.Errorf("unknown condition %q", key)
		}
	}
	return rule, nil
}
