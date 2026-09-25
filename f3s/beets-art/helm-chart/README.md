# beets-art

Daily CronJob running `beets` over the Navidrome library: fetches
`cover.jpg` per album and embeds art in every audio file. Navidrome's hourly
scan (`ND_SCANSCHEDULE: "1h"`) picks it up. Namespace `services`, ArgoCD app
`beets-art`, pod pinned to r1.

Schedule 12:00 Europe/Sofia, `activeDeadlineSeconds` 6h. One pod, three
idempotent steps:

1. `beet import -A -q --quiet-fallback=asis /music` (`incremental: yes`;
   `auto: yes` on fetchart/embedart handles new albums).
2. `beet fetchart` (retries albums that failed before).
3. `beet embedart` (`compare_threshold: 50`, `ifempty: no`: won't overwrite
   existing embeds).

| Storage | |
|---|---|
| `navidrome-music-pvc` | 200Gi hostPath `/data/nfs/k3svolumes/navidrome/music` at `/music`, shared with Navidrome (RWX, so either pod can run on any r-node) |
| `beets-art-state-pvc` | 2Gi RWX NFS hostPath (`/data/nfs/k3svolumes/beets-art/state`), `library.db` + `import.log`, disposable |
| `beets-art-config` ConfigMap | `/etc/beets/config.yaml`, `BEETSDIR=/etc/beets` |

## Operate

```sh
just status | logs | run-now | suspend | resume | shell | sync | argocd-status
```

The first run imports and rewrites the whole library and can take hours.
ZFS-snapshot the music dataset first so the embed pass can be undone, then
`just run-now`.

Knobs in `templates/configmap.yaml`: `embedart.maxwidth` (1200),
`embedart.compare_threshold` (0 always overwrite, 100 never),
`fetchart.sources` (Cover Art Archive first), `embedart.remove_art_file: no`
(keep `cover.jpg` for Navidrome and cmus). Schedule and deadline in
`templates/cronjob.yaml`.

## Failures

- Non-zero Job: `just logs`. Usually NFS or Cover Art Archive rate limits;
  the next run clears it.
- Wrong art: `just shell`, then `beet mbsync album:"X"`, or drop a
  `cover.jpg` in the folder and `beet embedart -f cover.jpg -- album:"X"`.
- Corrupt DB: delete `beets-art-state-pvc`; the next run rebuilds it.
