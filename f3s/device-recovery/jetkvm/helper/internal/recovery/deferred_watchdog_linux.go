//go:build linux

package recovery

import (
	"errors"
	"os"
	"path/filepath"
)

// WorkDeferredCanary uses the same independent deadline/readiness loop as the
// legacy canary. Its sole difference is an exact, read-only empty-state terminal
// path: failed vendor readiness must not require the vendor MAC to retire a
// claim that created nothing. Foreign/unknown/nonzero state never proves empty.
func WorkDeferredCanary(a Adapter, boot, generation string, now func() (int64, error), ready *os.File) error {
	return workDeferredCanary(a, boot, generation, now, ready, nativeDeferredOps(a))
}

func workDeferredCanary(a Adapter, boot, generation string, now func() (int64, error), ready *os.File, ops deferredOps) error {
	var claim DeferredClaim
	if err := readPrivate(filepath.Join(a.Store.Dir, deferredClaimFile), &claim); err != nil {
		return err
	}
	expected := State{Generation: generation, Boot: boot, Deadline: claim.Deadline}
	if err := deferredWatchdogBinding(a, expected, false); err != nil {
		return err
	}
	return workCanary(a, boot, generation, now, ready, func() error { return deferredWatchdogCleanup(a, expected, ops) })
}

func deferredWatchdogCleanup(a Adapter, expected State, ops deferredOps) error {
	// The shared loop has already saved Expired=true under the stable lock. A
	// changed/foreign claim is an error, not permission to clean another canary.
	if err := deferredWatchdogBinding(a, expected, true); err != nil {
		return err
	}
	if err := deferredPrimaryAbsent(a.Store); err != nil {
		return err
	}
	if err := deferredEmpty(a, ops); err == nil {
		// Zero intent AND global modules/child/uppers/table/rule absent. There is
		// nothing to remove or relabel; preserve ownership bytes and parent state.
		return nil
	}
	// Existing typed cleanup retains its parent admission, ownership checks,
	// preservation of pre-existing modules, foreign refusal and retry behavior.
	return a.Cleanup()
}

func deferredWatchdogBinding(a Adapter, expected State, terminal bool) error {
	if err := a.Profile.Validate(); err != nil {
		return err
	}
	var claim DeferredClaim
	if err := readPrivate(filepath.Join(a.Store.Dir, deferredClaimFile), &claim); err != nil {
		return err
	}
	var auth CanaryAuthorization
	if err := readPrivate(filepath.Join(a.Store.Dir, "canary-active.json"), &auth); err != nil {
		return err
	}
	if a.Profile.kind() != "ipvlan" || a.Boot == "" || expected.Boot != a.Boot || expected.Deadline < WindowNS || claim.Hash != a.Profile.Hash() || claim.Generation != expected.Generation || claim.Boot != expected.Boot || claim.Deadline != expected.Deadline || claim.WaitUntil != claim.Deadline-WindowNS+deferredWaitNS || (claim.Attempt != "watchdog-attempted" && claim.Attempt != "waiter-attempted") || auth.Hash != claim.Hash || auth.Generation != claim.Generation || auth.Boot != claim.Boot || auth.Deadline != claim.Deadline || auth.BootAllowance || auth.Confirmed || (terminal && !auth.Expired) {
		return errors.New("deferred watchdog claim changed or not terminal")
	}
	return nil
}
