// Package tasks declares the configuration tasks for the conf repository.
package tasks

import (
	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"
)

// Frontends holds the frontend host tasks, including the unattended-upgrade
// deployment per frontends/docs/unattended-upgrades.implementation.md
// (methods in unattended.go). Every unattended task needs root on the
// OpenBSD frontends (Privileged companions). The per-host cron schedule is
// selected inside the cron task body with the WhenHostname recipe, which is
// plan-serializable and evaluated on the destination — so the task is safe
// for aggregate pushes and `gonf fleet` runs (record once, apply per host).
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
