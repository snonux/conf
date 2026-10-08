# shuriken

Kubernetes CronJobs that build and publish the `irregular.ninja` and
`alt.irregular.ninja` photo albums with
[shuriken.sh](https://github.com/snonux/shuriken.sh). Namespace `services`,
ArgoCD app `shuriken` (`f3s/shuriken/helm-chart`).

| CronJob | When (Europe/Sofia) | What |
|---|---|---|
| `shuriken` | hourly (12:00 excluded, beets-art slot); marker-gated: a tick generates only when the last successful run is >= 24h ago | `shuriken --generate --config` for each `/configs/*.conf`, sequentially, `--image-jobs 1` (`SHURIKEN_IMAGE_JOBS`) |
| `shuriken-sync` | 10:30, 18:30 | rsync each changed `dist/` to fishfinger and blowfish |

The `shuriken` CronJob ticks hourly but keeps marker files on the PV:
`/data/shuriken.sh/.last-generate-run` is the **hour-quantized** epoch of
the last fully successful multi-site run — a tick aborts early unless
that run is >= 24h ago, a missing or malformed marker means "never ran"
and generates immediately, and quantizing to the gate-pass hour keeps
the run pinned to one stable hour (a completion-time stamp would drift
an hour later every day, because the run duration rounds up to the next
tick; within one DST season the hour is stable — each spring-forward
transition shifts it one local hour later, where it re-pins).
`/data/shuriken.sh/.last-generate-fail` is stamped before every attempt
and removed on success, so it also covers runs killed by the nightly
power-off or the 6h deadline; a surviving marker backs the next retry
off for 4h. This self-heals missed
schedules — the cluster is powered off at night, so a fixed daily slot
silently no-ops whenever the nodes are down at that hour, while the
hourly tick just catches the first hour the cluster is back up (12:00
is skipped so a run doesn't start on top of the daily beets-art sweep).

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
  .last-generate-run    # hour-quantized epoch of last successful generate (24h gate)
  .last-generate-fail   # epoch of last attempt, removed on success (4h retry backoff)
  .lock                 # flock shared with shuriken-sync (fd form, see cronjob.yaml)
  incoming/irregular.ninja      -> ../../syncthing/.../irregular.ninja
  incoming/alt.irregular.ninja  -> ../../syncthing/.../alt.irregular.ninja
  irregular.ninja/dist/
  alt.irregular.ninja/dist/
```

(The markers are written atomically via a same-directory `.<name>.tmp` +
rename; a leftover `.tmp` after a crash is harmless and overwritten by the
next run.)

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

`just run` goes through the same marker gate as the hourly ticks: if the
last successful run is < 24h ago (or the last attempt failed < 4h ago),
the manual run skips too. To force a regeneration, delete the markers
first (root on any r-node):
`rm /data/nfs/k3svolumes/shuriken.sh/.last-generate-{run,fail}`.
