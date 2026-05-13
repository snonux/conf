# beets-art: nightly cover-art sweep for Navidrome

A Kubernetes CronJob that runs `beets` against the Navidrome music library
to fetch external cover art (`cover.jpg` in each album folder) and embed
art into every audio file. Navidrome's own hourly scan
(`ND_SCANSCHEDULE: "1h"`, see the [navidrome chart](../../navidrome/helm-chart/templates/deployment.yaml))
then picks the new art up.

## What it does

Every night at 03:30 UTC the CronJob runs three idempotent steps inside a
single short-lived pod on r1:

1. `beet import -A -q --quiet-fallback=asis /music` — registers any new
   albums in the beets library (`incremental: yes` skips already-known
   folders). With `auto: yes` set on both `fetchart` and `embedart`, this
   step alone is enough for newly imported music.
2. `beet fetchart` — backfill pass for albums that previously failed art
   lookup (rate limit, network blip, missing MBID). Idempotent: skips
   albums that already have art.
3. `beet embedart` — embeds art into audio files where the picture frame
   is missing. `compare_threshold: 50` and `ifempty: no` make it refuse
   to overwrite existing embeds.

## Why a CronJob (not a host systemd timer)

* Declarative: lives in git, deployed by ArgoCD, same pattern as every
  other f3s service.
* Sandbox: beets, ImageMagick, and ffmpeg dependencies are pinned to the
  container image — no host pollution, no Python venv drift on r1.
* Restart safety: if the job dies mid-run, k8s reaps it and the next run
  picks up where it left off (`incremental: yes`).

## Storage

* `navidrome-music-pvc` (200 GiB hostPath PV at
  `/data/nfs/k3svolumes/navidrome/music`) — **shared with Navidrome**.
  Mounted at `/music` inside the CronJob pod. RWO is fine because both
  pods are pinned to r1 (multi-pod single-node mounts are allowed for
  RWO PVCs).
* `beets-art-state-pvc` (2 GiB local-path PVC) — holds `library.db` and
  `import.log` between runs. Regenerable: deleting it just forces the
  next run to re-import the whole library.
* `beets-art-config` ConfigMap — mounted at `/etc/beets/config.yaml`
  (read-only) via `BEETSDIR=/etc/beets`. Single source of truth for
  beets settings.

## First run (initial backfill)

The first run will be **slow** (potentially hours) because:

* It has to import every album into the beets DB.
* Every album with no embedded art triggers a Cover Art Archive lookup.
* Every audio file gets rewritten to embed the new picture.

Trigger it manually instead of waiting for the nightly schedule:

```bash
just run-now
just logs       # tail the job
```

While the backfill runs, ZFS-snapshot the music dataset first so the
embed pass is reversible. Subsequent nightly runs touch only new or
previously-failed albums and finish in minutes.

## Operations

```bash
just status         # CronJob, recent Jobs, PVC, ArgoCD state
just logs           # follow logs of the most recent Job
just run-now        # ad-hoc Job from the CronJob template
just suspend        # pause the schedule
just resume         # resume the schedule
just shell          # interactive beets shell with the same mounts
just sync           # trigger ArgoCD sync
```

## Tuning knobs

In [templates/configmap.yaml](templates/configmap.yaml):

* `embedart.maxwidth` — embedded image cap (default 1200 px). Lower for
  smaller files.
* `embedart.compare_threshold` — 0 = always overwrite, 100 = never.
  Default 50 is "overwrite only when the new art is broadly similar".
* `fetchart.sources` — order of art providers. Cover Art Archive first
  exploits MusicBrainz IDs already present from beets imports.
* `embedart.remove_art_file: no` — keep `cover.jpg` in album folders too;
  Navidrome and cmus both benefit.

In [templates/cronjob.yaml](templates/cronjob.yaml):

* `schedule` — change from nightly if you want a different cadence.
* `activeDeadlineSeconds` — current cap is 6 h; raise for the very first
  run if your library is huge.

## Failure handling

* Job exits non-zero → visible in `just status` and `kubectl get jobs`.
  Inspect with `just logs`. Common causes: NFS hiccup, Cover Art Archive
  rate limit. The next nightly run usually clears it.
* Wrong art picked for an album → `just shell`, then
  `beet mbsync album:"Bad Album"` to refresh the MBID and re-fetch, or
  drop a hand-sourced `cover.jpg` into the album folder and run
  `beet embedart -f cover.jpg -- album:"Bad Album"`.
* Library DB corruption → delete the `beets-art-state-pvc` (state is
  regenerable) and let the next run rebuild it.
