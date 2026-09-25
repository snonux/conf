# KOReader sync server

[koreader-sync-server](https://github.com/koreader/koreader-sync-server) at
`https://koreader.f3s.buetow.org`. Namespace `services`, ArgoCD app
`kobo-sync-server`, selector `app=koreader-sync-server`, PVC `kobo-data-pvc`.
Volume `/data/nfs/k3svolumes/koreader-sync-server/data` (create once on the
storage master).

```sh
just status | logs [lines] | port-forward [8080] | sync | argocd-status | restart
```
