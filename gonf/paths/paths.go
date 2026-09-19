// Package paths holds filesystem roots used by configuration tasks.
package paths

import . "github.com/snonux/gonf/api"

// Repo roots used when declaring resources.
var (
	Conf      = Home("git/conf")
	Frontends = Home("git/conf/frontends")
)
