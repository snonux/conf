package netbsd

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// ShellyEndpoints manages the partial power-controller configuration.
// Missing fields retain f3sctl defaults. Existing custom configurations are
// refused rather than replaced: they need a separately reviewed merge.
type ShellyEndpoints struct{ RequiresRoot }

const (
	shellyConfig    = "/usr/local/etc/f3sctl.json"
	shellyCandidate = shellyConfig + ".triggerfish-shelly-candidate"
)

func (ShellyEndpoints) OptsPrepare() TaskOptions { return TaskOptions{Operational()} }
func (ShellyEndpoints) OptsActivate() TaskOptions {
	return TaskOptions{Operational(), Needs(ShellyEndpoints.Prepare)}
}

// Prepare installs an inactive partial configuration without changing callers.
func (ShellyEndpoints) Prepare() {
	EnsureDir("/usr/local/etc", RootOwned)
	EnsureDir("/var/db/gonf-shelly-migration", RootPrivate)
	InstallFile(shellyCandidate, paths.GonfAsset("netbsd", "shelly-endpoints.json"), RootOwned)
}

// Activate adopts verified plug endpoints while preserving all other defaults.
func (ShellyEndpoints) Activate() {
	guard := Command("/bin/sh", List("-ec", `
PATH=/sbin:/bin:/usr/sbin:/usr/bin:/usr/pkg/bin
export PATH
config=/usr/local/etc/f3sctl.json
candidate=$config.triggerfish-shelly-candidate
gate=/var/db/gonf-shelly-migration/devices-approved
test ! -L "$config"
if test -e "$config"; then
    test -f "$config"
    cmp -s "$config" "$candidate"
else
    test -f "$gate" && test ! -L "$gate"
    test "$(stat -f '%u:%Lp' "$gate")" = 0:600
    test "$(cat "$gate")" = 'shellyplugmg3-0892725e366c 10.30.0.28
shellyplugmg3-0892725f8a04 10.30.0.29'
fi`), WithName("shelly-endpoints-ownership-and-acceptance"))
	InstallFile(shellyConfig, paths.GonfAsset("netbsd", "shelly-endpoints.json"),
		RootOwned, DependsOn(guard))
}
