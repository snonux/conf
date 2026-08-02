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

The image is pushed as `r0.lan.buetow.org:30001/shuriken:0.14.0` and the
CronJob pulls `registry.lan.buetow.org:30001/shuriken:0.14.0`. Bump `TAG` in
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

## Publishing (separate rsync CronJob, rsync protocol)

`shuriken-sync` is a second CronJob that publishes the generated
`/data/shuriken.sh/<site>/dist` trees to the public web servers (fishfinger +
blowfish) every 30 min. It uses the **rsync daemon protocol** (`rsync://`),
NOT SSH -- no key/Secret needed. The frontends run rsyncd via inetd with
`hosts allow = *.wg0.wan.buetow.org,*.wg0,localhost`; the k3s pods run on r-nodes
with `.wg0` (WireGuard) connectivity, so they're authorized to push over the
mesh. The writable modules `irregular-ninja` and `alt-irregular-ninja` are
declared in `frontends/etc/rsyncd.conf.tpl` (deploy with `rex -f
frontends/Rexfile rsync`).

It only publishes when a generation has **completed** since the last sync:
shuriken deletes `dist/status.json` at the start of a run and writes it last
on success, so status.json's presence + freshness vs a `.last-sync` marker on
NFS is the "completed, not yet published" signal. Most ticks are no-ops; a
publish fires once after each successful daily generation.

The generation CronJob has no `SYNC_*` settings -- it only writes to NFS; all
publishing goes through `shuriken-sync`. The `shuriken --sync` over SSH stays
available as a manual option (openssh-client is in the image); the cron job
just uses the rsync protocol.

### Frontend setup (one-time)

1. Deploy the rsyncd modules: `rex -f frontends/Rexfile rsync`.
2. The modules drop to `uid=www`; ensure the web dirs are www-writable. If
   migrating from the old SSH sync (files owned by `admin`):
   `doas chown -R www:www /var/www/htdocs/irregular.ninja /var/www/htdocs/alt.irregular.ninja` on both frontends.
3. The image must include `rsync` (it does). Rebuild/push if the registry holds
   an older `shuriken:0.13.2`: `cd /home/paul/git/conf/f3s/shuriken && just build-push` (from on-LAN).

## ArgoCD

`argocd-apps/services/shuriken.yaml` points ArgoCD at
`f3s/shuriken/helm-chart` in this repo (auto-sync, prune, self-heal), same
pattern as the other service apps.