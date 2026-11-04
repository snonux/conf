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
	hostfirewall.Stage(hostfirewall.Profile{Interface: "enp0s5", EnableWireGuard: true, WireGuardPort: 56709})
}

// OptsMigrationStage keeps future source permissions out of routine deployments.
func (Firewall) OptsMigrationStage() TaskOptions { return TaskOptions{Operational()} }

// MigrationStage installs a dormant physical-source migration policy without loading rules.
func (Firewall) MigrationStage() {
	EnsureDir("/etc/f3s-firewall", RootPrivate)
	InstallFile("/etc/f3s-firewall/migration.nft",
		paths.RNodeAsset("source-guard/migration.nft"), RootPrivate,
		WithValidation("/usr/sbin/nft", List("-c", "-f", CandidatePath)))
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
