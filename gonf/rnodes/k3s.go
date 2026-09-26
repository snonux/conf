package rnodes

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// K3s manages the k3s server configuration files on r0-r2 (task pk2), NOT
// the k3s install: the binary, its version and the systemd unit (ExecStart
// with --cluster-init on r0, --server https://r0.wg0.wan.buetow.org:6443 on
// r1/r2, --tls-san=rN.wg0.wan.buetow.org) come from get.k3s.io and the
// upgrade procedure (f3s-k3s skill, install.md "Upgrading k3s"). The
// installer leaves config.yaml, config.yaml.d/ and registries.yaml alone, so
// gonf and an upgrade do not fight over them.
//
// The join token (K3S_TOKEN in /etc/systemd/system/k3s.service.env) is a
// secret and is not in config.yaml; gonf does not touch it.
//
// config.yaml.d/50-image-gc.yaml is Maintenance.ImageGC (rnodes_image_gc);
// its "kubelet-arg+" appends, so it coexists with this config.yaml.
//
// Nothing here restarts k3s. k3s reads these files only at start, and the
// three control-plane members deploy in parallel: restarting all of them at
// once drops etcd quorum. A change applies at the next boot (the r-VMs go
// down with the nightly f-host power-off) or by a manual rolling restart,
// one node at a time: systemctl restart k3s on one node, wait until it is
// Ready, etcd has a leader (curl -s 127.0.0.1:2381/metrics | grep
// etcd_server_has_leader) and all pods run, then the next node.
//
// Register with OnCluster(cluster.NameRockyK3s).
type K3s struct {
	RequiresRoot
}

// K3sNode is the per-host k3s server data, attached with WithData in
// cluster.Register(). IP is the node's WireGuard (wg0) address, used for
// both node-ip and advertise-address so all control-plane and etcd traffic
// stays on the mesh.
type K3sNode struct {
	IP string
}

const (
	k3sConfigDir  = "/etc/rancher/k3s"
	k3sConfig     = k3sConfigDir + "/config.yaml"
	k3sRegistries = k3sConfigDir + "/registries.yaml"
	k3sAssetsPath = "k3s/"
)

// Config renders /etc/rancher/k3s/config.yaml (etcd metrics, event-ttl,
// controller-manager bind, wg0 node-ip/advertise-address); no k3s restart.
//
// Config renders config.yaml from f3s/r-nodes/k3s/config.yaml.tmpl, byte for
// byte the hand-made files of 2026-09-25 (f3s blog parts 7 and 8):
//   - etcd-expose-metrics and kube-controller-manager bind-address=0.0.0.0
//     let Prometheus scrape etcd and the controller manager;
//   - event-ttl=1h keeps etcd small;
//   - node-ip / advertise-address pin the node to its wg0 address.
//
// The file carries no header comment on purpose, so the live files adopt
// without a change. Mode 0644 as found (no secrets in it).
func (K3s) Config() {
	EnsureDir(k3sConfigDir, RootOwned)
	EachHost(func(n K3sNode) {
		InstallFile(k3sConfig, paths.RNodeAsset(k3sAssetsPath+"config.yaml.tmpl"),
			WithTemplateData(n), RootOwned)
	})
}

// Registries installs /etc/rancher/k3s/registries.yaml
// (registry.lan.buetow.org:30001 -> http://localhost:30001); no k3s restart.
//
// Registries installs the containerd mirror for the in-cluster registry
// (f3s/registry): images are named registry.lan.buetow.org:30001/..., and
// every node dials its own NodePort over plain HTTP at localhost:30001, so
// pulls need neither DNS nor TLS. Identical on r0-r2.
func (K3s) Registries() {
	EnsureDir(k3sConfigDir, RootOwned)
	InstallFile(k3sRegistries, paths.RNodeAsset(k3sAssetsPath+"registries.yaml"),
		RootOwned)
}
