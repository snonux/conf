# Wallabag Helm Chart

This chart deploys Wallabag.

## Prerequisites

Before installing the chart, you must manually create the following directories on your host system to be used by the persistent volumes:

- `/data/nfs/k3svolumes/wallabag/data`
- `/data/nfs/k3svolumes/wallabag/images`

## Installing the Chart

Manual install (normally ArgoCD deploys it):

```bash
helm install wallabag . --namespace services --create-namespace
```

## Verified upgrade, 2026-11-08

Wallabag 2.6.15 is deployed. The staged immutable image, public version API,
login form with CSRF token and rejection of an unauthenticated entries API
request were verified. SQLite quick_check is clean; all table row counts are
unchanged, including 2,777 entries and two users. No current access token was
available, so an authenticated entry operation was not tested.

A quiesced archive of data and images was downloaded and SHA-256 checked
before startup, under `~/.local/state/conf/j34-app-upgrades-20261107/`.
Restore those directories with the writer stopped and the old 2.6.14 image
if rollback is needed. Automatic sync was held during the 2026-11-08
upgrade until this chart was published; e54 restored prune+selfHeal on
2026-11-09 after verifying Git equals live.
