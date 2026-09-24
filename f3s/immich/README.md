# Immich

Photo/video backup, `https://immich.f3s.buetow.org` (LAN ingress in
`ingress-lan.yaml`). Namespace `services`. ArgoCD app `immich` has two
sources: `f3s/immich/helm-chart` (PVs/PVCs, PostgreSQL with pgvector,
Traefik middleware, LAN ingress) and upstream chart `immich` 0.10.3
(release `immich`: server, machine learning, Valkey).

| Volume | Size | hostPath |
|---|---|---|
| library | 500Gi | `/data/nfs/k3svolumes/immich/library` |
| ml-cache | 10Gi | `/data/nfs/k3svolumes/immich/ml-cache` |
| postgres | 20Gi | `/data/nfs/k3svolumes/immich/postgres` |
| valkey | 1Gi | `/data/nfs/k3svolumes/immich/valkey` |

All owned 911:911. Only `postgres.yaml` is ours, so only it has the NFS
sentinel check. Valkey instead has a liveness probe that also writes to
`/data`, so a stale NFS handle after a CARP failover restarts it.

## First install

```sh
mkdir -p /data/nfs/k3svolumes/immich/{library,ml-cache,valkey,postgres}
chown -R 911:911 /data/nfs/k3svolumes/immich/
kubectl create secret generic immich-db-secret \
  --from-literal=password='...' -n services     # before postgres first starts
kubectl apply -f ../argocd-apps/services/immich.yaml
```

Create the admin account in the web UI on first visit.

## Operate

```sh
just status | port-forward [2283] | sync | argocd-status
just logs-server | logs-ml | logs-postgres | logs-valkey
just restart            # server + ml
just restart-server | restart-ml | restart-postgres | restart-valkey
kubectl exec -n services -it deployment/immich-postgres -- psql -U immich -d immich -c '\l'
```

Rotate the DB password: recreate `immich-db-secret`, then

```sh
kubectl exec -n services -it deployment/immich-postgres -- \
  psql -U immich -d immich -c "ALTER USER immich WITH PASSWORD '...';"
kubectl rollout restart deployment -l app.kubernetes.io/instance=immich -n services
kubectl rollout restart deployment/immich-postgres -n services
```
