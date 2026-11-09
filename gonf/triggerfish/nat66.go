package triggerfish

import (
	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

// nat66EgressCommand keeps the NAT66 egress table current (assets/nat66-egress).
const nat66EgressCommand = "/etc/gonf/nat66-egress"

// NAT66 maintains the runtime dependency of the o34 wifi-ipv6-nat66 profile.
type NAT66 struct{ RequiresRoot }

// OptsEgress keeps router cron changes out of routine deployment.
func (NAT66) OptsEgress() TaskOptions { return TaskOptions{Operational()} }

// Egress installs the per-minute job that points the NAT66 table at the uplink's SLAAC address.
func (NAT66) Egress() {
	// The helper acts only while the loaded rules use its table and only under
	// the rollback journal lock, so it can be installed before the profile and
	// stays harmless across every rollback; router-rollback refuses to arm the
	// NAT66 profile without it and retires an orphaned table on activation.
	EnsureDir("/etc/gonf", RootPrivate)
	script := InstallFile(nat66EgressCommand, paths.GonfAsset("triggerfish", "nat66-egress"), RootExec,
		WithValidation("/bin/ksh", List("-n", CandidatePath)))
	CronAt("triggerfish-nat66-egress", "* * * * *", nat66EgressCommand, DependsOn(script))
}
