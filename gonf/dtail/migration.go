package dtail

import (
	. "github.com/snonux/gonf/api"
)

// OptsRetireCandidate keeps the one-shot out of aggregates.
func (Deployment) OptsRetireCandidate() TaskOptions { return TaskOptions{Operational()} }

// RetireCandidate removes the inactive triggerfish dserver bind candidate.
// The retired MigrationCandidate staged it next to the live dtail.json (under
// /usr/local on FreeBSD; never on the OpenBSD frontends); dserver only reads
// the live file. This one-shot (task 974) is deleted in a later commit.
func (Deployment) RetireCandidate() {
	EachHost(func(host ServerHost) {
		if host.Platform == "openbsd" {
			return
		}
		path := "/etc/dserver/dtail.json.triggerfish-candidate"
		if host.Platform == "freebsd" {
			path = "/usr/local" + path
		}
		NoFile(path)
	})
}
