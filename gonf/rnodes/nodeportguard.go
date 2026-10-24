package rnodes

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// NodePortGuard stages the pod-source guard independently of Kubernetes rules.
type NodePortGuard struct{ RequiresRoot }

// OptsStage keeps staging out of ordinary parallel r-node deployments.
func (NodePortGuard) OptsStage() TaskOptions {
	return TaskOptions{Operational()}
}

// Stage installs the NodePort candidate and guarded tools without loading rules.
func (NodePortGuard) Stage() {
	const assets = "nodeport-guard/"
	EnsureDir("/etc/f3s-nodeport-guard", RootPrivate)
	EnsureDir("/usr/local/sbin", RootOwned)
	rules := InstallFile("/etc/f3s-nodeport-guard/candidate.nft",
		paths.RNodeAsset(assets+"candidate.nft"), RootPrivate,
		WithValidation("/usr/sbin/nft", List("-c", "-f", CandidatePath)))
	helper := InstallFile("/usr/local/sbin/f3s-nodeport-guard",
		paths.RNodeAsset(assets+"f3s-nodeport-guard"), RootExec,
		WithValidation("/usr/bin/bash", List("-n", CandidatePath)))
	unit := InstallFile("/etc/systemd/system/f3s-nodeport-guard.service",
		paths.RNodeAsset(assets+"f3s-nodeport-guard.service"), RootOwned)
	Sh("restorecon -RF /etc/f3s-nodeport-guard /usr/local/sbin/f3s-nodeport-guard /etc/systemd/system/f3s-nodeport-guard.service",
		OnChange(rules, helper, unit))
	Sh("systemctl daemon-reload", OnChange(unit))
}
