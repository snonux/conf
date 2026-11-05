package recovery

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type kernel struct {
	link         Link
	parent       Link
	ipvlanModule bool
	module       bool
	routes       []string
	rules        []string
	calls        [][]string
	fail         string
	after        bool
}

func (k *kernel) Run(args ...string) error {
	k.calls = append(k.calls, append([]string(nil), args...))
	if hasPrefix(args, "ip", "route", "add") || hasPrefix(args, "ip", "route", "del") {
		if err := busyBoxTable(args[4]); err != nil {
			return err
		}
	}
	if hasPrefix(args, "ip", "rule", "add") || hasPrefix(args, "ip", "rule", "del") {
		if err := busyBoxTable(args[10]); err != nil {
			return err
		}
	}
	key := strings.Join(args, " ")
	failed := k.fail != "" && strings.HasPrefix(key, k.fail)
	if failed && !k.after {
		return errors.New("injected operation failure")
	}
	switch {
	case args[0] == "insmod":
		if strings.HasSuffix(args[1], "/ipvlan.ko") {
			k.ipvlanModule = true
		} else {
			k.module = true
		}
	case args[0] == "rmmod":
		if args[1] == "ipvlan" {
			k.ipvlanModule = false
		} else {
			k.module = false
		}
	case hasPrefix(args, "ip", "link", "add"):
		k.link = Link{Exists: true, Index: 17, Parent: args[4], MAC: args[8], Kind: "macvlan"}
	case hasPrefix(args, "ip", "link", "set"):
		k.link.Up = true
	case hasPrefix(args, "ip", "link", "delete"):
		k.link = Link{}
	case hasPrefix(args, "ip", "addr", "add"):
		k.link.Addresses = []string{args[3]}
	case hasPrefix(args, "ip", "route", "add"):
		k.routes = append(k.routes, strings.ReplaceAll(strings.Join(args[5:], " "), "/32", ""))
	case hasPrefix(args, "ip", "route", "del"):
		k.routes = remove(k.routes, strings.ReplaceAll(strings.Join(args[5:], " "), "/32", ""))
	case hasPrefix(args, "ip", "rule", "add"):
		k.rules = append(k.rules, args[4]+": from "+strings.TrimSuffix(args[6], "/32")+" to "+strings.TrimSuffix(args[8], "/32")+" lookup "+args[10])
	case hasPrefix(args, "ip", "rule", "del"):
		k.rules = nil
	default:
		return errors.New("unexpected mutation")
	}
	if failed {
		return errors.New("failure after mutation")
	}
	return nil
}
func (k *kernel) Output(args ...string) (string, error) {
	if hasPrefix(args, "ip", "route", "show") {
		if err := busyBoxTable(args[4]); err != nil {
			return "", err
		}
		return strings.Join(k.routes, "\n"), nil
	}
	if hasPrefix(args, "ip", "rule", "show") {
		return strings.Join(k.rules, "\n"), nil
	}
	return "", errors.New("unexpected inspection")
}
func (k *kernel) Link(name string) (Link, error) {
	if name == "eth0" {
		return k.parent, nil
	}
	return k.link, nil
}
func (k *kernel) Module(kind string) (bool, error) {
	if kind == "ipvlan" {
		return k.ipvlanModule, nil
	}
	return k.module, nil
}
func (k *kernel) CreateIPvlan(parent, child string) error {
	k.calls = append(k.calls, []string{"netlink-ipvlan-l2", parent, child})
	failed := k.fail == "netlink-ipvlan-l2"
	if failed && !k.after {
		return errors.New("injected create failure")
	}
	k.link = Link{Exists: true, Index: 17, Parent: parent, ParentIndex: k.parent.Index, MAC: k.parent.MAC, Kind: "ipvlan", ModeObserved: true, Mode: 0}
	if failed {
		return errors.New("failure after create")
	}
	return nil
}
func (k *kernel) DisableIPv6(link string) error {
	if link != "jkrecover" {
		return errors.New("foreign sysctl")
	}
	k.link.IPv6Disabled = true
	return nil
}

// Model the installed parser limit found by native read-only inspection, not
// the full Linux routing-table ABI. Priority is deliberately not parsed here.
func busyBoxTable(value string) error {
	table, err := strconv.Atoi(value)
	if err != nil || table < 0 || table > 1023 {
		return errors.New("BusyBox table parser rejected identifier")
	}
	return nil
}

func TestProfileSeparatesBusyBoxTableAndRulePriority(t *testing.T) {
	for _, tc := range []struct {
		table, priority int
		valid           bool
	}{
		{999, 1140, false}, {1000, 1140, true}, {1023, 1140, true},
		{1024, 1140, false}, {32000, 1140, false},
		{1000, 999, false}, {1000, 1000, true}, {1023, 32000, true},
		{1000, 32001, false},
	} {
		t.Run(strconv.Itoa(tc.table)+"/"+strconv.Itoa(tc.priority), func(t *testing.T) {
			a, k := setupAdapter(t)
			a.Profile.Table, a.Profile.Priority = tc.table, tc.priority
			if tc.valid {
				if err := a.Ensure(); err != nil {
					t.Fatal(err)
				}
				if err := a.Probe(); err != nil {
					t.Fatal(err)
				}
				if err := a.Cleanup(); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := a.Ensure(); err == nil {
					t.Fatal("unsupported profile accepted")
				}
				if len(k.calls) != 0 || k.link.Exists || k.module {
					t.Fatal("invalid profile reached kernel mutations")
				}
			}
		})
	}
}

func TestFakeKernelRejectsNativeUnsupportedTable(t *testing.T) {
	k := new(kernel)
	for _, args := range [][]string{
		{"ip", "route", "show", "table", "1024"},
		{"ip", "route", "show", "table", "1140"},
	} {
		if _, err := k.Output(args...); err == nil {
			t.Fatal("fixture lost native read-only table parser rejection")
		}
	}
	if _, err := k.Output("ip", "route", "show", "table", "1023"); err != nil {
		t.Fatal(err)
	}
}

func hasPrefix(args []string, prefix ...string) bool {
	if len(args) < len(prefix) {
		return false
	}
	for i, v := range prefix {
		if args[i] != v {
			return false
		}
	}
	return true
}
func remove(values []string, want string) []string {
	var result []string
	for _, v := range values {
		if v != want {
			result = append(result, v)
		}
	}
	return result
}
func setupAdapter(t *testing.T) (Adapter, *kernel) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	k := new(kernel)
	p := Profile{Device: "exampledevice0001", Cabling: "shared-igc0", Parent: "eth0", Link: "jkrecover", MAC: "02:00:5e:00:53:01", Address: "192.168.11.201/24", Gateway: "192.168.11.2", Controller: "10.99.0.2/32", Table: 1000, Priority: 1201}
	return Adapter{Store: Store{Dir: dir}, Boot: "boot-original-001", Profile: p, Runner: k}, k
}
func privateJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestAdapterCreatesExactOwnedResourcesAndCleans(t *testing.T) {
	for _, preexisting := range []bool{false, true} {
		t.Run(strconv.FormatBool(preexisting), func(t *testing.T) {
			a, k := setupAdapter(t)
			k.module = preexisting
			if err := a.Ensure(); err != nil {
				t.Fatal(err)
			}
			if err := a.Probe(); err != nil {
				t.Fatal(err)
			}
			mutations := len(k.calls)
			if err := a.Ensure(); err != nil {
				t.Fatal(err)
			}
			if len(k.calls) != mutations {
				t.Fatal("reapply mutated already correct state")
			}
			if err := a.Cleanup(); err != nil {
				t.Fatal(err)
			}
			if err := a.Cleanup(); err != nil {
				t.Fatal(err)
			}
			if k.link.Exists || len(k.routes) != 0 || len(k.rules) != 0 || k.module != preexisting {
				t.Fatal("cleanup did not restore ownership baseline")
			}
			for _, args := range k.calls {
				line := strings.Join(args, " ")
				if strings.Contains(line, "default") || strings.Contains(line, "flush") || hasPrefix(args, "ip", "addr", "del") || hasPrefix(args, "ip", "link", "set", "dev", "eth0") || strings.Contains(line, "mode bridge") {
					t.Fatalf("unowned or unsupported mutation %s", line)
				}
			}
		})
	}
}

func TestPartialMutationCleanup(t *testing.T) {
	failures := []string{"insmod", "ip link add", "ip addr add", "ip link set", "ip route add", "ip rule add"}
	for _, operation := range failures {
		for _, after := range []bool{false, true} {
			t.Run(operation+strconv.FormatBool(after), func(t *testing.T) {
				a, k := setupAdapter(t)
				k.fail = operation
				k.after = after
				if err := a.Ensure(); err == nil {
					t.Fatal("injected failure missed")
				}
				k.fail = ""
				if err := a.Cleanup(); err != nil {
					t.Fatal(err)
				}
				if k.link.Exists || k.module || len(k.routes) > 0 || len(k.rules) > 0 {
					t.Fatal("partial mutation leaked owned resources")
				}
			})
		}
	}
}

func TestForeignOwnershipRefusedWithoutDeletion(t *testing.T) {
	for _, change := range []string{"mac", "parent", "kind", "ifindex", "address", "route", "rule", "duplicate-rule", "boot"} {
		t.Run(change, func(t *testing.T) {
			a, k := setupAdapter(t)
			if err := a.Ensure(); err != nil {
				t.Fatal(err)
			}
			switch change {
			case "mac":
				k.link.MAC = "02:00:5e:00:53:02"
			case "parent":
				k.link.Parent = "eth9"
			case "kind":
				k.link.Kind = "vlan"
			case "ifindex":
				k.link.Index++
			case "address":
				k.link.Addresses = append(k.link.Addresses, "203.0.113.1/24")
			case "route":
				k.routes = append(k.routes, "203.0.113.0/24 dev foreign")
			case "rule":
				k.rules = []string{"1201: from all lookup 1000"}
			case "duplicate-rule":
				k.rules = append(k.rules, k.rules[0])
			case "boot":
				a.Boot = "newboot-00000002"
			}
			before := len(k.calls)
			if err := a.Cleanup(); err == nil {
				t.Fatal("foreign resource deleted")
			}
			if len(k.calls) != before {
				t.Fatal("foreign cleanup partially mutated state")
			}
		})
	}
}
func TestOccupiedNamesPolicyAndBusyModule(t *testing.T) {
	for _, test := range []string{"link", "table", "rule"} {
		t.Run(test, func(t *testing.T) {
			a, k := setupAdapter(t)
			switch test {
			case "link":
				k.link = Link{Exists: true, MAC: a.Profile.MAC, Kind: "macvlan", Parent: "eth0"}
			case "table":
				k.routes = []string{"default via 203.0.113.1"}
			case "rule":
				k.rules = []string{"1201: from all lookup main"}
			}
			if err := a.Ensure(); err == nil {
				t.Fatal("occupied identifiers adopted")
			}
			if len(k.calls) > 0 {
				t.Fatal("mutation before ownership preflight")
			}
		})
	}
	a, k := setupAdapter(t)
	if err := a.Ensure(); err != nil {
		t.Fatal(err)
	}
	k.fail = "rmmod"
	if err := a.Cleanup(); err == nil {
		t.Fatal("busy module falsely cleaned")
	}
	owned, err := a.load()
	if err != nil {
		t.Fatal(err)
	}
	if !owned.Module {
		t.Fatal("busy module ownership forgotten")
	}
	k.fail = ""
	if err := a.Cleanup(); err != nil {
		t.Fatal(err)
	}
}
func TestRebootCannotUnloadForeignModule(t *testing.T) {
	a, k := setupAdapter(t)
	if err := a.Ensure(); err != nil {
		t.Fatal(err)
	}
	a.Boot = "boot-new-00000002"
	k.link = Link{}
	k.routes = nil
	k.rules = nil
	// The same module may have been loaded by another owner on this boot.
	before := len(k.calls)
	if err := a.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if len(k.calls) != before || !k.module {
		t.Fatal("previous boot ownership unloaded current module")
	}
	if err := a.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := a.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if !k.module {
		t.Fatal("new boot adopted preexisting module")
	}
}
func TestCablingAndAcceptanceGates(t *testing.T) {
	a, k := setupAdapter(t)
	for _, change := range []string{"future-cabling", "wrong-gateway", "wrong-controller", "global-mac", "wrong-parent", "main-table"} {
		t.Run(change, func(t *testing.T) {
			bad := a
			switch change {
			case "future-cabling":
				bad.Profile.Cabling = "igc1"
				bad.Profile.Address = "10.98.0.201/24"
				bad.Profile.Gateway = "10.98.0.1"
			case "wrong-gateway":
				bad.Profile.Gateway = "192.168.11.1"
			case "wrong-controller":
				bad.Profile.Controller = "0.0.0.0/0"
			case "global-mac":
				bad.Profile.MAC = "00:00:5e:00:53:01"
			case "wrong-parent":
				bad.Profile.Parent = "eth1"
			case "main-table":
				bad.Profile.Table = 254
			}
			if err := bad.Ensure(); err == nil {
				t.Fatal("unreviewed profile accepted")
			}
			if len(k.calls) > 0 {
				t.Fatal("mutated before cabling validation")
			}
		})
	}
	if err := a.Accepted(); err == nil {
		t.Fatal("missing acceptance trusted")
	}
	receipt := Receipt{Hash: a.Profile.Hash(), Device: a.Profile.Device, Cabling: a.Profile.Cabling, FixedPath: true, Boot: true, Renew: true, Owner: true, Services: true, TransactionHook: true}
	receipt.Boot = false
	privateJSON(t, filepath.Join(a.Store.Dir, "management-accepted.json"), receipt)
	if err := a.Accepted(); err == nil {
		t.Fatal("unproved boot accepted")
	}
	receipt.Boot = true
	privateJSON(t, filepath.Join(a.Store.Dir, "management-accepted.json"), receipt)
	if err := a.Accepted(); err == nil {
		t.Fatal("receipt accepted without live state")
	}
	if err := a.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := a.Accepted(); err != nil {
		t.Fatal(err)
	}
	k.link.Up = false
	if err := a.Accepted(); err == nil {
		t.Fatal("stale accepted endpoint trusted")
	}
}
func TestBoundedCanaryBootDoesNotNeedAcceptedReceipt(t *testing.T) {
	a, k := setupAdapter(t)
	now := int64(3_000_000_000)
	auth := CanaryAuthorization{Hash: a.Profile.Hash(), Generation: genA, Boot: "boot-previous-001", Deadline: WindowNS, BootAllowance: true}
	privateJSON(t, filepath.Join(a.Store.Dir, "canary-active.json"), auth)
	starts := 0
	start := func(s State) error {
		starts++
		var durable CanaryAuthorization
		if err := readPrivate(filepath.Join(a.Store.Dir, "canary-active.json"), &durable); err != nil {
			t.Fatal(err)
		}
		if durable.BootAllowance || durable.Boot != a.Boot || s.Deadline != durable.Deadline {
			t.Fatal("boot allowance not consumed before worker")
		}
		return nil
	}
	if err := ManagementBoot(a, a.Boot, now, start); err != nil {
		t.Fatal(err)
	}
	var consumed CanaryAuthorization
	if err := readPrivate(filepath.Join(a.Store.Dir, "canary-active.json"), &consumed); err != nil {
		t.Fatal(err)
	}
	if consumed.Deadline != now+WindowNS || starts != 1 || !k.link.Exists {
		t.Fatal("bounded management boot was not established")
	}
	if err := ManagementBoot(a, a.Boot, now+1_000_000_000, start); err != nil {
		t.Fatal(err)
	}
	var repeated CanaryAuthorization
	if err := readPrivate(filepath.Join(a.Store.Dir, "canary-active.json"), &repeated); err != nil {
		t.Fatal(err)
	}
	if repeated.Deadline != consumed.Deadline {
		t.Fatal("same-boot reapply extended canary")
	}
	a.Boot = "boot-third-000003"
	k.link = Link{}
	k.routes = nil
	k.rules = nil
	k.module = false
	if err := ManagementBoot(a, a.Boot, 1_000_000_000, start); err != nil {
		t.Fatal(err)
	}
	if starts != 2 || k.link.Exists {
		t.Fatal("second pending boot recreated canary")
	}
	if err := a.Accepted(); err == nil {
		t.Fatal("bootstrap implied full native acceptance")
	}
}
func TestCanaryWorkerFailureAndExpiry(t *testing.T) {
	a, k := setupAdapter(t)
	now := int64(1_000_000_000)
	a.Now = func() (int64, error) { return now, nil }
	auth := CanaryAuthorization{Hash: a.Profile.Hash(), Generation: genA, Boot: a.Boot, Deadline: now + WindowNS}
	privateJSON(t, filepath.Join(a.Store.Dir, "canary-active.json"), auth)
	if err := a.Bootstrap(a.Boot, now); err != nil {
		t.Fatal(err)
	}
	if err := ManagementBoot(a, a.Boot, now, func(State) error { return errors.New("lost worker") }); err == nil {
		t.Fatal("worker failure ignored")
	}
	if k.link.Exists || len(k.routes) > 0 || len(k.rules) > 0 {
		t.Fatal("worker failure returned with unguarded canary")
	}
	auth.Expired = false
	privateJSON(t, filepath.Join(a.Store.Dir, "canary-active.json"), auth)
	if err := a.Bootstrap(a.Boot, auth.Deadline); err == nil {
		t.Fatal("expired canary mutated")
	}
	if err := a.Bootstrap("wrong-boot", now); err == nil {
		t.Fatal("wrong boot mutated")
	}
	if err := KeepCanary(a, a.Boot, false); err == nil {
		t.Fatal("unproved canary confirmed")
	}
	if err := a.Bootstrap(a.Boot, now); err != nil {
		t.Fatal(err)
	}
	if err := KeepCanary(a, a.Boot, true); err != nil {
		t.Fatal(err)
	}
	if err := a.Accepted(); err == nil {
		t.Fatal("keep-canary forged native acceptance")
	}
}

func TestCanaryCannotReturnReadyAfterSetupDeadline(t *testing.T) {
	a, k := setupAdapter(t)
	now := int64(1_000_000_000)
	auth := CanaryAuthorization{Hash: a.Profile.Hash(), Generation: genA, Boot: a.Boot, Deadline: now + WindowNS}
	privateJSON(t, filepath.Join(a.Store.Dir, "canary-active.json"), auth)
	tick := now
	a.Now = func() (int64, error) { return tick, nil }
	if err := ManagementBoot(a, a.Boot, now, func(State) error { tick = auth.Deadline; return nil }); err == nil {
		t.Fatal("late worker readiness accepted")
	}
	if len(k.calls) != 0 || k.link.Exists {
		t.Fatal("setup mutated after late worker readiness")
	}
}

func TestPermanentAcceptanceCannotRacePendingCanaryCleanup(t *testing.T) {
	a, _ := setupAdapter(t)
	if err := a.Ensure(); err != nil {
		t.Fatal(err)
	}
	receipt := Receipt{Hash: a.Profile.Hash(), Device: a.Profile.Device, Cabling: a.Profile.Cabling, FixedPath: true, Boot: true, Renew: true, Owner: true, Services: true, TransactionHook: true}
	privateJSON(t, filepath.Join(a.Store.Dir, "management-accepted.json"), receipt)
	auth := CanaryAuthorization{Hash: a.Profile.Hash(), Generation: genA, Boot: a.Boot, Deadline: WindowNS}
	privateJSON(t, filepath.Join(a.Store.Dir, "canary-active.json"), auth)
	if err := a.Accepted(); err == nil {
		t.Fatal("pending additive cleanup can remove accepted recovery path")
	}
	auth.Confirmed = true
	privateJSON(t, filepath.Join(a.Store.Dir, "canary-active.json"), auth)
	if err := a.Accepted(); err != nil {
		t.Fatal(err)
	}
}
