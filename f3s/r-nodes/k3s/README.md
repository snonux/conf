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

## CoreDNS

`rnodes_k3s_core_dns_prepare` creates the documented `coredns.yaml.skip`
on every server. It suppresses the bundled manifest without uninstalling DNS.
Run this preparation on all three servers before the first
`rnodes_k3s_core_dns` deployment, which installs the identical
`conf-coredns.yaml` user AddOn on each. K3s still applies and reconciles it.
Do not use `--disable=coredns`, delete the old AddOn, or edit the generated
bundled file. The native node controller continues owning ConfigMap NodeHosts.

The tracked copy is based on k3s 1.37.2 and preserves Corefile/imports,
Service, RBAC and pod settings. Its intentional changes are the immutable
1.15.0 image and maxUnavailable=0, retaining the old single DNS replica until
its replacement is ready. Review the copy against bundled changes on future
k3s upgrades; .skip means the bundle will not update these resources for you.

Rollback by restoring the previous image in this managed AddOn and reapply
through gonf on all servers. Keep the skip marker and common manifest in sync;
do not uninstall DNS or restore application data. If deliberately returning
ownership to the bundle later, plan that as a separate coordinated change.
Task z34 native backups: `/root/z34-coredns-20261108/` on each node; Earth
receipts: `~/.local/state/conf/z34-coredns-20261108/`.
