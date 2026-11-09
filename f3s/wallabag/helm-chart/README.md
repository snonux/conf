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

## Image revision 2.6.15.1, 2026-11-09

The image is `wallabag/wallabag:2.6.15.1`, pinned by its multi-arch
index digest; the Wallabag app is still 2.6.15. The revision moves the
shipped `php.ini` and `php-fpm.conf` into `/etc/php82`. From 2.6.14.1
to 2.6.15 they sat in an unread `/etc/php81`, so PHP ran Alpine
defaults. The pod now runs with a 300 s execution time, matching
nginx's 300 s FastCGI timeout, 100M upload and post limits, and no
`X-Powered-By` header. Packages, binaries, the entrypoint and the
Wallabag tree are unchanged, including its migrations, so the rollout
wrote nothing to the database: the SQLite file's checksum is the same
before and after.

A verified online backup (data/images archive plus a `VACUUM INTO`
snapshot) is under `~/.local/state/conf/k54-wallabag-2.6.15.1-20261109/`.
`doctrine:migrations:status` lists ten 2016-2019 migrations as not
migrated. That predates this change and is expected on SQLite: each one
has a `skipIf` guard (MySQL-only, SQLite-incompatible or already applied),
and Doctrine does not record skipped migrations.

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
