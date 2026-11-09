# goprecords on f3s

This directory contains Docker image and Kubernetes deployment config for `goprecords`.

## Image workflow

Build and push to the private NodePort registry:

```bash
cd /home/paul/git/conf/f3s/goprecords
GOPRECORDS_TAG=<app version>-runtime<YYYYMMDD> just build-push
```

`docker-image/Justfile` builds the source's `test` stage (vet, unit and
fixture tests as a non-root user) before the `runtime` stage. The source
Dockerfile pins the Go and Alpine bases by digest. Every rebuild gets a NEW
immutable tag; never re-push a deployed tag.

Current image (app v0.6.2, Go 1.27.3, Alpine 3.24.4, task 644):

- pushed as `r0.lan.buetow.org:30001/goprecords:0.6.2-runtime20261108`
- deployed as `registry.lan.buetow.org:30001/goprecords:0.6.2-runtime20261108`
  (manifest `sha256:f725e56bb71108a105c52740833ceddf6afb233ff8e1feb15b1ffc833a04b445`)

The previous image `goprecords:0.6.2` (Go 1.21.13, Alpine 3.20.10) stays in
the registry for emergency rollback only.

## Updating the image safely

The Deployment is a single `Recreate` writer on the NFS stats tree, which also
holds `goprecords-auth.db` (client key hashes). Hosts upload hourly at :00,
:04, :15, :17 and :19, so roll out between minute 21 and 52. Before switching
images: hold Argo automated sync for this app, scale to 0, take a held ZFS
snapshot of `zdata/enc/nfsdata` on the CARP master, and archive the stats tree.
Never roll back the shared NFS dataset; restore only this stopped tree.

## Runtime config

The container runs daemon mode:

- `-daemon`
- `-listen=:8080`
- `-stats-dir=/data/stats`

Data persistence:

- PVC: `goprecords-stats-pvc`
- Mount path: `/data/stats`
- Auth DB defaults to `/data/stats/goprecords-auth.db`

## Endpoints

- `/health` (liveness)
- `/livez` (liveness)
- `/readyz` (readiness)
- `/report` (HTTP read API)
- `/upload/{host}/{kind}` (upload API)
