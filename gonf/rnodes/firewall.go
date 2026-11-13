package rnodes

import (
	"github.com/snonux/conf/gonf/hostfirewall"
	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

// Firewall stages the r-node source guard independently of kube-proxy/flannel.
type Firewall struct{ RequiresRoot }

// OptsStage excludes firewall staging from ordinary parallel deployments.
func (Firewall) OptsStage() TaskOptions { return TaskOptions{Operational()} }

// Stage installs a candidate and guarded firewall activation tools without loading rules.
func (Firewall) Stage() {
	// The policy trusts the lab LAN and individual wg2 identities; its old
	// 192.168.1.x trust was removed after i24 retired those addresses (task
	// 074). The Cluster NodePorts admit only the r-nodes and, for the
	// registry, rocky (task o74), since a non-hosting node masquerades them
	// past the receiver NetworkPolicies. Routine staging must retain this policy.
	hostfirewall.Stage(hostfirewall.Profile{Interface: "enp0s5", EnableWireGuard: true, WireGuardPort: 56709,
		Rules: paths.RNodeAsset("source-guard/f3s-guard.nft")})
}

// OptsRpcbindOff keeps the NFS-adjacent cleanup an explicitly named operation.
func (Firewall) OptsRpcbindOff() TaskOptions { return TaskOptions{Operational()} }

// RpcbindOff stops and masks rpcbind only after checking every NFS mount uses NFSv4.
func (Firewall) RpcbindOff() {
	// No remount or NFS/stunnel restart: first prove the deployed mount and
	// reject an unexpected NFSv3 consumer. Roll out on one node at a time.
	Command("bash", List("-euc", `findmnt -rn --mountpoint /data/nfs/k3svolumes -o FSTYPE | grep -Fxq nfs4
if findmnt -rn -t nfs,nfs4 -o FSTYPE | grep -Fxq nfs; then
    echo 'NFSv3 mount present; refusing to stop rpcbind' >&2
    exit 1
fi
systemctl mask --now rpcbind.service rpcbind.socket`))
}
