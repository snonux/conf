# shuriken

Kubernetes CronJobs that build and publish the `irregular.ninja` and
`alt.irregular.ninja` photo albums with
[shuriken.sh](https://github.com/snonux/shuriken.sh). Namespace `services`,
ArgoCD app `shuriken` (`f3s/shuriken/helm-chart`).

| CronJob | When (Europe/Sofia) | What |
|---|---|---|
| `shuriken` | 13:00 (the cluster is powered off at night) | `shuriken --generate --config` for each `/configs/*.conf`, sequentially, `--image-jobs 1` (`SHURIKEN_IMAGE_JOBS`) |
| `shuriken-sync` | 10:00, 18:00 | rsync each changed `dist/` to fishfinger and blowfish |

## Image

```sh
# in ~/git/shuriken.sh first: just build   (bin/shuriken must be current)
cd ~/git/conf/f3s/shuriken
just build-push      # -> r0.lan.buetow.org:30001/shuriken:0.14.1, run on the LAN
```

Pods pull `registry.lan.buetow.org:30001/shuriken:0.14.1`. On a new release
bump `TAG` in `docker-image/Justfile`, chart `appVersion` and the CronJob
`image:` together. The image carries rsync and openssh-client, no jq.

## Storage

PV/PVC `shuriken-data-pv`/`shuriken-data-pvc`: hostPath
`/data/nfs/k3svolumes` at `/data` (RWX). The whole tree is mounted because
`incoming/<site>` are relative symlinks into `../../syncthing/...`. Writes
only go to `/data/shuriken.sh/<site>/dist`. Sentinel:
`shuriken.sh/.nfs-sentinel`.

```
/data/nfs/k3svolumes/shuriken.sh/
  .nfs-sentinel
  incoming/irregular.ninja      -> ../../syncthing/.../irregular.ninja
  incoming/alt.irregular.ninja  -> ../../syncthing/.../alt.irregular.ninja
  irregular.ninja/dist/
  alt.irregular.ninja/dist/
```

Per-site `shuriken.conf` in `helm-chart/templates/configmap.yaml`, mounted at
`/configs`: `INCOMING_DIR` = the resolved syncthing path, `DIST_DIR` =
`/data/shuriken.sh/<site>/dist` (a real subdirectory, so the atomic-swap
rename works). No `SYNC_*` settings.

## Publishing

`shuriken-sync` uses the rsync daemon protocol (`rsync://`), no SSH key. The
frontends run rsyncd from inetd with `hosts allow =
*.wg0.wan.buetow.org,*.wg0,localhost`; pods on the r-nodes reach it over
WireGuard. Writable modules `irregular-ninja` and `alt-irregular-ninja` in
`gonf/frontends/assets/rsyncd.conf`, running as `uid=www`.

A site is published only when `image_count` or `total_size_bytes` in
`dist/status.json` differs from `<site>/.last-published-status.json` on NFS
(`generated_at` is ignored; it changes every run).

One-time frontend setup:

```sh
./gonf.sh cluster frontends frontends_rsync
doas chown -R www:www /var/www/htdocs/irregular.ninja /var/www/htdocs/alt.irregular.ninja   # on both, if files are owned by admin
```

Manual SSH publishing (`shuriken --sync`) still works from the image.

## Operate

```sh
just status          # CronJobs, PVC, ArgoCD
just run             # manual generate run, tails logs
just sync | argocd-status
```
