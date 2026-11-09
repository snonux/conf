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

### Adopting kube-dns (m54)

k3s's apply (wrangler) finds the objects an AddOn owns by the label
`objectset.rio.cattle.io/hash`, a hash of the owning AddOn's name. If it does not
find an object, it creates it, and adopts the existing one only when the create
fails with AlreadyExists. That took over the Deployment, ConfigMap,
ServiceAccount and RBAC. It cannot take over `svc/kube-dns`: the create fails
first because ClusterIP 10.43.0.10 is already allocated. From z34's activation
until m54, `conf-coredns` logged ApplyManifestFailed every 15 s and never got
a checksum.

m54 fixed this on 2026-11-09 with a one-time relabel, without deleting the
Service (UID, ClusterIP and spec unchanged, no DNS failures):

```sh
kubectl -n kube-system label svc kube-dns --overwrite \
  objectset.rio.cattle.io/hash=abfc2dac76a56946fd6db38ebdb52fae5fc72eed
```

The value is the hash k3s puts on every `conf-coredns` object (compare
`kubectl -n kube-system get deploy coredns --show-labels`). Within one 15 s
pass, k3s patched the owner annotations itself and logged AppliedManifest on
all servers. The labels are part of the AddOn's applied state, so k3s keeps
them, also across k3s restarts. Repeat the relabel only if a cluster ran the
bundled AddOn before this one, for example after a restore.

The skipped bundled `coredns` AddOn object stays. Deleting it would be
harmless (tested on a scratch k3s 1.37.2): the objects have no owner
references and k3s does not recreate it while the file is skipped. It would
also gain nothing, because the `.skip` marker alone prevents a fallback. Without the marker, k3s applies the regenerated bundled manifest on
restart, takes the Deployment back to 1.14.7 and then fails on the Service.
Earth receipts: `~/.local/state/conf/m54-coredns-adopt-20261109/`.

## Metrics-server

`rnodes_k3s_metrics_server_prepare` stages the nested
`metrics-server/metrics-server-deployment.yaml.skip` on every server.
Then `rnodes_k3s_metrics_server` installs identical
`conf-metrics-server.yaml` user AddOns. Only the packaged ServiceAccount and
Deployment move to this AddOn; the other six packaged files keep their
original RBAC, Service and APIService ownership.

The k3s 1.37.2 copy changes only the pinned v0.9.1 image and
maxUnavailable=0. Preserve its UID 1000, kubelet certificate checks, arguments
and aggregation configuration. Do not add an insecure TLS option to get a
rollout working. Verify fresh metrics for all three nodes, pod metrics,
APIService availability and unauthenticated API rejection after rollout and a
startup canary. No HPA was deployed at task 044 acceptance.

Rollback the image in the managed AddOn through gonf on all servers; retain
the skip markers while this AddOn owns the deployment. Review the source
copy against future k3s bundles. Native backups:
`/root/044-metrics-20261108/`; Earth receipts:
`~/.local/state/conf/044-metrics-20261108/`.
