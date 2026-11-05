package recovery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Profile names only the helper-owned link and a narrow controller return path.
// Cabling is a root-reviewed identifier, not inferred from reachable gateway IPs.
type Profile struct {
	Device     string `json:"device"`
	Cabling    string `json:"cabling"`
	Parent     string `json:"parent"`
	Link       string `json:"link"`
	MAC        string `json:"mac"`
	Address    string `json:"address"`
	Gateway    string `json:"gateway"`
	Controller string `json:"controller"`
	Table      int    `json:"table"`
	Priority   int    `json:"priority"`
}

// Link is the exact observable identity needed for safe link ownership checks.
type Link struct {
	Exists       bool
	Index        int
	Parent       string
	MAC          string
	Kind         string
	Up           bool
	Addresses    []string
	IPv6Disabled bool
}

// Runner implements only fixed BusyBox argv, sysfs writes for the owned child,
// and typed link/module observations. It never accepts a shell fragment.
type Runner interface {
	Run(...string) error
	Output(...string) (string, error)
	Link(string) (Link, error)
	Module() (bool, error)
	DisableIPv6(string) error
}

// Owned is durable intent plus observed ownership, separate from vendor state.
type Owned struct {
	Hash   string `json:"profile_hash"`
	Boot   string `json:"boot"`
	Module bool   `json:"module_owned"`
	Link   bool   `json:"link_intent"`
	Index  int    `json:"ifindex"`
	Routes int    `json:"route_intents"`
	Rule   bool   `json:"rule_intent"`
}

// Adapter operates only under the same separate Store lock as transaction commands.
type Adapter struct {
	Store   Store
	Boot    string
	Profile Profile
	Runner  Runner
	Now     func() (int64, error)
}

// Receipt is installed privately by root after native proof, never by the helper.
type Receipt struct {
	Hash            string `json:"profile_hash"`
	Device          string `json:"device"`
	Cabling         string `json:"cabling"`
	FixedPath       bool   `json:"fresh_authenticated_fixed_path"`
	Boot            bool   `json:"boot_recreation"`
	Renew           bool   `json:"dhcp_renew_rebind"`
	Owner           bool   `json:"owner_reapply"`
	Services        bool   `json:"usb_video_media_auth"`
	TransactionHook bool   `json:"full_transaction_hook_installed"`
}

// CanaryAuthorization permits additive changes only on this boot until Deadline.
type CanaryAuthorization struct {
	Hash          string `json:"profile_hash"`
	Generation    string `json:"generation"`
	Boot          string `json:"boot"`
	Deadline      int64  `json:"deadline_boottime_ns"`
	BootAllowance bool   `json:"boot_allowance"`
	Confirmed     bool   `json:"confirmed"`
	Expired       bool   `json:"expired"`
}

// Validate rejects unrelated parent/default/global-policy operations by construction.
func (p Profile) Validate() error {
	address, err := netip.ParsePrefix(p.Address)
	if err != nil || !address.Addr().Is4() || address.Bits() != 24 {
		return errors.New("management address must be IPv4 /24")
	}
	octets := address.Addr().As4()
	if octets[3] == 0 || octets[3] == 255 {
		return errors.New("management address must be a usable host")
	}
	gateway, err := netip.ParseAddr(p.Gateway)
	if err != nil || !gateway.Is4() || !address.Contains(gateway) || gateway == address.Addr() {
		return errors.New("management gateway must be distinct and on-link")
	}
	controller, err := netip.ParsePrefix(p.Controller)
	if err != nil || !controller.Addr().Is4() || controller.Bits() != 32 {
		return errors.New("controller must be IPv4 /32")
	}
	mac, err := net.ParseMAC(p.MAC)
	if err != nil || len(mac) != 6 || mac[0]&3 != 2 {
		return errors.New("management MAC must be locally administered unicast")
	}
	// Shipped BusyBox resolves table IDs through a fixed 0..1023 name table.
	// Rule priority has a separate uint32 parser; retain its reviewed range.
	if p.Table < 1000 || p.Table > 1023 {
		return errors.New("management table must be in reviewed BusyBox range 1000..1023")
	}
	if p.Parent != "eth0" || p.Link != "jkrecover" || p.Priority < 1000 || p.Priority > 32000 || p.Device == "" {
		return errors.New("invalid owned management identifiers")
	}
	// This version deliberately supports only the reviewed current shared-wire profile.
	// A future igc1 profile requires an explicit reviewed implementation change.
	if p.Cabling != "shared-igc0" || address.Masked().String() != "192.168.11.0/24" || p.Gateway != "192.168.11.2" || p.Controller != "10.99.0.2/32" {
		return errors.New("unsupported current-cabling profile")
	}
	return nil
}

// Hash pins every profile field into receipts and authorizations.
func (p Profile) Hash() string {
	data, _ := json.Marshal(p)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Accepted verifies external native acceptance and exact currently running state.
func (a Adapter) Accepted() error {
	if err := a.Profile.Validate(); err != nil {
		return err
	}
	var receipt Receipt
	if err := readPrivate(filepath.Join(a.Store.Dir, "management-accepted.json"), &receipt); err != nil {
		return errors.New("management acceptance receipt missing or invalid")
	}
	if receipt.Hash != a.Profile.Hash() || receipt.Device != a.Profile.Device || receipt.Cabling != a.Profile.Cabling || !receipt.FixedPath || !receipt.Boot || !receipt.Renew || !receipt.Owner || !receipt.Services || !receipt.TransactionHook {
		return errors.New("management native acceptance incomplete")
	}
	if err := a.noPendingCanary(); err != nil {
		return err
	}
	return a.Probe()
}

// EnsureAccepted recreates the accepted owned path during pre-app boot only.
func (a Adapter) EnsureAccepted() error {
	var receipt Receipt
	if err := readPrivate(filepath.Join(a.Store.Dir, "management-accepted.json"), &receipt); err != nil {
		return errors.New("management acceptance required")
	}
	if receipt.Hash != a.Profile.Hash() || receipt.Device != a.Profile.Device || receipt.Cabling != a.Profile.Cabling || !receipt.FixedPath || !receipt.Boot || !receipt.Renew || !receipt.Owner || !receipt.Services || !receipt.TransactionHook {
		return errors.New("management acceptance incomplete")
	}
	if err := a.noPendingCanary(); err != nil {
		return err
	}
	if err := a.Ensure(); err != nil {
		return err
	}
	return a.Probe()
}

// Bootstrap verifies a root-private short-lived authorization before any mutation.
func (a Adapter) Bootstrap(boot string, now int64) error {
	var auth CanaryAuthorization
	if err := readPrivate(filepath.Join(a.Store.Dir, "canary-active.json"), &auth); err != nil {
		return errors.New("canary authorization required")
	}
	if auth.Confirmed || auth.Expired || auth.Hash != a.Profile.Hash() || auth.Boot != boot || auth.Deadline <= now || auth.Deadline-now > WindowNS {
		return errors.New("canary authorization stale or exceeds bound")
	}
	return a.Ensure()
}

// Ensure creates only a missing owned link, routes and rule; it never edits eth0.
func (a Adapter) Ensure() error {
	if err := a.Profile.Validate(); err != nil {
		return err
	}
	own, err := a.load()
	if err != nil {
		return err
	}
	link, err := a.Runner.Link(a.Profile.Link)
	if err != nil {
		return err
	}
	if link.Exists {
		if own.Boot != a.Boot || !own.Link || !a.matches(link, own.Index) {
			return errors.New("management link is foreign")
		}
		return a.Probe()
	}
	// A reboot can discard the link/module/routes while durable ownership survives.
	// Reject any orphan/foreign table or rule before recreating the link.
	routes, rules, err := a.policy()
	if err != nil {
		return err
	}
	if len(routes) != 0 || len(rules) != 0 {
		return errors.New("management policy identifiers already occupied")
	}
	priorModule := own.Module && own.Boot == a.Boot
	own = Owned{Hash: a.Profile.Hash(), Boot: a.Boot, Module: priorModule}
	loaded, err := a.Runner.Module()
	if err != nil {
		return err
	}
	if !loaded {
		own.Module = true
		if err := a.save(own); err != nil {
			return err
		}
		if err := a.Runner.Run("insmod", "/oem/usr/ko/macvlan.ko"); err != nil {
			return errors.New("load shipped macvlan module; cleanup required")
		}
	}
	own.Link = true
	if err := a.save(own); err != nil {
		return err
	}
	p := a.Profile
	if err := a.Runner.Run("ip", "link", "add", "link", p.Parent, "name", p.Link, "address", p.MAC, "type", "macvlan"); err != nil {
		return errors.New("create owned link; cleanup required")
	}
	link, err = a.Runner.Link(p.Link)
	if err != nil {
		return err
	}
	if !a.matches(link, 0) || link.Up || len(link.Addresses) != 0 {
		return errors.New("new link ownership mismatch")
	}
	own.Index = link.Index
	if err := a.save(own); err != nil {
		return err
	}
	if err := a.Runner.DisableIPv6(p.Link); err != nil {
		return err
	}
	if err := a.Runner.Run("ip", "addr", "add", p.Address, "dev", p.Link); err != nil {
		return err
	}
	if err := a.Runner.Run("ip", "link", "set", "dev", p.Link, "up"); err != nil {
		return err
	}
	for i, args := range a.routeCommands("add") {
		own.Routes = i + 1
		if err := a.save(own); err != nil {
			return err
		}
		if err := a.Runner.Run(args...); err != nil {
			return err
		}
	}
	own.Rule = true
	if err := a.save(own); err != nil {
		return err
	}
	if err := a.Runner.Run(a.ruleCommand("add")...); err != nil {
		return err
	}
	return a.Probe()
}

// Probe refuses partial, extra or foreign state. It never mutates kernel state.
func (a Adapter) Probe() error {
	if err := a.Profile.Validate(); err != nil {
		return err
	}
	own, err := a.load()
	if err != nil {
		return err
	}
	link, err := a.Runner.Link(a.Profile.Link)
	if err != nil {
		return err
	}
	if own.Boot != a.Boot || !own.Link || !a.matches(link, own.Index) || !link.Up || !link.IPv6Disabled || len(link.Addresses) != 1 || link.Addresses[0] != a.Profile.Address {
		return errors.New("owned management link not ready")
	}
	routes, rules, err := a.policy()
	if err != nil {
		return err
	}
	if len(routes) != 2 || !has(routes, a.expectedRoutes()[0]) || !has(routes, a.expectedRoutes()[1]) || len(rules) != 1 || rules[0] != a.expectedRule() {
		return errors.New("owned management policy not ready")
	}
	return nil
}

// Cleanup removes exact recorded resources only. Foreign changes cause a refusal.
func (a Adapter) Cleanup() error {
	if err := a.Profile.Validate(); err != nil {
		return err
	}
	own, err := a.load()
	if err != nil {
		return err
	}
	routes, rules, err := a.policy()
	if err != nil {
		return err
	}
	if own.Boot != a.Boot {
		if link, err := a.Runner.Link(a.Profile.Link); err != nil {
			return err
		} else if link.Exists || len(routes) > 0 || len(rules) > 0 {
			return errors.New("foreign resources after boot identity change")
		}
		return a.save(Owned{Hash: a.Profile.Hash(), Boot: a.Boot})
	}
	if len(rules) > 1 {
		return errors.New("duplicate rule at owned priority")
	}
	expected := a.expectedRoutes()
	for _, route := range routes {
		if !has(expected, route) {
			return errors.New("foreign route in owned table")
		}
	}
	for _, rule := range rules {
		if rule != a.expectedRule() {
			return errors.New("foreign rule at owned priority")
		}
	}
	link, err := a.Runner.Link(a.Profile.Link)
	if err != nil {
		return err
	}
	if link.Exists && (!own.Link || !a.matches(link, own.Index)) {
		return errors.New("refuse cleanup of foreign link")
	}
	if link.Exists {
		for _, address := range link.Addresses {
			if address != a.Profile.Address {
				return errors.New("foreign address on owned link")
			}
		}
	}
	if len(rules) > 0 {
		if !own.Rule {
			return errors.New("rule not owned")
		}
		if err := a.Runner.Run(a.ruleCommand("del")...); err != nil {
			return err
		}
	}
	for i, args := range a.routeCommands("del") {
		if has(routes, expected[i]) {
			if own.Routes <= i {
				return errors.New("route not owned")
			}
			if err := a.Runner.Run(args...); err != nil {
				return err
			}
		}
	}
	if link.Exists {
		if err := a.Runner.Run("ip", "link", "delete", "dev", a.Profile.Link); err != nil {
			return err
		}
	}
	if own.Module {
		loaded, err := a.Runner.Module()
		if err != nil {
			return err
		}
		if loaded {
			if err := a.Runner.Run("rmmod", "macvlan"); err != nil {
				return errors.New("owned module busy; cleanup remains pending")
			}
		}
	}
	return a.save(Owned{Hash: a.Profile.Hash(), Boot: a.Boot})
}

func (a Adapter) matches(link Link, index int) bool {
	return link.Exists && (index == 0 || link.Index == index) && link.Parent == a.Profile.Parent && strings.EqualFold(link.MAC, a.Profile.MAC) && link.Kind == "macvlan"
}
func (a Adapter) load() (Owned, error) {
	var own Owned
	err := readPrivate(filepath.Join(a.Store.Dir, "network-owned.json"), &own)
	if errors.Is(err, os.ErrNotExist) {
		return Owned{Hash: a.Profile.Hash()}, nil
	}
	if err != nil {
		return own, err
	}
	if own.Hash != a.Profile.Hash() {
		return own, errors.New("different management profile already owned")
	}
	return own, nil
}
func (a Adapter) save(own Owned) error {
	data, err := json.Marshal(own)
	if err != nil {
		return err
	}
	return Atomic(filepath.Join(a.Store.Dir, "network-owned.json"), data)
}
func (a Adapter) routeCommands(action string) [][]string {
	p := a.Profile
	prefix, _ := netip.ParsePrefix(p.Address)
	source := prefix.Addr().String()
	table := strconv.Itoa(p.Table)
	return [][]string{{"ip", "route", action, "table", table, prefix.Masked().String(), "dev", p.Link, "src", source}, {"ip", "route", action, "table", table, p.Controller, "via", p.Gateway, "dev", p.Link, "src", source}}
}
func (a Adapter) ruleCommand(action string) []string {
	p := a.Profile
	prefix, _ := netip.ParsePrefix(p.Address)
	return []string{"ip", "rule", action, "priority", strconv.Itoa(p.Priority), "from", prefix.Addr().String() + "/32", "to", p.Controller, "lookup", strconv.Itoa(p.Table)}
}
func (a Adapter) expectedRoutes() []string {
	commands := a.routeCommands("add")
	return []string{strings.Join(commands[0][5:], " "), strings.ReplaceAll(strings.Join(commands[1][5:], " "), "/32", "")}
}
func (a Adapter) expectedRule() string {
	p := a.Profile
	prefix, _ := netip.ParsePrefix(p.Address)
	return fmt.Sprintf("%d: from %s to %s lookup %d", p.Priority, prefix.Addr(), strings.TrimSuffix(p.Controller, "/32"), p.Table)
}
func (a Adapter) policy() ([]string, []string, error) {
	routes, err := a.Runner.Output("ip", "route", "show", "table", strconv.Itoa(a.Profile.Table))
	if err != nil {
		return nil, nil, err
	}
	rules, err := a.Runner.Output("ip", "rule", "show")
	if err != nil {
		return nil, nil, err
	}
	var selected []string
	for _, line := range lines(rules) {
		if strings.HasPrefix(line, strconv.Itoa(a.Profile.Priority)+":") {
			selected = append(selected, strings.ReplaceAll(line, "/32", ""))
		}
	}
	return normalizeRoutes(lines(routes)), selected, nil
}
func normalizeRoutes(routes []string) []string {
	for i, line := range routes {
		line = strings.ReplaceAll(line, " scope link", "")
		line = strings.ReplaceAll(line, " proto boot", "")
		routes[i] = line
	}
	return routes
}
func lines(text string) []string {
	var result []string
	for _, line := range strings.Split(text, "\n") {
		if line = strings.Join(strings.Fields(line), " "); line != "" {
			result = append(result, line)
		}
	}
	return result
}
func has(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

// ReadPrivate admits only regular mode-0600 files owned by the effective UID.
func ReadPrivate(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || !ownedByEffectiveUID(info) {
		return nil, errors.New("private file must be owned regular mode 0600")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("read private file")
	}
	return data, nil
}

func readPrivate(path string, value any) error {
	data, err := ReadPrivate(path)
	if err != nil {
		return err
	}
	if _, err := document(data); err != nil {
		return errors.New("invalid private metadata object")
	}
	if err := json.Unmarshal(data, value); err != nil {
		return errors.New("invalid private metadata")
	}
	return nil
}

func (a Adapter) noPendingCanary() error {
	var auth CanaryAuthorization
	err := readPrivate(filepath.Join(a.Store.Dir, "canary-active.json"), &auth)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !auth.Confirmed {
		return errors.New("additive canary guard must be confirmed before permanent management acceptance")
	}
	return nil
}
