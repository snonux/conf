package freebsd

import (
	. "github.com/snonux/gonf/api"
)

// candidateSuffix names the inactive copies the retired service-migration
// MigrationCandidate tasks staged next to the live files. Nothing reads them:
// the daemons use the live paths and sshd only the exact f3sctl key file.
// The RetireCandidate one-shots below remove them (task 974) and are deleted
// in a later commit together with this file.
const candidateSuffix = ".triggerfish-candidate"

// OptsRetireCandidate keeps the one-shot out of aggregates.
func (Apcupsd) OptsRetireCandidate() TaskOptions { return TaskOptions{Operational()} }

// RetireCandidate removes the inactive triggerfish UPS configuration candidate.
func (Apcupsd) RetireCandidate() { NoFile(apcupsdConf + candidateSuffix) }

// OptsRetireCandidate keeps the one-shot out of aggregates.
func (ShellyFans) OptsRetireCandidate() TaskOptions { return TaskOptions{Operational()} }

// RetireCandidate removes the inactive triggerfish fan helper and rc.conf.d candidates.
func (ShellyFans) RetireCandidate() {
	NoFile(List("/usr/local/etc/rc.conf.d/shellyfans"+candidateSuffix, shellyFansScript+candidateSuffix))
}

// OptsRetireCandidate keeps the one-shot out of aggregates.
func (F3sctl) OptsRetireCandidate() TaskOptions { return TaskOptions{Operational()} }

// RetireCandidate removes the inactive triggerfish f3sctl key candidate.
func (F3sctl) RetireCandidate() { NoFile(f3sctlKeyFile + candidateSuffix) }
