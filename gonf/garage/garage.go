// Package garage declares the Garage configuration task for the FreeBSD
// storage nodes.
package garage

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

const (
	configPath = "/usr/local/etc/garage.toml"
	template   = "etc/garage.toml.tmpl"
)

// Node is a Garage node's own address (host data, see gonf/cluster):
// RPCPublicAddr is the RPC address other nodes reach it on, with its port;
// AdminAPIBindAddr is its private admin/metrics listener, also with its port.
type Node struct {
	RPCPublicAddr    string
	AdminAPIBindAddr string
}

// tomlData carries the per-node values rendered into garage.toml. It stays
// JSON-compatible because WithTemplateData serializes it with the plan for
// destination-side rendering.
type tomlData struct {
	ReplicationFactor            int
	ConsistencyMode              string
	MetadataDir                  string
	DataDir                      string
	DataCapacity                 string
	MetadataAutoSnapshotInterval string
	DBEngine                     string
	RPCSecret                    string
	RPCBindAddr                  string
	RPCPublicAddr                string
	S3APIBindAddr                string
	S3Region                     string
	RootDomain                   string
	AdminAPIBindAddr             string
	MetricsToken                 string
}

// Deployment carries the root-only Garage configuration task. The config is
// written by the privileged plan chunk, so secret material is never staged in
// a login-owned temporary file on the destination.
//
// It deploys configuration, not a Garage node: installing the garage
// package, creating its garage group, the metadata/data storage
// (/var/db/garage) and the cluster layout (garage layout assign/apply) are
// first-host provisioning steps outside gonf (see f3s/garage/Justfile and
// the f3s Garage docs). On a host without them the config install fails on
// the missing group, or the restart fails, instead of provisioning.
type Deployment struct {
	RequiresRoot
}

// Config renders private admin/metrics settings and installs Garage TOML
// on f0, f1, and f2 (config only;
// Garage must already be installed and provisioned).
//
// Config installs each node's Garage configuration and only restarts Garage
// when its managed configuration changed. Service still converges to enabled
// and running on every apply.
//
// The logical reference paths.GarageSecret("rpc_secret") resolves through
// the provider cmd/gonf/main.go configures (api.SetSecretProvider): since
// task ze2 the foostore/KeePass entry Infra/garage-rpc (Password field),
// through secret.NewFallback. A locked or unavailable vault fails the plan
// loudly rather than falling back to gonf/secrets/garage/rpc_secret, which
// is kept only as a legacy copy for an unmapped reference (see
// gonf/secrets/README.md, "Typed provider references and cutover policy").
// The vault value has no trailing newline; trimFinalNewline keeps the
// rendered template_data byte-identical for either source.
func (Deployment) Config() {
	secret := trimFinalNewline(MustSecret(paths.GarageSecret("rpc_secret")))
	metrics := strings.TrimRight(MustSecret(MetricsTokenSecret()), "\r\n")
	if !validMetricsToken(metrics) {
		Refuse("File", configPath, fmt.Errorf("Garage metrics token must be nonempty printable ASCII without whitespace"))
		return
	}
	EachHost(func(node Node) {
		if !validAdminBind(node.AdminAPIBindAddr) {
			Refuse("File", configPath, fmt.Errorf("Garage admin listener requires an explicit private or loopback IP and numeric port"))
			return
		}
		config := InstallFile(configPath, paths.GarageAsset(template),
			Perm(0o640, "root:garage"),
			WithTemplateData(defaultConfig(secret, metrics, node)), WithSensitive,
		)
		Service("garage", WithRestart, OnChange(config))
	})
}

// MetricsTokenSecret is the logical reference shared with Prometheus.
func MetricsTokenSecret() string { return paths.GarageSecret("metrics_token") }

func defaultConfig(secret, metrics string, node Node) tomlData {
	return tomlData{
		ReplicationFactor:            3,
		ConsistencyMode:              "consistent",
		MetadataDir:                  "/var/db/garage/meta",
		DataDir:                      "/var/db/garage/data",
		DataCapacity:                 "23G",
		MetadataAutoSnapshotInterval: "6h",
		DBEngine:                     "lmdb",
		RPCSecret:                    strconv.Quote(secret),
		RPCBindAddr:                  "0.0.0.0:3901",
		RPCPublicAddr:                node.RPCPublicAddr,
		S3APIBindAddr:                "0.0.0.0:3900",
		S3Region:                     "garage",
		RootDomain:                   ".garage.f3s.buetow.org",
		AdminAPIBindAddr:             node.AdminAPIBindAddr,
		MetricsToken:                 strconv.Quote(metrics),
	}
}

// trimFinalNewline preserves MustSecret's byte-preserving contract while
// matching Rex's chomp before it interpolated the one-line RPC secret.
func trimFinalNewline(secret string) string {
	return strings.TrimSuffix(secret, "\n")
}

func validMetricsToken(token string) bool {
	return token != "" && strings.IndexFunc(token, func(r rune) bool { return r < 33 || r > 126 }) < 0
}

func validAdminBind(address string) bool {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	n, err := strconv.Atoi(port)
	return err == nil && n > 0 && n <= 65535 && ip != nil && (ip.IsPrivate() || ip.IsLoopback())
}
