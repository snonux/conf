// Package garage declares the Garage configuration task for the FreeBSD
// storage nodes.
package garage

import (
	"strconv"
	"strings"

	"codeberg.org/snonux/conf/gonf/cluster"
	"codeberg.org/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"
)

const (
	configPath = "/usr/local/etc/garage.toml"
	template   = "etc/garage.toml.tmpl"
)

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
}

// Deployment carries the root-only Garage configuration task. The config is
// written by the privileged plan chunk, so secret material is never staged in
// a login-owned temporary file on the destination.
type Deployment struct {
	RequiresRoot
}

// DescConfig returns the description for the Garage configuration deployment.
func (Deployment) DescConfig() string {
	return "Render and install Garage TOML on f0, f1, and f2"
}

// Config installs each node's Garage configuration and only restarts Garage
// when its managed configuration changed. Service still converges to enabled
// and running on every apply.
func (Deployment) Config() {
	secret := trimFinalNewline(MustSecret(paths.GarageSecret("rpc_secret")))
	for _, host := range ClusterHosts() {
		rpcPublicAddr := MustHostValue[string](host, cluster.ValueGarageRPCPublicAddr)
		WhenHostname(host, func() {
			config := InstallFile(configPath, paths.GarageAsset(template),
				WithMode(0o640), WithOwner("root"), WithGroup("garage"),
				WithTemplateData(defaultConfig(secret, rpcPublicAddr)),
			)
			Service("garage", WithRestart, OnChange(config))
		})
	}
}

func defaultConfig(secret, rpcPublicAddr string) tomlData {
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
		RPCPublicAddr:                rpcPublicAddr,
		S3APIBindAddr:                "0.0.0.0:3900",
		S3Region:                     "garage",
		RootDomain:                   ".garage.f3s.buetow.org",
		AdminAPIBindAddr:             "0.0.0.0:3903",
	}
}

// trimFinalNewline preserves MustSecret's byte-preserving contract while
// matching Rex's chomp before it interpolated the one-line RPC secret.
func trimFinalNewline(secret string) string {
	return strings.TrimSuffix(secret, "\n")
}
