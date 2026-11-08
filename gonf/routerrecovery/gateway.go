// Package routerrecovery describes the temporary, loopback-only router tunnel.
package routerrecovery

// Gateway is attached to the router and its selected public SSH gateway.
type Gateway struct {
	Address, User       string
	SSHPort, ListenPort uint16
}

const (
	// Directory contains only the temporary router tunnel's owned files.
	Directory = "/etc/gonf/router-recovery"
	// CronName identifies its reconnect job, independently of other maintenance.
	CronName = "triggerfish-recovery-tunnel"
)
