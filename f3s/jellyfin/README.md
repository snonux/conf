# Jellyfin retirement (2026-10-17)

Jellyfin is retired. Player and Xplayer serve its former libraries using
read-only mounts. Their existing container paths and application databases
are retained.

All source and destination paths below are relative to
`/data/nfs/k3svolumes/` on the NFS server:

| Former directory | Current directory | Application mount |
|---|---|---|
| `jellyfin/libraries/musicvideos` | `player/libraries/musicvideos` | Player `/media/Music Videos` |
| `jellyfin/libraries/yoga` | `player/libraries/yoga` | Player `/media/Yoga` |
| `jellyfin/libraries/xman/ai` | `xplayer/libraries/ai` | Xplayer `/media/AI` |
| `jellyfin/libraries/xman/pictures` | `xplayer/libraries/pictures` | Xplayer `/media/Pictures` |
| `jellyfin/libraries/xman/videos` | `xplayer/libraries/videos` | Xplayer `/media/Videos` |

Both new library roots have `.nfs-sentinel` files. Ownership is numeric
`65534:65534`, directories are `0755`, and regular files are `0644`.
`player-content-pv/pvc` and `xplayer-content-pv/pvc` explicitly bind to the
new roots with `Retain`; their deployment mounts remain read-only.

The Jellyfin ArgoCD Application, chart, service/NodePort 30096, ingresses,
and obsolete shared-library PV/PVC pairs have been removed. The frontend
site inventory no longer generates its relayd routes, Gogios HTTP/HTTPS/TLS
checks, DNS records, or ACME renewal configuration. The dedicated relayd
upstream and PF rule have also been removed on both frontends.

## Rollback data

All three applications were stopped before the held snapshot on f0:
`zdata/enc/nfsdata@jellyfin-retirement-20261017T101941Z`, hold tag
`jellyfin-retirement`. Task `rf3` releases and deletes only this snapshot
on or after 2026-10-31 10:19:41 UTC, after checking the migration remains
healthy. The snapshot is protected from ordinary retention pruning. It also covers the
Audiobookshelf retirement; task `rf3` must verify both migrations before
cleanup (see [Audiobookshelf](../audiobookshelf/README.md)).

Jellyfin's remaining configuration/data tree is retained privately at
`/data/nfs/retired/jellyfin/20261017/`. No Jellyfin users, history, or database
records were imported into Player. Existing Player/Xplayer data and thumbnail
volumes remain in place. Separate pre-migration database backups and private
verification records are at
`~/.local/state/f3s/jellyfin-retirement-20261017/` on the controller.

To roll back, stop Player/Xplayer, move the five library trees back, restore
the Jellyfin tree and its chart/Application from git history, and restore
frontend site/routing configuration. Restore individual paths from the held
snapshot if needed; do not roll back the whole shared NFS dataset. Update
PV/PVC bindings through new resources rather than editing bound PV paths.

Earlier deployment and library notes remain in
[`docs/archive/f3s/jellyfin/`](../../docs/archive/f3s/jellyfin/).

## Verification

File counts, byte sizes, inode identities and relative paths matched before
and after all five moves (1,468 regular files total). Recursive ownership and
mode checks found no mismatches. Both applications completed rescans without
errors; set IDs, existing media IDs and container paths remained unchanged.

| Application | Set | Indexed media before / after |
|---|---|---|
| Player | Music Videos | 118 / 118 |
| Player | Yoga | 449 / 449 |
| Xplayer | AI | 301 / 301 |
| Xplayer | Pictures | 415 / 415 |
| Xplayer | Videos | 166 / 166 |

Authenticated tests covered all five sets: listing, decoded thumbnails,
media streaming, HTTP 206 raw downloads, and video or image decoding.
Both applications rejected missing media (404) and unauthenticated browsing
(401). Both frontend configurations validate, daemons are running, and each
Gogios configuration contains 539 checks with no Jellyfin targets.

A headless Chrome browser also passed real login, set/folder navigation,
and video playback for Music Videos, Yoga and Videos, plus image display
for AI and Pictures. Containers running as UID 65534 could read every moved
file, and write attempts to the mounted libraries were rejected.
