package recovery

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergePreservesLatestUnknownFieldsAndFallback(t *testing.T) {
	latest := `{"network_config":{"mode":"target"},"auth":{"password":"NEW_PRIVATE_AUTH"},"usb":{"enabled":false},"video":{"quality":90},"unknown":[{"keep":1234567890123456789}]}`
	for _, test := range []struct {
		name, latest string
		fallback     bool
	}{{"valid", latest, false}, {"truncated", `{"password":"PRIVATE_BROKEN`, true}, {"duplicate", `{"network_config":{},"auth":1,"auth":2}`, true}} {
		t.Run(test.name, func(t *testing.T) {
			saved, _ := Network([]byte(original))
			result, fallback, err := Merge([]byte(test.latest), []byte(original), saved)
			if err != nil {
				t.Fatal(err)
			}
			if fallback != test.fallback {
				t.Fatal("incorrect fallback status")
			}
			var got map[string]json.RawMessage
			if err := json.Unmarshal(result, &got); err != nil {
				t.Fatal(err)
			}
			baseline := test.latest
			if fallback {
				baseline = original
			}
			var want map[string]json.RawMessage
			if err := json.Unmarshal([]byte(baseline), &want); err != nil {
				t.Fatal(err)
			}
			for key, value := range want {
				if key == "network_config" {
					continue
				}
				a, _ := json.Marshal(value)
				b, _ := json.Marshal(got[key])
				if string(a) != string(b) {
					t.Fatalf("unrelated %s changed", key)
				}
			}
		})
	}
}
func TestMalformedJSONAndPrivacy(t *testing.T) {
	for _, data := range []string{`null`, `[]`, `{}`, `{"network_config":null}`, `{"network_config":{}} {}`, `{"network_config":{},"network_config":{"private":"PRIVATE_VALUE"}}`} {
		if _, err := Network([]byte(data)); err == nil {
			t.Fatalf("invalid document accepted %s", data)
		} else if strings.Contains(err.Error(), "PRIVATE_VALUE") {
			t.Fatal("private value leaked")
		}
	}
	if _, _, err := Merge([]byte(`bad`), []byte(`{"PRIVATE_VALUE`), []byte(`{}`)); err == nil || strings.Contains(err.Error(), "PRIVATE_VALUE") {
		t.Fatal("invalid fallback not safely rejected")
	}
	a, err := Digest([]byte(`{"a":{"y":2,"x":1},"z":1234567890123456789}`))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Digest([]byte(`{"z":1234567890123456789,"a":{"x":1,"y":2}}`))
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatal("semantic digest depends on JSON field order")
	}
}
func TestAtomicInterruptionAndPrivateFiles(t *testing.T) {
	for _, point := range []string{"written", "synced", "renamed"} {
		t.Run(point, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "state.json")
			if err := os.WriteFile(path, []byte(`{"old":1}`), 0600); err != nil {
				t.Fatal(err)
			}
			err := atomicFile(path, []byte(`{"new":2}`), func(stage string) error {
				if stage == point {
					return errors.New("injected interruption")
				}
				return nil
			})
			if err == nil {
				t.Fatal("failure injection missed")
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want := `{"old":1}`
			if point == "renamed" {
				want = `{"new":2}`
			}
			if string(data) != want {
				t.Fatal("partial replacement visible")
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0600 {
				t.Fatal("private permissions lost")
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 {
				t.Fatal("private temporary file leaked")
			}
		})
	}
}
func TestStoreRejectsSymlinksPermissionsAndCorruption(t *testing.T) {
	f := setup(t)
	arm(t, f.engine)
	path := filepath.Join(f.engine.Store.Dir, "state.json")
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := f.engine.Store.Load(); err == nil {
		t.Fatal("public state accepted")
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(targetPath, before, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(targetPath, path); err != nil {
		t.Fatal(err)
	}
	if _, err := f.engine.Store.Load(); err == nil {
		t.Fatal("state symlink accepted")
	}
	if err := Atomic(path, []byte("secret")); err == nil {
		t.Fatal("atomic write followed symlink")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"PRIVATE_AUTH_SENTINEL`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := f.engine.PreApp(); err == nil || strings.Contains(err.Error(), "PRIVATE_AUTH_SENTINEL") {
		t.Fatal("corrupt journal not rejected privately")
	}
	lock := filepath.Join(f.engine.Store.Dir, "lock")
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(targetPath, lock); err != nil {
		t.Fatal(err)
	}
	if err := f.engine.Store.Locked(func() error { t.Fatal("symlink lock entered"); return nil }); err == nil {
		t.Fatal("lock symlink accepted")
	}
}

func TestLockPermissionsAndCorruptStateContract(t *testing.T) {
	f := setup(t)
	arm(t, f.engine)
	lock := filepath.Join(f.engine.Store.Dir, "lock")
	if err := os.Chmod(lock, 0644); err != nil {
		t.Fatal(err)
	}
	if err := f.engine.Store.Locked(func() error { t.Fatal("public lock admitted"); return nil }); err == nil {
		t.Fatal("public lock accepted")
	}
	state := mustState(t, f.engine)
	for _, change := range []string{"version", "phase", "digest", "allowance", "saved"} {
		t.Run(change, func(t *testing.T) {
			bad := state
			switch change {
			case "version":
				bad.Version = 99
			case "phase":
				bad.Phase = "unknown"
			case "digest":
				bad.TargetDigest = "bad"
			case "allowance":
				bad.Phase = "trial"
			case "saved":
				bad.Saved = []byte(`{"mode":"different"}`)
			}
			if err := bad.Validate(); err == nil {
				t.Fatal("corrupt durable contract accepted")
			}
		})
	}
}
