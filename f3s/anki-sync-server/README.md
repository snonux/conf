# Anki sync server

`https://anki.f3s.buetow.org`. Namespace `services`, ArgoCD app
`anki-sync-server`, PVC `anki-data-pvc` mounted at `/anki_data`. Image built
from `docker-image/`.

The login comes from secret `anki-sync-server-secret` (env `SYNC_USER1`,
`user:password`), not in git:

```sh
kubectl create secret generic anki-sync-server-secret \
  --from-literal=SYNC_USER1='paul:...' -n services
kubectl edit secret anki-sync-server-secret -n services    # change it
```

```sh
just status | logs [lines] | port-forward [8080] | sync | argocd-status | restart
```
