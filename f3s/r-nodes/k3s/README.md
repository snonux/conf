# k3s node assets

`rnodes_k3s_service_lb_image` stages `servicelb-cloud-config.json` and
`config.yaml.d/60-servicelb.yaml` through gonf. It does not restart k3s.
Activate with serial node restarts, requiring Ready/etcd leader/ingress
acceptance between nodes. Never restart all three etcd members together.

The JSON retains the cloud-provider defaults verified on k3s 1.37.2 and
overrides only the ServiceLB image with an immutable v0.4.18 digest. The
supported `kube-cloud-controller-manager-arg` selects this separate config.
K3s continues to own and reconcile the ServiceLB DaemonSet; do not edit that
DaemonSet or the generated `server/etc/cloud-config.yaml` manually.

Recheck these defaults and the bundled image on future k3s upgrades. Remove
or update the override deliberately when the bundle supplies a newer release.
Rollback both managed files through gonf and restart nodes serially to use
the previous image; application data requires no rollback for this change.

Task 144 receipts and selected config backups on Earth:
`~/.local/state/conf/144-servicelb-20261108/`. Native root-private backups live
at `/root/144-servicelb-20261108/` on each r-node. The dated acceptance and
rollback instructions are in IQ's k3s area.
