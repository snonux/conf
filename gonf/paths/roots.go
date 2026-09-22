package paths

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/snonux/gonf/api"
)

// Controller checkouts outside this repository that recipes read source
// files from. Like Conf they are controller paths, resolved when a plan is
// recorded; they never name a path on a destination host.
var (
	// Shuriken is the shuriken.sh checkout (Gogios' check_shuriken_age
	// plugin). Override: GONF_SHURIKEN_ROOT.
	Shuriken = checkoutRoot("GONF_SHURIKEN_ROOT", "shuriken.sh")
	// Foostats is the foostats checkout (preferred over the in-repo copies
	// under frontends/scripts). Override: GONF_FOOSTATS_ROOT.
	Foostats = checkoutRoot("GONF_FOOSTATS_ROOT", "foostats")
)

// checkoutRoot returns the controller checkout root named by the environment
// variable env, or ~/git/<name> when env is unset or empty. An override must
// be an absolute path: a relative one would silently depend on the recipe's
// working directory (gonf.sh changes into ./gonf), so it fails at startup.
func checkoutRoot(env, name string) string {
	root := os.Getenv(env)
	if root == "" {
		return Home("git", name)
	}
	if !filepath.IsAbs(root) {
		panic(fmt.Sprintf("%s=%q must be an absolute path to the %s checkout", env, root, name))
	}
	return filepath.Clean(root)
}

// RequireFile returns path when it is a regular file on the controller and
// otherwise fails the recording with an actionable message: what needs the
// file, where it was looked for, and how to provide it (hint). Recipes call
// it inside the task body that uses the file, so a missing optional checkout
// only fails the tasks that need it.
func RequireFile(path, purpose, hint string) string {
	info, err := os.Stat(path)
	if err != nil {
		panic(fmt.Errorf("%s needs %s: %w; %s", purpose, path, err, hint))
	}
	if !info.Mode().IsRegular() {
		panic(fmt.Sprintf("%s needs %s to be a regular file; %s", purpose, path, hint))
	}
	return path
}
