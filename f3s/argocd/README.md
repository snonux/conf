# ArgoCD

GitOps controller for the f3s k3s cluster. Helm release `argocd` (chart
`argo/argo-cd`) in namespace `cicd`, single non-HA instance, Dex disabled.
UI at `argocd.f3s.buetow.org` (Traefik ingress, server runs `--insecure`
behind it). Metrics via ServiceMonitor.

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
just upgrade        # helm upgrade -f values.yaml + ingress
just status
just logs
just get-password   # initial admin password (user admin)
just uninstall      # also deletes the PV/PVC objects; NFS data stays
```

UI password changes survive `helm upgrade`.

CLI login: `argocd login argocd.f3s.buetow.org --insecure`.

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
kubectl describe ingress -n cicd argocd-server-ingress
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
