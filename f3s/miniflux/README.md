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

## Application upgrade (2026-11-08)

Miniflux 2.3.5 is running. Health, the authenticated user/feed APIs, invalid-token
rejection and the runtime version passed verification. Database counts remained
2 users, 38 feeds and 195 entries. PostgreSQL remains on 18.6.

Automatic sync was held during the upgrade, until the matching chart was
published. On 2026-11-09 (task e54) Argo was Synced at the published commit
1fc3dfa, and the original `prune: true, selfHeal: true` policy was restored.

The application was stopped for a consistent pre-upgrade custom-format dump at
`~/.local/state/conf/j34-app-upgrades-20261107/miniflux-before-2.3.5.dump`.
Its archive listing passed validation. The saved Application/Deployment objects
are in the same private directory. A schema rollback requires stopping the
application and restoring that database together with the old 2.3.3 image;
restoring it loses writes made after the backup.

## PostgreSQL 18 (2026-10-17)

The database was dumped from PG17 and restored into PG18.6. The 2 users,
38 feeds, and 376 entries matched afterward, and Miniflux returned healthy.
`PGDATA=/var/lib/postgresql/18/f3s`; the original PG17 files remain at the
PVC root. The private dump is
`~/.local/state/f3s/qf3-20261017/miniflux-pre-pg18.dump`. Retention removed
the ad hoc stopped-database snapshot. The surviving pre-upgrade snapshot on
f0 is `zdata/enc/nfsdata@zrepl_20261017_084600_000`, now protected by a ZFS
hold; PostgreSQL was running when it was taken.
Rollback requires restoring the old image, mount path and PGDATA, and
accounting for writes since the restore.
