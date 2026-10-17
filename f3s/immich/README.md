# Immich

Photo/video backup, `https://immich.f3s.buetow.org` (LAN ingress in
`ingress-lan.yaml`). Namespace `services`. ArgoCD app `immich` has two
sources: `f3s/immich/helm-chart` (PVs/PVCs, PostgreSQL with pgvector,
Traefik middleware, LAN ingress) and upstream chart `immich` 0.10.3
(release `immich`: server, machine learning, Valkey). Server and machine
learning images are pinned to `v3.3.3` in the ArgoCD application.

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

## Last upgrade verification (2026-10-17)

Upgraded the server and machine-learning images from v3.0.3 to v3.3.3.
Helm rendering and Kubernetes server-side validation passed. Both deployments
rolled out on r0/r1; ArgoCD reported Synced/Healthy. PostgreSQL migrations
completed with no schema drift. The pre-upgrade custom-format database dump
is retained outside git at
`~/.local/state/f3s/immich-upgrades/20261017-v3.3.3/pre-upgrade.dump`.

Verified public HTTPS (IPv4 and IPv6), LAN HTTPS, authentication, library
statistics, albums, people, external libraries, original photo/video reads,
photo thumbnails, video playback, and ML-powered smart search. A disposable
PNG upload round-tripped byte-for-byte and completed metadata extraction and
thumbnail generation, then was deleted. Library counts and usage stayed at
78,958 photos, 5,763 videos, and 134,291,943,729 bytes.

r2 was already unreachable to Kubernetes, but its orphaned Immich worker
was still consuming Valkey jobs and failing PostgreSQL DNS lookups. A temporary
`immich-valkey-quarantine-r2` NetworkPolicy on Valkey allowed verification using
r0/r1. **The policy was deleted after verification; r2 is unblocked.** r2 repair
is handled separately. Face detection was already paused and remains paused;
queue snapshots record the existing backlog and failures. Final queue counts
and pause states match the baseline, with no active or waiting jobs.
