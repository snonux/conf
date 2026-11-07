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
func MergeHostsIn(rows []string, directory string) {
	EnsureDir(directory, RootOwned)
	EnsureDir("/usr/local/etc", RootOwned)
	helper := InstallFile(filepath.Join(directory, filepath.Base(hostsHelper)), paths.GonfAsset("fleetfiles", "merge-hosts"), RootExec)
	candidate := File(hostsCandidate, WithContent(strings.Join(rows, "\n")+"\n"), RootOwned)
	Command(endpointfiles.Helper, List("apply", "hosts"), DependsOn(helper, candidate),
		Unless(hostsHelper, List("check", hostsCandidate)))
}
