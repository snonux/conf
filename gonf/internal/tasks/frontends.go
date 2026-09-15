// Package tasks declares the configuration tasks for the conf repository.
package tasks

import (
	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"
)

// Frontends holds the frontend host tasks, including the unattended-upgrade
// deployment per frontends/docs/unattended-upgrades.implementation.md
// (methods in unattended.go). Every unattended task needs root on the
// OpenBSD frontends (Privileged companions); the per-host cron schedules
// are gated with the serializable WhenHostnameContains plan recipes,
// evaluated on the destination host at apply time. Note: the frontends
// aggregate expands via Matching over LOCALLY activated tasks, so
// hostname-gated cron tasks are absent from aggregate pushes recorded on
// the controller — push the task names explicitly to deploy everything.
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
