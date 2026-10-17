# Audiobookshelf retirement (2026-10-17)

Audiobookshelf is retired. Player serves its audiobook files and the former
podcast storage as read-only sets. The existing Audiobooks container path
and Player database remain in place.

Paths below are relative to `/data/nfs/k3svolumes/` on the NFS server:

| Former directory | Current directory | Player mount |
|---|---|---|
| `audiobookshelf/audiobooks/default` | `player/libraries/audiobooks` | `/media/Audiobooks` |
| `audiobookshelf/podcasts` | `player/libraries/podcasts` | `/media/Podcasts` |

The existing `player-content-pvc` serves both directories through subPaths.
The library init container checks the shared `.nfs-sentinel` and both new
subdirectories before mounting them. The separate Player audiobook volume,
PV/PVC and init check have been removed. Ownership is numeric `65534:65534`,
directories are `0755`, and regular files are `0644`.

The two moved trees contain 1,494 and 1,166 regular files respectively.
File counts, inode identities, byte sizes and relative paths matched after
both moves. The podcast tree includes audio, video and images; its `default`
folder is empty, and the collection lives under `headspace`.

Audiobookshelf's Application, chart, service and public/LAN ingresses have
been removed. Its frontend site entry was removed, retiring the generated
relayd/httpd routes, Gogios HTTP/HTTPS/TLS checks, DNS records and ACME
renewal configuration on both frontends.

## Rollback data

No new snapshot was created, as requested. The held snapshot from the
Jellyfin migration already covers the original Audiobookshelf paths:
`zdata/enc/nfsdata@jellyfin-retirement-20261017T101941Z` on f0, with hold
`jellyfin-retirement`. Existing cleanup task `rf3`, due 2026-10-31
10:19:41 UTC, must verify both migrations before releasing that hold and
deleting the snapshot.

Current Player data and Audiobookshelf configuration were separately backed
up while both applications were stopped. The private backup and migration
verification records are under
`~/.local/state/f3s/audiobookshelf-retirement-20261017/` on the controller.
The remaining Audiobookshelf tree is retained at
`/data/nfs/retired/audiobookshelf/20261017/` (root-owned, mode `0700`).

The preserved Audiobookshelf database passed SQLite quick_check. It contains
140 book records, two users and nine listening-progress records; these were
not imported into Player. There were no podcast subscriptions or RSS feeds
to recreate. Existing Player users, permissions and listening progress remain
in its own database.

To roll back, stop Player, restore the two trees to their original paths,
restore the archived Audiobookshelf tree, and restore the chart/Application
and frontend site entry from git history. Restore individual paths from the
held snapshot only if necessary; do not roll back the shared NFS dataset.

## Verification and remaining format limitations

Player rescanned without errors. All 732 existing Audiobooks items retained
their set ID, media IDs and container paths. The new Podcasts set indexes
500 items: 372 audio files, 30 videos and 98 images.

Authenticated API tests passed for browsing, thumbnails, streaming, raw
HTTP 206 downloads, audio/video/image decoding and saved playback progress.
Missing media returned 404; unauthenticated browsing returned 401. A real
Chrome browser passed login, folder navigation, audio playback, seeking,
progress persistence and resume in both sets. Test positions were restored.
Containers running as UID 65534 can read every moved file, write attempts
are rejected, and all three r-nodes see the new library paths.

Two source-format limitations remain, without deleting or converting files:

- Player's extension list excludes 60 visible MPG videos. Follow-up task
  `sf3` in the **player** project covers MPEG indexing and playback support.
- One MP4 fails FFprobe with invalid data and is not indexed. Its original
  is retained; the private `unindexed-files.json` records its path.

Both frontend configurations validate and all services remain running.
Authoritative DNS no longer serves the Audiobookshelf name or its www/standby
aliases. Live and published Gogios configurations have no Audiobookshelf
checks. This retirement did not remove or alter Xplayer's media.
