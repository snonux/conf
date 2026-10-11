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
	// WireGuardMeshGenerator is the wireguardmeshgenerator checkout, run in
	// place by the opt-in wireguard_mesh_install task (gonf/wireguard).
	// Override: GONF_WIREGUARDMESHGENERATOR_ROOT.
	WireGuardMeshGenerator = checkoutRoot("GONF_WIREGUARDMESHGENERATOR_ROOT", "wireguardmeshgenerator")
)

// checkoutRoot returns the controller checkout root named by the environment
// variable env, or ~/git/<name> when env is unset or empty. An override must
// be an absolute path: a relative one would silently depend on the recipe's
// working directory (gonf.sh changes into ./gonf), so it fails at startup.
//
// The roots are package-level variables, so this runs while the recipe's
// packages initialize, before any task is registered. A relative override is
// input, not a programmer bug, so it is reported as a gonf declaration error
// (Refuse; Checkout[env] names the variable) instead of panicking:
// gonf's CLI then refuses every invocation, -list included, printing the
// error and exiting 1, exactly as the former startup panic failed them but
// without a stack trace. The returned root is then "" and inert, since
// nothing is recorded or applied from it.
func checkoutRoot(env, name string) string {
	root := os.Getenv(env)
	if root == "" {
		return Home("git", name)
	}
	if !filepath.IsAbs(root) {
		Refuse("Checkout", env,
			fmt.Errorf("%s=%q must be an absolute path to the %s checkout", env, root, name))
		return ""
	}
	return filepath.Clean(root)
}
