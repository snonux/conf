# ArgoCD

GitOps controller for the f3s k3s cluster. Helm release `argocd` (chart
`argo/argo-cd`) in namespace `cicd`, single non-HA instance, Dex disabled.
UI at `https://argocd.f3s.lan.buetow.org` over LAN/WireGuard only.
`argocd-server-ingress-lan` uses `websecure` and the existing wildcard
`f3s-lan-tls` from the cicd `f3s-lan-wildcard` Certificate. The server runs
`--insecure` behind TLS termination. Metrics remain available via ServiceMonitor.
See [the staged migration and rollback](private-access.md) before deploying this
change to an existing installation; applying ingress.yaml does not delete the
old public ingress.

The chart version is pinned in the Justfile (`CHART_VERSION`, currently
`10.15.0` = Argo CD v3.6.1, verified on 2026-11-07). Bump it on purpose;
`just upgrade` never floats to the latest chart.

The 3.5 → 3.6 upgrade passed the server dry run and Helm atomic rollout.
API health/version, repository access and all 40 Application health checks
passed; Application/Project specs and admin/Redis credentials were preserved.
The five network-policy apps intentionally awaiting source publication remain
OutOfSync. Backups and the previous chart are stored privately under
`~/.local/state/conf/j34-app-upgrades-20261107/`; the previous Helm revision is
21. No source push was performed.

Redis is explicitly overridden to `8.10.3-alpine` in `values.yaml`.
Verified 2026-11-08: Helm revision 23 is deployed; only the Redis Deployment
spec changed. Runtime version, authenticated PING, a unique SET/GET/DEL
roundtrip and rejection of missing/incorrect credentials passed. Existing
Secret data and Application/Project specs were preserved (motionEye's
separate manual sync hold was the only concurrent Application spec change).
The chart's ECR image and official Docker image have the same immutable
config ID; the chart's exact reference is cached and CRI-verified on all
three nodes. Redis is an ephemeral cache (`--save "" --appendonly no`, no
PVC). Rollback is Helm revision 22, retaining the current credentials;
ArgoCD rebuilds its cache.

Files: `values.yaml`, `persistent-volumes.yaml` (10Gi hostPath PV for the
repo-server), `ingress.yaml`. The Application manifests live in
[`../argocd-apps/`](../argocd-apps/README.md).

## Install and operate

Once, before the first install:

```sh
mkdir -p /data/nfs/k3svolumes/argocd/repo-server   # NFS share, visible on r0-r2
chmod 777 /data/nfs/k3svolumes/argocd/repo-server
```

```sh
just install        # helm repo add, PV/PVC, helm install, ingress
just upgrade        # helm upgrade --version CHART_VERSION -f values.yaml + ingress
just status
just logs
just get-password   # initial admin password (user admin)
just uninstall      # also deletes the PV/PVC objects; NFS data stays
```

UI password changes survive `helm upgrade`.

CLI login: `argocd login argocd.f3s.lan.buetow.org --grpc-web --server-crt /path/to/f3s-lan-ca.crt`.

## Git source

Every Application pulls from Forgejo inside the cluster, no credentials:

```
repoURL: http://forgejo.services.svc.cluster.local/snonux/conf.git
targetRevision: master
```

```sh
git ls-remote https://code.f3s.buetow.org/snonux/conf.git
kubectl -n cicd exec deploy/argocd-repo-server -- \
  git ls-remote http://forgejo.services.svc.cluster.local/snonux/conf.git
```

There is no app-of-apps. A change under `f3s/argocd-apps/` needs a push to
Forgejo and a `kubectl apply -f` of that Application. The old git-server is
gone; if Forgejo itself breaks, restore from the ZFS/zrepl snapshots of
`zdata/enc/nfsdata` (see [`../forgejo/README.md`](../forgejo/README.md)).

## Troubleshooting

```sh
kubectl get pods -n cicd
kubectl get pv argocd-repo-server-pv; kubectl get pvc -n cicd argocd-repo-server-pvc
kubectl logs -n cicd -l app.kubernetes.io/name=argocd-repo-server
kubectl logs -n cicd -l app.kubernetes.io/name=argocd-application-controller
kubectl describe ingress -n cicd argocd-server-ingress-lan
```

Reset a lost admin password:

```sh
kubectl -n cicd delete secret argocd-initial-admin-secret
kubectl -n cicd rollout restart deployment argocd-server
kubectl -n cicd rollout status deployment argocd-server
just get-password
```

Backup: `kubectl get applications,appprojects,secrets -n cicd -o yaml > argocd-backup.yaml`;
the repo-server cache under `/data/nfs/k3svolumes/argocd/repo-server` is
disposable.

Grafana dashboards: 14584 (ArgoCD), 19993 (application metrics).
