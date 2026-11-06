//go:build linux

package recovery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// deferredDiagnostic contains only reviewed fixed codes and an original-boot
// clock. Collector errors, command output, profiles and config never enter it.
// It is a terminal observation, not a new allowance or readiness acceptance.
type deferredDiagnostic struct {
	Reason      string `json:"reason"`
	Readiness   string `json:"readiness"`
	Completed   int64  `json:"completed_boottime_ns"`
	Persistence string `json:"persistence"`
}

func persistDeferredFailure(a Adapter, ops deferredOps, claim DeferredClaim, self WorkerIdentity, diagnostic deferredDiagnostic) error {
	return persistDeferredFailureWith(a, ops, claim, self, diagnostic, deferredSave)
}

func persistDeferredFailureWith(a Adapter, ops deferredOps, claim DeferredClaim, self WorkerIdentity, diagnostic deferredDiagnostic, save func(Store, string, any) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := deferredLocked(ctx, a.Store, func() error {
		var current DeferredClaim
		if err := readPrivate(filepath.Join(a.Store.Dir, deferredClaimFile), &current); err != nil || current != claim {
			return errors.New("deferred terminal claim changed")
		}
		var waiter WorkerIdentity
		if err := readPrivate(filepath.Join(a.Store.Dir, deferredWaiterFile), &waiter); err != nil || waiter != self {
			return errors.New("deferred terminal waiter changed")
		}
		var active CanaryAuthorization
		if err := readPrivate(filepath.Join(a.Store.Dir, "canary-active.json"), &active); err != nil || active.Hash != claim.Hash || active.Generation != claim.Generation || active.Boot != claim.Boot || active.Deadline != claim.Deadline || active.BootAllowance || active.Confirmed {
			return errors.New("deferred terminal authorization changed")
		}
		tick, err := ops.now()
		if err != nil || tick < 0 {
			diagnostic.Completed = 0
			diagnostic.Reason = "clock-unavailable"
		} else {
			diagnostic.Completed = tick
		}
		result := deferredResult{Claim: claim, Outcome: "failed", Diagnostic: &diagnostic}
		var existing deferredResult
		err = readPrivate(filepath.Join(a.Store.Dir, deferredResultFile), &existing)
		if !errors.Is(err, os.ErrNotExist) {
			return errors.New("deferred terminal result already exists or unreadable")
		}
		if err := save(a.Store, deferredResultFile, result); err == nil {
			return nil
		}
		// A sync error can follow a successful rename. Retry only our exact attempted
		// bytes, or an absent destination; never replace foreign/ensured evidence.
		if err := ctx.Err(); err != nil {
			return err
		}
		expected, encodeErr := json.Marshal(result)
		if encodeErr != nil {
			return errors.New("deferred terminal retry encoding failed")
		}
		raw, readErr := ReadPrivate(filepath.Join(a.Store.Dir, deferredResultFile))
		if readErr == nil && !bytes.Equal(raw, expected) || readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			return errors.New("deferred terminal retry result changed")
		}
		diagnostic.Persistence = "retry-after-write-failure"
		return save(a.Store, deferredResultFile, result)
	})
	if err != nil {
		// The caller must keep this unknown: neither the watchdog nor absence of a
		// result can manufacture a waiter outcome when durable storage failed.
		return errors.New("deferred terminal persistence failed")
	}
	return nil
}

func deferredCollectorCode(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "collector-timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "collector-cancelled"
	}
	switch err.Error() {
	case "unknown process read", "unknown DHCP command line", "unknown DHCP executable":
		return "collector-process-read"
	default:
		return "collector-refused"
	}
}

// The fallback does not pretend a missing primary result proves any outcome.
// It records this exact waiter's known returned error when primary persistence
// was unavailable. No watchdog writes this receipt on the waiter's behalf.
const deferredDiagnosticFile = "management-deferred-diagnostic.json"

type deferredTerminalDiagnostic struct {
	Kind                          string             `json:"kind"`
	Claim                         DeferredClaim      `json:"claim"`
	Waiter                        WorkerIdentity     `json:"waiter"`
	Diagnostic                    deferredDiagnostic `json:"diagnostic"`
	ResultPersistenceAcknowledged bool               `json:"result_persistence_acknowledged"`
}

func pinDeferredDiagnostic(store Store) (*os.File, error) {
	fd, err := syscall.Open(store.Dir, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, errors.New("deferred diagnostic directory unavailable")
	}
	file := os.NewFile(uintptr(fd), "deferred diagnostic directory")
	if err := deferredDiagnosticDirectory(file, store); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func deferredDiagnosticDirectory(file *os.File, store Store) error {
	pinned, err := file.Stat()
	if err != nil || !pinned.IsDir() || pinned.Mode().Perm() != 0700 || !ownedByEffectiveUID(pinned) {
		return errors.New("deferred diagnostic directory invalid")
	}
	raw, ok := pinned.Sys().(*syscall.Stat_t)
	if !ok || raw.Mode&07777 != 0700 || raw.Gid != uint32(os.Getegid()) {
		return errors.New("deferred diagnostic directory ownership invalid")
	}
	current, err := os.Lstat(store.Dir)
	if err != nil || !os.SameFile(pinned, current) || !current.IsDir() || current.Mode().Perm() != 0700 || !ownedByEffectiveUID(current) {
		return errors.New("deferred diagnostic directory changed")
	}
	return nil
}

func saveDeferredDiagnostic(directory *os.File, store Store, ops deferredOps, claim DeferredClaim, self WorkerIdentity, diagnostic deferredDiagnostic) error {
	if err := deferredDiagnosticDirectory(directory, store); err != nil {
		return err
	}
	dir := fmt.Sprintf("/proc/self/fd/%d", directory.Fd())
	check := func() error {
		if err := deferredDiagnosticDirectory(directory, store); err != nil {
			return err
		}
		var current DeferredClaim
		var waiter WorkerIdentity
		var active CanaryAuthorization
		if err := readPrivate(filepath.Join(dir, deferredClaimFile), &current); err != nil || current != claim {
			return errors.New("deferred diagnostic claim changed")
		}
		if err := readPrivate(filepath.Join(dir, deferredWaiterFile), &waiter); err != nil || waiter != self {
			return errors.New("deferred diagnostic waiter changed")
		}
		if err := readPrivate(filepath.Join(dir, "canary-active.json"), &active); err != nil || active.Hash != claim.Hash || active.Generation != claim.Generation || active.Boot != claim.Boot || active.Deadline != claim.Deadline || active.BootAllowance || active.Confirmed {
			return errors.New("deferred diagnostic authorization changed")
		}
		return nil
	}
	if err := check(); err != nil {
		return err
	}
	tick, err := ops.now()
	if err == nil && tick >= 0 {
		diagnostic.Completed = tick
	} else {
		diagnostic.Completed = 0
		diagnostic.Reason = "clock-unavailable"
	}
	diagnostic.Persistence = "result-persistence-failed"
	value := deferredTerminalDiagnostic{Kind: "waiter-terminal-diagnostic-v1", Claim: claim, Waiter: self, Diagnostic: diagnostic, ResultPersistenceAcknowledged: false}
	data, err := json.Marshal(value)
	if err != nil || len(data) > 4096 {
		return errors.New("deferred diagnostic encoding failed")
	}
	// CreateTemp creates root-private O_EXCL files. The pinned FD path keeps all
	// writes inside this exact directory even if its public path is replaced.
	file, err := os.CreateTemp(dir, ".jkrecover-diagnostic-*")
	if err != nil {
		return errors.New("deferred diagnostic storage unavailable")
	}
	defer func() { _ = file.Close(); _ = os.Remove(file.Name()) }()
	if _, err := file.Write(data); err != nil {
		return errors.New("deferred diagnostic write failed")
	}
	if err := file.Sync(); err != nil {
		return errors.New("deferred diagnostic sync failed")
	}
	if err := file.Close(); err != nil {
		return errors.New("deferred diagnostic close failed")
	}
	if err := check(); err != nil {
		return err
	}
	// A hard link publishes the complete fsynced inode without replacing any
	// existing file or symlink. A crash before unlink leaves an honest HOLD layout.
	if err := os.Link(file.Name(), filepath.Join(dir, deferredDiagnosticFile)); err != nil {
		return errors.New("deferred diagnostic publication refused")
	}
	if err := os.Remove(file.Name()); err != nil {
		return errors.New("deferred diagnostic temporary unlink failed")
	}
	if err := directory.Sync(); err != nil {
		return errors.New("deferred diagnostic directory sync failed")
	}
	return check()
}

// Only fixed source-known validator categories are retained, never err.Error().
func deferredValidatorCode(err error) string {
	if err == nil {
		return "validated"
	}
	switch err.Error() {
	case "parent boot, identity or UP not ready":
		return "validator-parent"
	case "DHCP handler provenance not ready":
		return "validator-handler"
	case "stable completed DHCP daemon required":
		return "validator-owner"
	case "DHCP owner changed during collection":
		return "validator-owner-changed"
	case "complete applied DHCP lease required":
		return "validator-lease-schema"
	case "current-boot tmpfs lease provenance required":
		return "validator-lease-provenance"
	case "lease address or resolver state not applied":
		return "validator-address-or-dns"
	case "lease default routes not applied", "lease default route not applied":
		return "validator-default-route"
	case "parent readiness observations changed":
		return "validator-observations-changed"
	default:
		return "validator-refused"
	}
}
