# Anki sync server

`https://anki.f3s.buetow.org`. Namespace `services`, ArgoCD app
`anki-sync-server`, PVC `anki-data-pvc` mounted at `/anki_data`. Image built
from `docker-image/`, pinned to Anki 26.09.4.

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

## Verified upgrade, 2026-11-08

The 26.09.4 image uses Alpine 3.24.4. Its Docker HEALTHCHECK metadata is
retained; local health, authenticated protocol-11 login/metadata and rejection
of a bad password passed in a disposable instance. The published registry
and running CRI config IDs match the tested image. Build helpers use the
correct nested Justfile and version; Cargo defaults to two build jobs.

Before rollout the writer was stopped, the complete data tree was captured
in f0's held snapshot `zdata/enc/nfsdata@j34-before-anki26-20261108` (hold
`j34-anki-upgrade`), and a private SQLite-family/metadata archive was downloaded
and SHA-256 verified. Both quiesced databases passed quick_check with Anki's
Unicode case-folding collation registered. Online SQL readers encounter the
server's exclusive locks; use sync-protocol metadata while it is running.

Live health/login/metadata checks passed after rollout. The credential Secret,
host key and collection/media sync metadata are preserved, as are all 3,113
media files, their byte total and sampled contents. No production sync upload
or download was initiated for the test.

Automatic sync was held until this matching chart was published; it was
restored (`prune: true, selfHeal: true`) on 2026-11-09 (task e54), after Argo
was Synced at the published commit 1fc3dfa. For rollback,
stop writes and preserve any new user changes, then restore only the selected
Anki data with the previous image. Never roll back the shared NFS dataset.
Private receipts are under `~/.local/state/conf/j34-app-upgrades-20261107/`.
