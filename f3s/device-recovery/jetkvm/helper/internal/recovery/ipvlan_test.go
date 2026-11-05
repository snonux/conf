package recovery

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func ipvlanAdapter(t *testing.T) (Adapter, *kernel) {
	t.Helper()
	a, k := setupAdapter(t)
	a.Profile.Kind = "ipvlan"
	a.Profile.MAC = "30:52:53:00:00:01"
	k.parent = Link{Exists: true, Index: 2, MAC: a.Profile.MAC}
	return a, k
}

func TestOldProfileHashAndDefaultMacvlanRemainUnchanged(t *testing.T) {
	a, k := setupAdapter(t)
	// Original JSON field order/bytes are the hash contract for installed profiles.
	old := `{"device":"exampledevice0001","cabling":"shared-igc0","parent":"eth0","link":"jkrecover","mac":"02:00:5e:00:53:01","address":"192.168.11.201/24","gateway":"192.168.11.2","controller":"10.99.0.2/32","table":1000,"priority":1201}`
	sum := sha256.Sum256([]byte(old))
	data, err := json.Marshal(a.Profile)
	if err != nil || string(data) != old || a.Profile.Hash() != hex.EncodeToString(sum[:]) {
		t.Fatal("legacy profile serialization/hash changed")
	}
	if err := a.Ensure(); err != nil {
		t.Fatal(err)
	}
	if k.link.Kind != "macvlan" || k.ipvlanModule {
		t.Fatal("default no longer macvlan")
	}
	explicit := a.Profile
	explicit.Kind = "macvlan"
	if err := explicit.Validate(); err != nil {
		t.Fatal(err)
	}
	if explicit.Hash() == a.Profile.Hash() {
		t.Fatal("explicit kind not bound to hash")
	}
}

func TestIPvlanL2CreateProbeCleanupPreservesOtherModules(t *testing.T) {
	for _, preexisting := range []bool{false, true} {
		a, k := ipvlanAdapter(t)
		k.ipvlanModule = preexisting
		k.module = true
		if err := a.Ensure(); err != nil {
			t.Fatal(err)
		}
		if err := a.Probe(); err != nil {
			t.Fatal(err)
		}
		before := len(k.calls)
		if err := a.Ensure(); err != nil {
			t.Fatal(err)
		}
		if len(k.calls) != before {
			t.Fatal("idempotent ensure mutated")
		}
		for _, args := range k.calls {
			command := strings.Join(args, " ")
			if strings.Contains(command, "macvlan") || strings.Contains(command, "address ") || strings.Contains(command, "promisc") {
				t.Fatal("IPvlan touched unrelated module/MAC/flags")
			}
		}
		if err := a.Cleanup(); err != nil {
			t.Fatal(err)
		}
		if err := a.Cleanup(); err != nil {
			t.Fatal(err)
		}
		if k.link.Exists || len(k.routes) > 0 || len(k.rules) > 0 || k.ipvlanModule != preexisting || !k.module {
			t.Fatal("ownership baseline not restored")
		}
	}
}

func TestIPvlanRefusesInvalidKindOrParentBeforeMutation(t *testing.T) {
	for _, change := range []string{"kind", "local-mac", "missing-parent", "parent-mac", "parent-index"} {
		t.Run(change, func(t *testing.T) {
			a, k := ipvlanAdapter(t)
			switch change {
			case "kind":
				a.Profile.Kind = "vlan"
			case "local-mac":
				a.Profile.MAC = "02:00:00:00:00:01"
			case "missing-parent":
				k.parent.Exists = false
			case "parent-mac":
				k.parent.MAC = "30:00:00:00:00:02"
			case "parent-index":
				k.parent.Index = 0
			}
			if err := a.Ensure(); err == nil {
				t.Fatal("invalid parent/kind accepted")
			}
			if len(k.calls) != 0 {
				t.Fatal("invalid identity reached mutation")
			}
		})
	}
}

func TestIPvlanForeignIdentityRefusesProbeAndCleanup(t *testing.T) {
	for _, change := range []string{"mode", "unobserved-mode", "parent", "parent-index", "factory-mac", "child-mac", "child-index", "kind"} {
		t.Run(change, func(t *testing.T) {
			a, k := ipvlanAdapter(t)
			if err := a.Ensure(); err != nil {
				t.Fatal(err)
			}
			switch change {
			case "mode":
				k.link.Mode = 1
			case "unobserved-mode":
				k.link.ModeObserved = false
			case "parent":
				k.link.Parent = "eth9"
			case "parent-index":
				k.link.ParentIndex++
			case "factory-mac":
				k.parent.MAC = "30:00:00:00:00:02"
			case "child-mac":
				k.link.MAC = "30:00:00:00:00:02"
			case "child-index":
				k.link.Index++
			case "kind":
				k.link.Kind = "macvlan"
			}
			before := len(k.calls)
			if err := a.Probe(); err == nil {
				t.Fatal("foreign identity ready")
			}
			if err := a.Cleanup(); err == nil {
				t.Fatal("foreign identity deleted")
			}
			if len(k.calls) != before {
				t.Fatal("foreign cleanup mutated")
			}
		})
	}
}

func TestIPvlanPartialFailuresRetainCleanupIntent(t *testing.T) {
	for _, op := range []string{"insmod", "netlink-ipvlan-l2", "ip addr add", "ip link set", "ip route add", "ip rule add"} {
		for _, after := range []bool{false, true} {
			a, k := ipvlanAdapter(t)
			k.fail = op
			k.after = after
			if err := a.Ensure(); err == nil {
				t.Fatal("injected failure missed", op)
			}
			k.fail = ""
			if err := a.Cleanup(); err != nil {
				t.Fatal(err)
			}
			if k.link.Exists || k.ipvlanModule || len(k.routes) > 0 || len(k.rules) > 0 {
				t.Fatal("partial failure leaked", op)
			}
		}
	}
	a, k := ipvlanAdapter(t)
	if err := a.Ensure(); err != nil {
		t.Fatal(err)
	}
	k.fail = "rmmod"
	if err := a.Cleanup(); err == nil {
		t.Fatal("busy module falsely cleaned")
	}
	own, err := a.load()
	if err != nil || !own.Module || !k.ipvlanModule {
		t.Fatal("busy ownership lost")
	}
	k.fail = ""
	if err := a.Cleanup(); err != nil {
		t.Fatal(err)
	}
}

func TestIPvlanTypedCreateRequestAndModeReadback(t *testing.T) {
	request := ipvlanRequest(2)
	messages, err := syscall.ParseNetlinkMessage(request)
	if err != nil || len(messages) != 1 {
		t.Fatal("invalid request framing")
	}
	m := messages[0]
	if m.Header.Type != syscall.RTM_NEWLINK || m.Header.Seq != 1 || m.Header.Flags != syscall.NLM_F_REQUEST|syscall.NLM_F_ACK|syscall.NLM_F_CREATE|syscall.NLM_F_EXCL || !bytes.Equal(m.Data[:16], make([]byte, 16)) {
		t.Fatal("unsafe create header")
	}
	attrs, err := nestedAttrs(m.Data[16:])
	if err != nil {
		t.Fatal(err)
	}
	if len(attrs) != 3 || binary.NativeEndian.Uint32(attrs[5]) != 2 || string(attrs[3]) != "jkrecover\x00" {
		t.Fatal("unrelated create attributes")
	}
	kind, mode, seen, err := linkInfo(attrs[18])
	if err != nil || kind != "ipvlan" || mode != 0 || !seen {
		t.Fatal("explicit L2 not encoded")
	}
	for _, v := range []uint16{0, 1, 2} {
		data := make([]byte, 2)
		binary.NativeEndian.PutUint16(data, v)
		info := append(linkAttr(2, linkAttr(1, data)), linkAttr(1, []byte("ipvlan\x00"))...)
		kind, mode, seen, err = linkInfo(info)
		if err != nil || kind != "ipvlan" || mode != v || !seen {
			t.Fatal("mode parse mismatch")
		}
	}
	for _, data := range [][]byte{linkAttr(1, []byte("ipvlan\x00")), append(linkAttr(1, []byte("ipvlan\x00")), linkAttr(2, linkAttr(1, []byte{0}))...), {1, 0, 1, 0}} {
		if _, _, _, err := linkInfo(data); err == nil {
			t.Fatal("missing/truncated mode accepted")
		}
	}
}

func TestIPvlanAcknowledgmentRejectsKernelErrorsTruncationAndWrongSequence(t *testing.T) {
	ack := make([]byte, 36)
	binary.NativeEndian.PutUint32(ack, 36)
	binary.NativeEndian.PutUint16(ack[4:], syscall.NLMSG_ERROR)
	binary.NativeEndian.PutUint32(ack[8:], 1)
	copy(ack[20:], ipvlanRequest(2)[:16])
	if err := ipvlanAck(ack); err != nil {
		t.Fatal(err)
	}
	for _, change := range []string{"error", "sequence", "type", "truncated", "duplicate", "echo"} {
		data := append([]byte(nil), ack...)
		switch change {
		case "error":
			binary.NativeEndian.PutUint32(data[16:], uint32(0xffffffff))
		case "sequence":
			binary.NativeEndian.PutUint32(data[8:], 2)
		case "type":
			binary.NativeEndian.PutUint16(data[4:], syscall.NLMSG_DONE)
		case "truncated":
			data = data[:18]
		case "echo":
			binary.NativeEndian.PutUint32(data[28:], 2)
		case "duplicate":
			data = append(data, ack...)
		}
		if err := ipvlanAck(data); err == nil {
			t.Fatal("invalid acknowledgment accepted", change)
		}
	}
	if _, err := (Native{}).Module("foreign"); err == nil {
		t.Fatal("foreign module accepted")
	}
	if err := (Native{}).CreateIPvlan("eth9", "foreign"); err == nil {
		t.Fatal("foreign create accepted")
	}
}

func TestIPvlanNewBootDoesNotUnloadForeignModule(t *testing.T) {
	a, k := ipvlanAdapter(t)
	if err := a.Ensure(); err != nil {
		t.Fatal(err)
	}
	a.Boot = "new-boot-00000002"
	k.link = Link{}
	k.routes = nil
	k.rules = nil
	before := len(k.calls)
	if err := a.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if len(k.calls) != before || !k.ipvlanModule {
		t.Fatal("previous-boot ownership unloaded module")
	}
	if err := a.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := a.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if !k.ipvlanModule {
		t.Fatal("preexisting new-boot module removed")
	}
}

func TestIPvlanParentUpperGuard(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ordinary"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ipvlanParentFree(entries) {
		t.Fatal("parent without upper refused")
	}
	// Refuse every upper relation, including a dangling cross-namespace target.
	if err := os.Symlink("missing", filepath.Join(dir, "upper_foreign")); err != nil {
		t.Fatal(err)
	}
	entries, err = os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ipvlanParentFree(entries) {
		t.Fatal("foreign upper relationship accepted")
	}
}
