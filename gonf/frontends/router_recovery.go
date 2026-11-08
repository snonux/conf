package frontends

import (
	"fmt"
	"strings"

	"github.com/snonux/conf/gonf/paths"
	"github.com/snonux/conf/gonf/routerrecovery"
	. "github.com/snonux/gonf/api"
)

// RouterRecovery admits the temporary router SSH key on its selected gateway.
type RouterRecovery struct{ RequiresRoot }

// OptsAuthorize keeps temporary recovery access out of routine aggregates.
func (RouterRecovery) OptsAuthorize() TaskOptions { return TaskOptions{Operational()} }

// Authorize admits a restricted key for one loopback reverse-forward listener.
func (RouterRecovery) Authorize() {
	EachHostWith(func(g routerrecovery.Gateway) {
		File("/home/"+g.User+"/.ssh/authorized_keys",
			WithLine(routerRecoveryKey(g)), Perm(0o600, g.User+":"+g.User))
	})
}

// OptsRetire keeps removal of the temporary key a deliberate operation.
func (RouterRecovery) OptsRetire() TaskOptions { return TaskOptions{Operational()} }

// Retire removes only the temporary router key after its tunnel is stopped.
func (RouterRecovery) Retire() {
	EachHostWith(func(g routerrecovery.Gateway) {
		File("/home/"+g.User+"/.ssh/authorized_keys",
			WithoutLine(routerRecoveryKey(g)), Perm(0o600, g.User+":"+g.User))
	})
}

func routerRecoveryKey(g routerrecovery.Gateway) string {
	key := strings.TrimSpace(MustSecret(paths.TriggerfishSecret("router-recovery.pub")))
	// restrict disables agent/X11 forwarding and PTYs; the forced command
	// rejects sessions. Re-enable TCP
	// forwarding only with a fixed reverse listener and an unusable local
	// forwarding destination (the gateway has no TCP service on loopback port 1).
	return fmt.Sprintf(`restrict,port-forwarding,permitlisten="127.0.0.1:%d",permitopen="127.0.0.1:1",command="/usr/bin/false" %s`, g.ListenPort, key)
}
