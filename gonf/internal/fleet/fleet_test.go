package fleet_test

import (
	"testing"

	"codeberg.org/snonux/conf/gonf/internal/fleet"
	. "github.com/snonux/gonf/api"
)

func TestRegisterPiHostsAndFleets(t *testing.T) {
	ResetInventory()
	fleet.Register()

	wantHosts := map[string]HostInfo{
		"blowfish": {
			Name: "blowfish", User: "rex",
			SSHHost: "blowfish.buetow.org", Port: 2, Privilege: "doas",
		},
		"fishfinger": {
			Name: "fishfinger", User: "rex",
			SSHHost: "fishfinger.buetow.org", Port: 2, Privilege: "doas",
		},
		"pi0": {
			Name: "pi0", User: "paul",
			SSHHost: "pi0.lan.buetow.org", Port: 22, Privilege: "doas",
		},
		"pi1": {
			Name: "pi1", User: "paul",
			SSHHost: "pi1.lan.buetow.org", Port: 22, Privilege: "doas",
		},
		"pi2": {
			Name: "pi2", User: "paul",
			SSHHost: "pi2.lan.buetow.org", Port: 22, Privilege: "sudo",
		},
		"pi3": {
			Name: "pi3", User: "paul",
			SSHHost: "pi3.lan.buetow.org", Port: 22, Privilege: "sudo",
		},
	}

	got := map[string]HostInfo{}
	for _, h := range Hosts() {
		got[h.Name] = h
	}
	if len(got) != len(wantHosts) {
		t.Fatalf("Hosts() len = %d, want %d (%v)", len(got), len(wantHosts), got)
	}
	for name, want := range wantHosts {
		h, ok := got[name]
		if !ok {
			t.Fatalf("missing host %q", name)
		}
		// Exact Port/Privilege asserts catch omitted WithSSHPort (Port=0 →
		// ssh omits -p → ~/.ssh/config Port 2) and a wrong doas/sudo split.
		if h.User != want.User || h.SSHHost != want.SSHHost ||
			h.Port != want.Port || h.Privilege != want.Privilege {
			t.Errorf("%s: got %+v, want %+v", name, h, want)
		}
	}

	wantFleets := map[string][]string{
		"frontends":  {"blowfish", "fishfinger"},
		"netbsd-pis": {"pi0", "pi1"},
		"rocky-pis":  {"pi2", "pi3"},
		"pis":        {"pi0", "pi1", "pi2", "pi3"},
	}
	gotFleets := map[string][]string{}
	for _, f := range Fleets() {
		gotFleets[f.Name] = f.Hosts
	}
	if len(gotFleets) != len(wantFleets) {
		t.Fatalf("Fleets() len = %d, want %d (%v)", len(gotFleets), len(wantFleets), gotFleets)
	}
	for name, wantMembers := range wantFleets {
		members, ok := gotFleets[name]
		if !ok {
			t.Fatalf("missing fleet %q", name)
		}
		if len(members) != len(wantMembers) {
			t.Fatalf("fleet %s: members %v, want %v", name, members, wantMembers)
		}
		for i, m := range wantMembers {
			if members[i] != m {
				t.Errorf("fleet %s[%d]=%q, want %q", name, i, members[i], m)
			}
		}
	}
}
