package freebsd

import . "github.com/snonux/gonf/api"

// GuestHosts adopts only the standalone guest's host database, not its baseline.
type GuestHosts struct{ RequiresRoot }

// Config merges the fleet while preserving this guest's local rows.
func (GuestHosts) Config() { Base{}.Hosts() }
