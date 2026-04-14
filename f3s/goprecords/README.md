# goprecords on f3s

This directory contains Docker image and Kubernetes deployment config for `goprecords`.

## Image workflow

Build and push to the private NodePort registry:

```bash
cd /home/paul/git/conf/f3s/goprecords
just build-push
```

The image is pushed as:

- `r0.lan.buetow.org:30001/goprecords:0.3.1`

The deployment pulls from:

- `registry.lan.buetow.org:30001/goprecords:0.3.1`

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
