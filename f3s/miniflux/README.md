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
