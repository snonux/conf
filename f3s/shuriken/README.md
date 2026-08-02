# shuriken on f3s

Nightly Kubernetes CronJob that regenerates the `irregular.ninja` and
`alt.irregular.ninja` static photo albums with [shuriken.sh][shuriken] and
writes them to the shared NFS export.

[shuriken]: https://github.com/snonux/shuriken.sh

## What it does

Once a day (04:00 Europe/Sofia) a CronJob pod runs the shuriken Docker image.
The image entrypoint iterates over `/configs/*.conf` and runs
`shuriken --generate --config <conf>` for each, single-threaded by default
(`--image-jobs 1`, overridable via `SHURIKEN_IMAGE_JOBS`). Both albums are
generated sequentially in one run.

## Image workflow

Build and push to the private NodePort registry (run `just build` in the
shuriken.sh repo first so `bin/shuriken` is current):

```bash
cd /home/paul/git/conf/f3s/shuriken
just build-push
```

The image is pushed as `r0.lan.buetow.org:30001/shuriken:0.13.2` and the
CronJob pulls `registry.lan.buetow.org:30001/shuriken:0.13.2`. Bump `TAG` in
`docker-image/Justfile` and `appVersion`/the CronJob `image:` tag together
when releasing a new shuriken version.

## Storage

- **PV/PVC** `shuriken-data-pv` / `shuriken-data-pvc`: hostPath
  `/data/nfs/k3svolumes` mounted at `/data` (RWX, shared NFS). The whole tree
  is mounted because the incoming dirs under
  `/data/shuriken.sh/incoming/<site>` are relative symlinks into
  `../../syncthing/...` that only resolve against the full layout. The job
  writes only under `/data/shuriken.sh/<site>/dist`.
- **NFS sentinel**: `shuriken.sh/.nfs-sentinel` on the NFS server; the
  initContainer refuses to start if it is missing (NFS down / stale local-XFS
  shadow).

## Per-site config

`helm-chart/templates/configmap.yaml` holds a `shuriken.conf` per site,
mounted at `/configs`. Each points `INCOMING_DIR` at the readlink-resolved
syncthing source and `DIST_DIR` at `/data/shuriken.sh/<site>/dist` (a regular
subdirectory under the mount, so shuriken's staging atomic-swap rename works).
No `SYNC_*` settings -- the container only generates; publishing to
fishfinger/blowfish stays a separate concern.

## Layout on the NFS server (f0)

```
/data/nfs/k3svolumes/shuriken.sh/
  .nfs-sentinel
  incoming/
    irregular.ninja      -> ../../syncthing/.../irregular.ninja
    alt.irregular.ninja  -> ../../syncthing/.../alt.irregular.ninja
  irregular.ninja/dist/     # generated
  alt.irregular.ninja/dist/ # generated
```

## Operate

```bash
just status          # CronJob + PVC + ArgoCD status
just run             # trigger a manual run and tail logs
just sync            # refresh the ArgoCD app
just argocd-status   # argocd CLI view
```

## Publishing (separate rsync CronJob)

`shuriken-sync` is a second CronJob that rsyncs the generated
`/data/shuriken.sh/<site>/dist` trees to the public web servers
(`admin@fishfinger.buetow.org` and `admin@blowfish.buetow.org`, with
`--delete`) every 4h -- decoupled from the daily generation so a publish can
be retried far more often than a (re)generate. It reuses the shuriken image
(rsync + openssh-client) but overrides the command, so it never runs
generation.

It needs the admin SSH key authorized on fishfinger/blowfish. Until that key
is provisioned the publish pods fail to mount the key and publish nothing (safe
by design -- no live publish without the key). Create the Secret once:

```bash
kubectl create secret generic shuriken-rsync-ssh-key -n services \
    --from-file=id_ed25519=/path/to/admin_ed25519
```

The generation CronJob deliberately has no `SYNC_*` settings -- it only writes
to NFS; all publishing goes through `shuriken-sync`.

The image includes `openssh-client` (alongside rsync) so the same image serves
both jobs. If the registry still holds an older `shuriken:0.13.2` without it,
rebuild and push before activating sync (the generation job is unaffected):

```bash
cd /home/paul/git/conf/f3s/shuriken && just build-push
```

## ArgoCD

`argocd-apps/services/shuriken.yaml` points ArgoCD at
`f3s/shuriken/helm-chart` in this repo (auto-sync, prune, self-heal), same
pattern as the other service apps.