package recovery

import (
	"net/netip"
	"strings"
	"testing"
)

func readinessFixtureFile(executable bool) ReadinessFile {
	mode := uint32(0644)
	if executable {
		mode = 0755
	}
	return ReadinessFile{Hash: strings.Repeat("a", 64), Mode: mode, Links: 1, Inode: 2, Device: 7, Regular: true}
}

func readinessFixtureLease() []byte {
	values := map[string]string{"ip": "192.0.2.10", "subnet": "255.255.255.0", "serverid": "192.0.2.1", "router": "192.0.2.1", "dns": "192.0.2.53 192.0.2.54", "lease": "600"}
	var b strings.Builder
	b.WriteString("# generated clock is deliberately not trusted\n")
	for _, key := range strings.Fields(readinessLeaseKeys) {
		b.WriteString(key + "=" + values[key] + "\n")
	}
	return []byte(b.String())
}

func readinessFixtureSnapshot(t *testing.T) ParentReadinessSnapshot {
	t.Helper()
	binary, script := readinessFixtureFile(true), readinessFixtureFile(true)
	script.Hash = readinessHandlerSHA
	lease, err := parseReadinessLease(readinessFixtureLease(), readinessFixtureFile(false))
	if err != nil {
		t.Fatal(err)
	}
	process := ReadinessProcesses{Candidates: 1, Owner: ReadinessOwner{PID: 42, PPID: 1, Group: 42, Session: 42, Birth: 123, State: 'S', Argv: []string{"udhcpc", "-i", "eth0", "-x", "hostname:a-example"}, Executable: binary}}
	return ParentReadinessSnapshot{Boot: "current-boot", CanonicalLease: readinessLeasePath, Parent: Link{Exists: true, Index: 2, Up: true, MAC: "30:00:00:00:00:01", Addresses: []string{"192.0.2.10/24"}}, Begin: process, End: process, Handler: script, Busybox: binary, HooksKnown: true, Lease: lease, Mount: ReadinessMount{ID: 5, Path: "/run", Type: "tmpfs", Device: 7}, Defaults: []ReadinessRoute{{Gateway: netip.MustParseAddr("192.0.2.1"), Interface: "eth0"}}, DNS: append([]netip.Addr(nil), lease.DNS...)}
}

func TestParentReadinessRequiresCompleteStableConjunction(t *testing.T) {
	valid := readinessFixtureSnapshot(t)
	if err := ValidateParentReadiness("current-boot", "30:00:00:00:00:01", valid, valid); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(*ParentReadinessSnapshot)
	}{
		{"old boot", func(s *ParentReadinessSnapshot) { s.Boot = "old-boot" }},
		{"different factory MAC", func(s *ParentReadinessSnapshot) { s.Parent.MAC = "30:00:00:00:00:02" }},
		{"parent DOWN", func(s *ParentReadinessSnapshot) { s.Parent.Up = false }},
		{"handler drift", func(s *ParentReadinessSnapshot) { s.Handler.Hash = strings.Repeat("b", 64) }},
		{"executable hook", func(s *ParentReadinessSnapshot) { s.ExecutableHooks = 1 }},
		{"unknown hooks", func(s *ParentReadinessSnapshot) { s.HooksKnown = false }},
		{"foreground owner", func(s *ParentReadinessSnapshot) { s.Begin.Owner.Session = 1; s.End.Owner.Session = 1 }},
		{"wrong parent", func(s *ParentReadinessSnapshot) { s.Begin.Owner.PPID = 3; s.End.Owner.PPID = 3 }},
		{"callback child", func(s *ParentReadinessSnapshot) { s.Begin.Children = 1; s.End.Children = 1 }},
		{"unknown process", func(s *ParentReadinessSnapshot) { s.Begin.Unknown = 1; s.End.Unknown = 1 }},
		{"multiple owners", func(s *ParentReadinessSnapshot) { s.Begin.Candidates = 2; s.End.Candidates = 2 }},
		{"owner birth race", func(s *ParentReadinessSnapshot) { s.End.Owner.Birth++ }},
		{"executable mismatch", func(s *ParentReadinessSnapshot) { s.Begin.Owner.Executable.Hash = strings.Repeat("b", 64) }},
		{"unspecified lease server", func(s *ParentReadinessSnapshot) { s.Lease.Server = netip.IPv4Unspecified() }},
		{"duplicate lease DNS", func(s *ParentReadinessSnapshot) { s.Lease.DNS = []netip.Addr{s.Lease.DNS[0], s.Lease.DNS[0]} }},
		{"incomplete lease", func(s *ParentReadinessSnapshot) { s.Lease.Complete = false }},
		{"persistent lease", func(s *ParentReadinessSnapshot) { s.Mount.Type = "ext4" }},
		{"different mount device", func(s *ParentReadinessSnapshot) { s.Mount.Device++ }},
		{"wrong covering mount", func(s *ParentReadinessSnapshot) { s.Mount.Path = "/run/other" }},
		{"symlink lease", func(s *ParentReadinessSnapshot) { s.Lease.File.Regular = false }},
		{"unapplied IP", func(s *ParentReadinessSnapshot) { s.Parent.Addresses = nil }},
		{"unapplied DNS", func(s *ParentReadinessSnapshot) { s.DNS = nil }},
		{"unapplied router", func(s *ParentReadinessSnapshot) { s.Defaults = nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := readinessFixtureSnapshot(t)
			tc.mutate(&s)
			if ValidateParentReadiness("current-boot", "30:00:00:00:00:01", s, s) == nil {
				t.Fatal("accepted incomplete readiness")
			}
		})
	}
	changed := readinessFixtureSnapshot(t)
	changed.Lease.File.Inode++
	if ValidateParentReadiness("current-boot", "30:00:00:00:00:01", valid, changed) == nil {
		t.Fatal("accepted a torn natural renewal")
	}
}

func TestReadinessArgvPinsHandlerAndRejectsAmbiguity(t *testing.T) {
	for _, args := range [][]string{{"udhcpc", "-ieth0"}, {"udhcpc", "-i", "eth0", "-s", readinessHandler}, {"udhcpc", "-ieth0", "-s" + readinessHandler, "-xhostname:a-example"}} {
		if err := readinessArgv(args); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"udhcpc"}, {"udhcpc", "-ieth1"}, {"udhcpc", "-ieth0", "-ieth0"}, {"udhcpc", "-ieth0", "-s/custom"}, {"udhcpc", "-ieth0", "-s", readinessHandler, "-s", readinessHandler}, {"udhcpc", "-ieth0", "-f"}, {"udhcpc", "-ieth0", "-b"}, {"udhcpc", "-ieth0", "-q"}, {"udhcpc", "-ieth0", "-xhostname:-bad"}} {
		if readinessArgv(args) == nil {
			t.Fatalf("accepted %q", args)
		}
	}
}

func TestReadinessLeaseRejectsPartialDuplicateAndMalformedRecords(t *testing.T) {
	valid := readinessFixtureLease()
	for _, data := range [][]byte{valid[:len(valid)-1], append(append([]byte(nil), valid...), []byte("ip=192.0.2.10\n")...), []byte(strings.ReplaceAll(string(valid), "subnet=255.255.255.0", "subnet=255.0.255.0")), []byte(strings.ReplaceAll(string(valid), "lease=600", "lease=0")), []byte(strings.ReplaceAll(string(valid), "dns=192.0.2.53 192.0.2.54", "dns=192.0.2.53 192.0.2.53")), []byte(strings.ReplaceAll(string(valid), "bootfile=\n", ""))} {
		if _, err := parseReadinessLease(data, readinessFixtureFile(false)); err == nil {
			t.Fatal("accepted malformed lease")
		}
	}
}
