# Immich

Photo/video backup, `https://immich.f3s.buetow.org` (LAN ingress in
`ingress-lan.yaml`). Namespace `services`. ArgoCD app `immich` has two
sources: `f3s/immich/helm-chart` (PVs/PVCs, PostgreSQL with pgvector,
Traefik middleware, LAN ingress) and upstream chart `immich` 0.12.0
(release `immich`: server, machine learning, Valkey). Server and machine
learning images are pinned to `v3.3.6` in the ArgoCD application. Valkey is
`9.1.3-alpine`, pinned by digest.

| Volume | Size | hostPath |
|---|---|---|
| library | 500Gi | `/data/nfs/k3svolumes/immich/library` |
| ml-cache | 10Gi | `/data/nfs/k3svolumes/immich/ml-cache` |
| postgres | 20Gi | `/data/nfs/k3svolumes/immich/postgres` |
| valkey | 1Gi | `/data/nfs/k3svolumes/immich/valkey` |

App volumes are owned 911:911; PostgreSQL manages its data as UID 999.
Only `postgres.yaml` is ours, so only it has the NFS
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

## pgvector 0.8.7 (2026-11-09)

The database image is now `immich-postgres:18.6-vchord1.1.1-pgvector0.8.7`
(manifest `sha256:0bddd210…`, config `be61f9a3…`). It is the previous image
plus the pinned PGDG package `postgresql-18-pgvector=0.8.7-1.pgdg12+1`. It
uses the same base digest and PostgreSQL 18.6 packages, and an identical
`vchord.so`. The SQL extension was then raised with
`ALTER EXTENSION vector UPDATE`. Immich only updates `vchord` itself, never
its `vector` dependency. The update scripts from 0.8.5 to 0.8.7 contain only
comments.

Compatibility: [pgvector 0.8.7](https://github.com/pgvector/pgvector/blob/v0.8.7/CHANGELOG.md)
is the latest tag. Immich v3.3.6 accepts `vector >=0.5 <1`, and its
[standalone Postgres docs](https://docs.immich.app/administration/postgres-standalone)
say VectorChord works with pgvector `>= 0.7, < 0.9`. VectorChord 1.1.1
remains the latest release. Upstream's bundle still ships 0.8.5 only
because its Renovate rule watches GitHub releases
([base-images#405](https://github.com/immich-app/base-images/issues/405)).
When upstream ships 0.8.7, consider returning to its image.

Before the live change, a copied-database trial ran the same sequence.
Restore with the old image, switch to the new binary, then `ALTER`. Data,
indexes and search did not change. Immich v3.3.6 started cleanly against the
upgraded copy. Live, the server and ML were scaled to zero for about
2.5 minutes. A quiesced dump was taken and proven to restore into a scratch
PostgreSQL with identical results. Then the image was set and the `ALTER`
applied. Checks across the change:

- All 72 table counts were unchanged.
- No index was invalid. `clip_index` and `face_index` (vchordrq) returned
  the same top-50 results through the index and by exact search.
- Updates to `vector` 0.8.8 and back to 0.8.5 were rejected.
- No schema drift was reported.
- Authenticated reads, smart search, thumbnail and original reads were
  unchanged. Wrong and missing API keys were rejected.
- Queue pause states and counters were unchanged.

Rollback: never return to the 0.8.5 image, because there is no downgrade
path. Stop the server and ML, then restore
`~/.local/state/conf/a44-pgvector-20261109/backup/immich-before-pgvector0.8.7.dump`
(311,652,221 bytes, SHA-256 `da60d9e8…`) into a fresh PGDATA on the old
image. Account for writes made since 2026-11-09 09:14 UTC. Never roll back
the shared NFS dataset.

## Patch verification (2026-11-08)

Updated server and ML to [v3.3.6](https://github.com/immich-app/immich/releases/tag/v3.3.6)
and Valkey to [9.1.3](https://github.com/valkey-io/valkey/releases/tag/9.1.3).
The chart remains 0.12.0. PostgreSQL remains 18.6 with VectorChord 1.1.1
and pgvector 0.8.5. All three new images were cached and checked through CRI
on r0-r2 before rollout. Helm rendering and server-side dry run passed.

Verified runtime versions, authenticated APIs, ML-backed smart search, original
photo/video bytes and thumbnails, and a unique PNG upload/download/thumbnail
round trip. The owned test image and its queued face-detection job were removed.
Invalid API credentials were rejected. Library totals remained 79,170 photos,
5,805 videos and 139,274,748,846 bytes; queue pause states, paused backlogs and
failure counts were preserved. Valkey PING, owned SET/GET/DEL and saved-RDB
validation passed. Automatic sync was restored to its original policy;
ArgoCD reports Synced/Healthy.

The server and ML were stopped for coherent database/queue backups. The private
directory `~/.local/state/conf/j34-app-upgrades-20261107/` contains
`immich-before-v3.3.6.dump` (311,603,739 bytes),
`immich-valkey-before-9.1.3.rdb` (16,162,760 bytes), their verified hashes and
archive checks in `immich-backups-ready.json`, plus the previous Application
and Deployment objects. Restore only with writers stopped and after accounting
for changes since the backup. No source push was performed.

## Upgrade verification (2026-10-17)

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

## Valkey security patch (2026-10-19)

The Application overrides the upstream chart's Valkey 9.1.0 with
`9.1.2-alpine`, pinned to the published multi-architecture digest. Valkey 9.2
is still a release candidate. Runtime 9.1.2, PING, write/read/delete, and RDB
persistence checks pass. All 19 queue counts and pause states match the
pre-upgrade baseline, including the paused face-detection queue and existing
failed jobs. Immich server version and ping APIs remain healthy.

A checked, point-in-time RDB backup (57,412 keys, 16,086,907 bytes) is private at
`~/.local/state/f3s/xf3-20261019/immich-valkey-pre-9.1.2.rdb` on the controller
(directory 0700, file 0600). SHA-256:
`4debb592b90df38b0795ab8b3d1a9b60d75017ec3aeee07ff209624d458917f4`.
For rollback, stop Immich writers and Valkey before restoring that RDB into
the Valkey PVC and returning the Application image to the old
`9.1-alpine@sha256:34f4f659e02a67cfdf3995d11ae432219c936005055b2cd39a042e86280e053e`
(runtime 9.1.0). Restoring the RDB loses queue changes since the backup;
photo files and PostgreSQL are separate and were not rolled back.

## PostgreSQL upgrade (2026-10-17)

PostgreSQL 16 was dumped and restored into PostgreSQL 18.6, with
VectorChord 1.1.1 and pgvector 0.8.5. Asset/user/album counts stayed at
89,359 / 2 / 1. Both Immich deployments returned healthy.

The current data directory is `/var/lib/postgresql/18/f3s`, under the
existing PostgreSQL PVC. The old PG16 files remain at the PVC root.
The private custom-format dump is
`~/.local/state/f3s/qf3-20261017/immich-pre-pg18.dump`. Retention removed
the ad hoc stopped-database snapshot. The surviving pre-upgrade snapshot on
f0 is `zdata/enc/nfsdata@zrepl_20261017_084600_000`, now protected by a ZFS
hold; PostgreSQL was running when it was taken.
Rollback requires returning to the old image, mount path, and PGDATA and
accounting for writes made since the restore.

The image is built from `docker-image/postgres/Dockerfile`. The latest
upstream extension bundle still had PostgreSQL 18.4, so the Dockerfile
pins the PostgreSQL 18.6 Debian packages (and, since 2026-11-09, pgvector
0.8.7; see above). Build and push under a new tag; never overwrite an
existing tag, since the Deployment's default IfNotPresent policy keeps
using cached copies on the nodes:

```sh
podman build -t r0.lan.buetow.org:30001/immich-postgres:18.6-vchord1.1.1-pgvector0.8.7 docker-image/postgres
podman push --tls-verify=false r0.lan.buetow.org:30001/immich-postgres:18.6-vchord1.1.1-pgvector0.8.7
```

The upstream image rejects NFS by default. `IGNORE_DATABASE_FSTYPE=true`
retains this cluster's storage arrangement: hard-mounted NFSv4, FreeBSD
`vfs.nfsd.async=0`, ZFS `sync=standard`, and PostgreSQL `fsync=on`.
These match [PostgreSQL's NFS requirements](https://www.postgresql.org/docs/18/creating-cluster.html#CREATING-CLUSTER-NFS).
