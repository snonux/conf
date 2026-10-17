# Miniflux

RSS reader, `https://flux.f3s.buetow.org`. Namespace `services`, ArgoCD app
`miniflux`, with its own `miniflux-postgres`. Volume
`/data/nfs/k3svolumes/miniflux/data` (create once).

Secrets (not in git), create before the first sync:

```sh
kubectl create secret generic miniflux-db-password \
  --from-literal=fluxdb_password='...' -n services
kubectl create secret generic miniflux-admin-password \
  --from-literal=admin_password='...' -n services
```

The chart builds
`postgres://miniflux:${POSTGRES_PASSWORD}@miniflux-postgres:5432/miniflux?sslmode=disable`
from `fluxdb_password`. `admin_password` is read as `ADMIN_PASSWORD` on first
start only, for user `admin`.

```sh
just status | logs [lines] | logs-postgres | port-forward [8080] | port-forward-postgres [5432]
just sync | argocd-status | restart | restart-postgres | psql
```

## PostgreSQL 18 (2026-10-17)

The database was dumped from PG17 and restored into PG18.6. The 2 users,
38 feeds, and 376 entries matched afterward, and Miniflux returned healthy.
`PGDATA=/var/lib/postgresql/18/f3s`; the original PG17 files remain at the
PVC root. The private dump is
`~/.local/state/f3s/qf3-20261017/miniflux-pre-pg18.dump`; the stopped-database
snapshot on f0 is `zdata/enc/nfsdata@qf3-postgres-before18-20261017`.
Rollback requires restoring the old image, mount path and PGDATA, and
accounting for writes since the restore.
