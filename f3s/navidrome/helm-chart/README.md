# Navidrome

Subsonic-compatible music server, `deluan/navidrome:0.64.2`, port 4533.
Namespace `services`, ArgoCD app `navidrome`, runs on any r-node (both
volumes are NFS hostPath; shared music PVC with
[beets-art](../../beets-art/helm-chart/README.md)).

| Ingress | Path |
|---|---|
| `https://navidrome.f3s.buetow.org` | OpenBSD relayd, WireGuard, Let's Encrypt |
| `https://navidrome.f3s.lan.buetow.org` | CARP VIP 192.168.1.138, self-signed LAN CA ([LAN access](../../docs/lan-access-setup-guide.md)) |

| Volume | Size | Mount |
|---|---|---|
| `/data/nfs/k3svolumes/navidrome/data` (`navidrome-data-nfs-pvc`) | 10Gi | `/data` (SQLite DB) |
| `/data/nfs/k3svolumes/navidrome/music` (`navidrome-music-pvc`, RWX) | 200Gi | `/music` |
| node-local `emptyDir` | 1Gi | `/cache` (`ND_CACHEFOLDER`, image/transcode cache, disposable) |

Until 2026-09-25 `/data` was a local-path PVC on r1 with a `nodeSelector`;
it moved back to NFS so r1 is no longer a single point of failure, with the
cache kept node-local (the reason it had left NFS: ~19 s image-cache init).

Env in `templates/deployment.yaml`: `ND_SCANSCHEDULE=1h`, `ND_LOGLEVEL=info`,
`ND_BASEURL=""`, `ND_CACHEFOLDER=/cache`, `ND_ENABLESHARING=false` (preserves
the pre-0.63 sharing default). Add music by copying into the music dir on the NFS server;
the hourly scan finds it. Users are managed in the web UI (first visit creates
the admin).

```sh
just status | logs [lines] | port-forward [4533] | sync | argocd-status | restart
```

Clients: play:Sub, substreamer (iOS); DSub, Ultrasonic, substreamer
(Android); Sublime Music, Sonixd.

## Lyrics

Navidrome 0.63.2 reads existing lyrics from matching sidecar files or embedded
metadata. Its default `LyricsPriority` is
`.ttml,.yaml,.yml,.elrc,.lrc,.srt,.txt,embedded`; the first available source wins.
For `Artist/Album/Song.mp3`, place `Song.lrc` or `Song.txt` in that same folder.
LRC provides line timestamps; plain text is available through the lyrics API.
The bundled web player renders synchronized lyrics.
Richer formats can include word timing and multiple voices. The web player and
OpenSubsonic lyrics API consume these local sources. Navidrome does not fetch
missing lyrics automatically. The shared Beets job currently fetches artwork
only (`fetchart embedart`).

A pre-upgrade SQLite backup is retained in `/data` as
`navidrome-before-0.64.2-20261015.db` (SQLite online backup, quick_check passed).
A rollback across schema migrations requires restoring that backup, not only
switching the image tag.

The 0.64.2 trial stalled authentication and API requests while its new artwork
workers contended in SQLite on this deployment. Private-cache and rollback-
journal trials did not resolve that. This deployment uses 0.64.2, which includes
the structured lyrics support from 0.63 without the 0.64 artwork changes.
The trial database is preserved as
`/data/navidrome-after-0.64.2-trial-20261015.db` before restoring the pre-upgrade
backup and migrating it to 0.63.2.

Verified on 2026-10-15: ArgoCD Synced/Healthy, normal account login, album
browsing and real audio playback, an existing track with 98 synchronized lyric
lines through `getLyricsBySongId`, lyric display and seek updates in the web
player, an empty result for a track without lyrics, and the expected error for
a missing track. The temporary verification account was removed by restoring
the pre-upgrade database. No music files were modified.
