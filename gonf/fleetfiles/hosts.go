// Package fleetfiles installs the shared fleet block without owning local rows.
package fleetfiles

import (
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
	EnsureDir("/usr/local/sbin", RootOwned)
	EnsureDir("/usr/local/etc", RootOwned)
	helper := InstallFile(hostsHelper, paths.GonfAsset("fleetfiles", "merge-hosts"), RootExec)
	candidate := File(hostsCandidate, WithContent(strings.Join(rows, "\n")+"\n"), RootOwned)
	Command(endpointfiles.Helper, List("apply", "hosts"), DependsOn(helper, candidate),
		Unless(hostsHelper, List("check", hostsCandidate)))
}
