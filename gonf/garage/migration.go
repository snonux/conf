package garage

import (
	"fmt"
	"net"
	"strings"

	"github.com/snonux/conf/gonf/migrationaddress"
	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

// OptsMigrationCandidate keeps candidate preparation out of normal aggregates.
func (Deployment) OptsMigrationCandidate() TaskOptions { return TaskOptions{Operational()} }

// MigrationCandidate stages Garage TOML with its reserved lab RPC address.
// It never restarts Garage or changes the active configuration. The admin
// metrics listener stays on wg0; existing RPC/S3 wildcard listeners stay put.
func (Deployment) MigrationCandidate() {
	EachHost(func(node Node) {
		host, port, err := net.SplitHostPort(node.RPCPublicAddr)
		if err != nil || port != "3901" {
			Refuse("Garage candidate", configPath, fmt.Errorf("expected existing RPC host:3901"))
			return
		}
		address, err := migrationaddress.Lab(host)
		if err != nil {
			Refuse("Garage candidate", configPath, err)
			return
		}
		node.RPCPublicAddr = net.JoinHostPort(address, port)
		if !validAdminBind(node.AdminAPIBindAddr) {
			Refuse("Garage candidate", configPath, fmt.Errorf("invalid existing admin bind"))
			return
		}
		secret := trimFinalNewline(MustSecret(paths.GarageSecret("rpc_secret")))
		metrics := strings.TrimRight(MustSecret(MetricsTokenSecret()), "\r\n")
		if !validMetricsToken(metrics) {
			Refuse("Garage candidate", configPath, fmt.Errorf("invalid metrics token"))
			return
		}
		InstallFile(configPath+".triggerfish-candidate", paths.GarageAsset(template),
			RootPrivate, WithTemplateData(defaultConfig(secret, metrics, node)), WithSensitive)
	})
}
