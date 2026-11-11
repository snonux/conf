// Package fleetfiles installs the shared fleet block without owning local rows.
package fleetfiles

import (
	"path/filepath"
	"strings"

	"github.com/snonux/conf/gonf/endpointfiles"
	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

const (
	hostsHelper    = "/usr/local/sbin/gonf-fleet-hosts"
	hostsCandidate = "/usr/local/etc/gonf-fleet-hosts"
)

// MergeHosts preserves unrelated aliases and replaces conflicting fleet names.
// The candidate is inert until the existing owning task invokes the merger.
func MergeHosts(rows []string) {
	MergeHostsIn(rows, "/usr/local/sbin")
}

// MergeHostsIn installs into the real directory behind a platform's sbin alias.
// Runtime paths keep using that alias, while file validation avoids symlinks.
// The endpoint apply helper (installed here too, since task a74 retired the
// operational task that used to install it) runs the merger only when the
// candidate's rows are not already in /etc/hosts.
func MergeHostsIn(rows []string, directory string) {
	EnsureDir(directory, RootOwned)
	EnsureDir("/usr/local/etc", RootOwned)
	merger := InstallFile(filepath.Join(directory, filepath.Base(hostsHelper)), paths.GonfAsset("fleetfiles", "merge-hosts"), RootExec)
	apply := endpointfiles.InstallHelper(directory)
	candidate := File(hostsCandidate, WithContent(strings.Join(rows, "\n")+"\n"), RootOwned)
	Command(endpointfiles.Helper, List("apply", "hosts"), DependsOn(merger, apply, candidate),
		Unless(hostsHelper, List("check", hostsCandidate)))
}
