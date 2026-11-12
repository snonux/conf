package garage

import (
	. "github.com/snonux/gonf/api"
)

// OptsRetireCandidate keeps the one-shot out of aggregates.
func (Deployment) OptsRetireCandidate() TaskOptions { return TaskOptions{Operational()} }

// RetireCandidate removes the inactive triggerfish Garage TOML candidate.
// Garage only reads the live configPath; the retired MigrationCandidate
// staged this copy. This one-shot (task 974) is deleted in a later commit.
func (Deployment) RetireCandidate() { NoFile(configPath + ".triggerfish-candidate") }
