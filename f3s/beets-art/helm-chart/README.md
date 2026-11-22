# beets-art

Daily CronJob running `beets` over the Navidrome library: fetches
`cover.jpg` per album and embeds art in every audio file that is still
missing it. Navidrome's hourly
scan (`Scanner.Schedule` in the navidrome ConfigMap's `navidrome.toml`, task
o44) picks it up; a quick scan's folder hash includes each file's size and
mtime, so it sees the changes even when beets-art runs on another node.
Namespace `services`, ArgoCD app `beets-art`; the pod is pinned to `r0` —
see `templates/cronjob.yaml` for why the thermal exception to the
"nothing is pinned" house rule.

Schedule 12:00 Europe/Sofia, `activeDeadlineSeconds` 6h per pod, and one
pod retry (`backoffLimit: 1`) so a run killed by an f3s power-off is
retried after boot instead of failing the Job (see the comments in
`templates/cronjob.yaml`). One pod, three idempotent steps:

1. `beet import -A -q --quiet-fallback=asis /music` (`incremental: yes`;
   `auto: yes` on fetchart/embedart handles new albums).
2. `beet fetchart` (retries albums that failed before).
3. `beet embedart` (`ifempty: yes`: embeds only where art is missing;
   `compare_threshold: 0`: the similarity check is disabled — it is broken
   with ImageMagick 7, see Failures).

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
`embedart.compare_threshold` (0 disables the similarity comparison,
`ifempty: yes` embeds only where art is missing),
`fetchart.sources` (Cover Art Archive first), `embedart.remove_art_file: no`
(keep `cover.jpg` for Navidrome and cmus). Schedule, deadline and the `r0`
nodeSelector in `templates/cronjob.yaml`.

## Failures

- `Error while checking art similarity; skipping.` on every track
  (fixed 2026-11-22): with beets 2.14.1 + ImageMagick 7.1.2-35, comparing
  identical images always failed — IM writes the PHASH metric to stderr
  and exits 0, beets reads stdout on the exit-0 path, gets `''` and
  returns `None`. Every already-embedded track errored and was skipped
  (1,296 per run, identical across runs and nodes): the sweep still spent
  ~2.5h extracting art it then discarded, burning ~1 core on whichever
  host landed the job (the original f1 thermal spike). Disabled via
  `compare_threshold: 0` + `ifempty: yes`.
- Non-zero Job: `just logs`. Usually NFS or Cover Art Archive rate limits;
  the next run clears it.
- A failed Job keeps KubeJobFailed firing until three newer Jobs fail
  (`failedJobsHistoryLimit`); successful runs do not prune it.
  `kubectl delete job -n services <job>` clears it. ArgoCD shows the app
  Degraded until a Job owned by the CronJob succeeds; `just run-now`
  (`create job --from=cronjob`) counts.
- Wrong art: `just shell`, then `beet mbsync album:"X"`, or drop a
  `cover.jpg` in the folder and `beet embedart -f cover.jpg -- album:"X"`.
- Corrupt DB: delete `beets-art-state-pvc`; the next run rebuilds it.

## Verified image update, 2026-11-08

Future scheduled jobs use LinuxServer `2.14.1-ls359` (beets itself remains
2.14.1). The existing scheduled sweep continues with its original image.
An owned one-shot Job mounted production state/music read-only, used SQLite's
backup API to make a disposable database copy, and passed quick_check,
`beet version` and `beet stats`: 1,690 tracks / 197 albums. Both art plugins
loaded, and the running immutable image config ID matched the staged image.
The test Job and its disposable copy were removed. No test art sweep modified
production music. Completion of the current scheduled sweep remains separate.

Automatic sync was held during the 2026-11-08 upgrade until this chart was
published; e54 restored prune+selfHeal on 2026-11-09 after verifying Git equals
live. Rollback of
the template is the previous `2.14.1-ls356` image; no production DB migration
was performed by this image test.
