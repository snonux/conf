//go:build linux

package recovery

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestDeferredTerminalNegativeDiagnosticCodes(t *testing.T) {
	for _, kind := range []string{"collector", "validator"} {
		t.Run(kind, func(t *testing.T) {
			f := setupDeferred(t)
			f.begin(t)
			if kind == "collector" {
				f.ops.collect = func(context.Context) (ParentReadinessSnapshot, error) {
					return ParentReadinessSnapshot{}, errors.New("PRIVATE_COLLECTOR_SENTINEL")
				}
			} else {
				f.snapshot.Parent.Up = false
			}
			if continueDeferredManagement(context.Background(), f.adapter, f.ops, f.waiter) == nil {
				t.Fatal("negative continuation accepted")
			}
			data, err := ReadPrivate(filepath.Join(f.adapter.Store.Dir, deferredResultFile))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), "PRIVATE_COLLECTOR_SENTINEL") {
				t.Fatal("raw error leaked")
			}
			var result deferredResult
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatal(err)
			}
			expected := "collector-refused"
			if kind == "validator" {
				expected = "validator-parent"
			}
			if result.Outcome != "failed" || result.Diagnostic == nil || result.Diagnostic.Reason != "wait-timeout" || result.Diagnostic.Readiness != expected || result.Diagnostic.Completed != int64(time.Second)+deferredWaitNS || result.Diagnostic.Persistence != "first-write" || len(f.kernel.calls) != 0 {
				t.Fatal("negative receipt lost fixed binding or changed network")
			}
		})
	}
}

func TestDeferredTerminalReceiptRetryAndForeignRefusal(t *testing.T) {
	f := setupDeferred(t)
	f.begin(t)
	var claim DeferredClaim
	if err := readPrivate(filepath.Join(f.adapter.Store.Dir, deferredClaimFile), &claim); err != nil {
		t.Fatal(err)
	}
	diagnostic := deferredDiagnostic{Reason: "wait-timeout", Readiness: "collector-process-read", Persistence: "first-write"}
	calls := 0
	save := func(store Store, name string, value any) error {
		calls++
		if calls == 1 {
			return errors.New("PRIVATE_STORAGE_SENTINEL")
		}
		return deferredSave(store, name, value)
	}
	if err := persistDeferredFailureWith(f.adapter, f.ops, claim, f.waiter, diagnostic, save); err != nil {
		t.Fatal(err)
	}
	var result deferredResult
	if err := readPrivate(filepath.Join(f.adapter.Store.Dir, deferredResultFile), &result); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || result.Diagnostic.Persistence != "retry-after-write-failure" {
		t.Fatal("bounded persistence retry not retained")
	}
	if err := persistDeferredFailureWith(f.adapter, f.ops, claim, f.waiter, diagnostic, save); err == nil || calls != 2 {
		t.Fatal("existing receipt overwritten")
	}
}

func TestDeferredBusyLockRetainsIndependentTerminalDiagnostic(t *testing.T) {
	f := setupDeferred(t)
	f.begin(t)
	file, err := os.OpenFile(filepath.Join(f.adapter.Store.Dir, "lock"), os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN) }()
	f.ops.collect = func(context.Context) (ParentReadinessSnapshot, error) {
		return ParentReadinessSnapshot{}, errors.New("PRIVATE_COLLECTOR_SENTINEL")
	}
	f.ops.pause = func(context.Context) error { f.tick += deferredWaitNS; return nil }
	started := time.Now()
	err = continueDeferredManagement(context.Background(), f.adapter, f.ops, f.waiter)
	if err == nil || !strings.Contains(err.Error(), "deferred terminal persistence failed") || time.Since(started) > 2*time.Second {
		t.Fatal("busy lock terminal failure unbounded or unreported")
	}
	var diagnostic deferredTerminalDiagnostic
	p := filepath.Join(f.adapter.Store.Dir, deferredDiagnosticFile)
	if err := readPrivate(p, &diagnostic); err != nil {
		t.Fatal("independent terminal diagnostic absent", err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	raw := info.Sys().(*syscall.Stat_t)
	if info.Mode().Perm() != 0600 || raw.Nlink != 1 || diagnostic.Kind != "waiter-terminal-diagnostic-v1" || diagnostic.ResultPersistenceAcknowledged || diagnostic.Waiter != f.waiter || diagnostic.Diagnostic.Reason != "wait-timeout" || diagnostic.Diagnostic.Persistence != "result-persistence-failed" || len(f.kernel.calls) != 0 {
		t.Fatal("independent diagnostic provenance or scope wrong")
	}
	if _, err := os.Lstat(filepath.Join(f.adapter.Store.Dir, deferredResultFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("busy lock invented primary result")
	}
}

func TestDeferredIndependentDiagnosticRefusesForeignBindingAndPublication(t *testing.T) {
	for _, kind := range []string{"claim", "waiter", "directory", "existing", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			f := setupDeferred(t)
			f.begin(t)
			directory, err := pinDeferredDiagnostic(f.adapter.Store)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = directory.Close() }()
			var claim DeferredClaim
			if err := readPrivate(filepath.Join(f.adapter.Store.Dir, deferredClaimFile), &claim); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(f.adapter.Store.Dir, deferredDiagnosticFile)
			switch kind {
			case "claim":
				other := claim
				other.Generation = genB
				privateJSON(t, filepath.Join(f.adapter.Store.Dir, deferredClaimFile), other)
			case "waiter":
				other := f.waiter
				other.PID++
				privateJSON(t, filepath.Join(f.adapter.Store.Dir, deferredWaiterFile), other)
			case "directory":
				if err := os.Rename(f.adapter.Store.Dir, f.adapter.Store.Dir+"-inert"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(f.adapter.Store.Dir, 0700); err != nil {
					t.Fatal(err)
				}
			case "existing":
				privateJSON(t, output, map[string]string{"foreign": "preserve"})
			case "symlink":
				if err := os.Symlink("foreign", output); err != nil {
					t.Fatal(err)
				}
			}
			if err := saveDeferredDiagnostic(directory, f.adapter.Store, f.ops, claim, f.waiter, deferredDiagnostic{Reason: "admission", Readiness: "validated"}); err == nil {
				t.Fatal("foreign terminal diagnostic publication accepted")
			}
		})
	}
}

func TestDeferredFreshClockAtBootstrapCannotBorrowReadinessTime(t *testing.T) {
	f := setupDeferred(t)
	f.begin(t)
	collects, finalReads := 0, 0
	f.ops.collect = func(context.Context) (ParentReadinessSnapshot, error) { collects++; return f.snapshot, nil }
	f.ops.now = func() (int64, error) {
		if collects == 3 {
			finalReads++
			if finalReads >= 2 {
				return int64(time.Second) + deferredWaitNS, nil
			}
		}
		return f.tick, nil
	}
	if continueDeferredManagement(context.Background(), f.adapter, f.ops, f.waiter) == nil || len(f.kernel.calls) != 0 {
		t.Fatal("fresh clock past original readiness deadline reached Ensure")
	}
	var result deferredResult
	if err := readPrivate(filepath.Join(f.adapter.Store.Dir, deferredResultFile), &result); err != nil || result.Outcome != "failed" || result.Diagnostic.Reason != "wait-timeout" {
		t.Fatal("deadline admission negative receipt absent")
	}
}

func TestDeferredDiagnosticHistoryRefusesFreshAllowance(t *testing.T) {
	f := setupDeferred(t)
	path := filepath.Join(f.adapter.Store.Dir, "canary-active.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	privateJSON(t, filepath.Join(f.adapter.Store.Dir, deferredDiagnosticFile), map[string]string{"foreign": "preserve"})
	if beginDeferredManagement(context.Background(), f.adapter, f.ops, f.watch(t), f.wait(t)) == nil {
		t.Fatal("diagnostic history borrowed a fresh allowance")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) || len(f.starts) != 0 || len(f.kernel.calls) != 0 {
		t.Fatal("foreign diagnostic consumed allowance or changed resources")
	}
}
