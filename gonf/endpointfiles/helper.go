// Package endpointfiles installs the plain apply helper through which the
// fleet's owners publish /etc/hosts and the Pi controller's f3sctl.json, and
// (one-shot, task a74) retires the migration-era watchdog and boot hooks.
package endpointfiles

import (
	"path/filepath"

	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

// Helper is the runtime path the owners invoke. It keeps its migration-era
// name: OpenBSD's /etc/rc and the minute cron called it until the
// *_endpoints_retire tasks removed them, and the helper still answers those
// calls with a no-op.
const Helper = "/usr/local/sbin/gonf-endpoint-migration"

// InstallHelper installs the apply helper into directory: /usr/local/sbin,
// or the real directory behind a platform's sbin alias (Fedora's
// /usr/local/sbin -> bin). Runtime paths keep using Helper, while gonf's
// file resource writes into the real directory, never through the symlink.
// The returned dependency orders an owner's apply command after the helper.
func InstallHelper(directory string) Dependency {
	EnsureDir(directory, RootOwned)
	return InstallFile(filepath.Join(directory, filepath.Base(Helper)), paths.GonfAsset("endpointfiles", "apply"), RootExec,
		WithValidation("/bin/sh", List("-n", CandidatePath)))
}
