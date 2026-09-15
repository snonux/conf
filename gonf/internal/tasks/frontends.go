// Package tasks declares the configuration tasks for the conf repository.
package tasks

import (
	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"
)

// Frontends holds the frontend host tasks. Its methods are registered from
// main with the frontends_ prefix.
type Frontends struct{}

// DescPing returns the description shown for the frontends_ping task.
func (Frontends) DescPing() string {
	return "Skeleton task: verify the gonf push pipeline to this host"
}

// Ping is a minimal no-op task used to verify the gonf push pipeline to the
// OpenBSD frontends: the Unless guard makes the command never run.
func (Frontends) Ping() {
	Command("true", nil,
		Unless("true", nil),
		WithName("ping"),
	)
}
