# Navidrome

Subsonic-compatible music server, `deluan/navidrome:0.64.2` (pinned by
digest), port 4533.
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
the pre-0.63 sharing default), `ND_CONFIGFILE=/config/navidrome.toml` and
`ND_DEVEXTERNALSCANNER=false` (see [SQLite on NFS](#sqlite-on-nfs-unix-excl)). Add music by copying into the music dir on the NFS server;
the hourly scan finds it. Users are managed in the web UI (first visit creates
the admin).

```sh
just status | logs [lines] | port-forward [4533] | sync | argocd-status | restart
```

Clients: play:Sub, substreamer (iOS); DSub, Ultrasonic, substreamer
(Android); Sublime Music, Sonixd.

## SQLite on NFS (unix-excl)

`templates/configmap.yaml` sets the database DSN in the config file:

```toml
DbPath = "file:/data/navidrome.db?vfs=unix-excl&_busy_timeout=15000&_journal_mode=WAL&_foreign_keys=on&synchronous=normal"
```

Why (task 444, 2026-11-08): the r-nodes mount the export over NFSv4.2
through stunnel (`local_lock=none`). With SQLite's default `unix` VFS every
transaction locks `navidrome.db` and its mmap'd WAL index
(`navidrome.db-shm`) through NFS LOCK/LOCKU calls, each flushing and
invalidating the client cache. A copy of the DB on r1 managed about 50 write
and 230 point-read transactions per second that way. 0.63.2 fits in that;
0.64.2's new artwork queue (about 2,800 albums and artists to resolve after
the upgrade) does not, so logins and album queries timed out. That was the
regression in the October trials. Threads hung in
`fcntl -> do_unlk -> nfs_iocounter_wait` on `navidrome.db-shm`.

`vfs=unix-excl` holds one exclusive POSIX lock for the life of the process
and keeps the WAL index in heap memory (no `-shm` file). The connections in
the pool keep normal WAL semantics. The same NFS copy then managed about
12,700 writes/s and 48,000 point reads/s.

Constraints that follow:

- `DbPath` must come from the config file. 0.64.2 has no viper default for
  `dbpath`, so `ND_DBPATH` is silently ignored. The private-cache retry in
  October never took effect for that reason.
- `file:` is not required for `vfs=`: go-sqlite3 v1.14.52 reads `vfs` from
  the query with or without the prefix. Only `cache=shared` needs `file:`.
  Upstream's default has no `file:`, so its `cache=shared` was never in force
  (connections were always private), and it is left out here. `file:` stays
  because this exact string passed the fixtures (one exclusive lock, no
  `-shm`). Dropping it needs that check repeated on a fixture.
- `synchronous=normal` has no effect (go-sqlite3's key is `_synchronous`). It
  is kept as tested; go-sqlite3 sets `synchronous = NORMAL` by default anyway.
- No second process can open the DB while Navidrome runs. The scanner runs
  in-process (`ND_DEVEXTERNALSCANNER=false`, a developer option). No native
  CLI command runs alongside the server. With the pod stopped, `backup`,
  `doctor` and `search` abort with "No existing database":
  `requireExistingDB` cuts `DbPath` at `?` and checks
  `file:/data/navidrome.db`, a path that does not exist. The other DB
  commands (`user`, `pls`, `missing`, `plugin`, `artwork`, `scan`) skip that
  check and work with the pod stopped. An online `sqlite3`
  backup from another process also fails. Back up from a ZFS snapshot of
  `zdata/enc/nfsdata`, or copy the files with the pod stopped.
- A second pod (rollout overlap, node partition) fails with `database is
  locked` and restarts until the lock is free; `Recreate` avoids overlap.

Copied-DB fixture on the same NFS export, 0.64.2, this config, chart pod spec:
startup 4 s (1 min 26 s with the default DSN), login 0.06 s, wrong password
401, existing song, playlist, star, play queue and the first 64 KiB of the
stream unchanged, album list 0.27 s, and a 12-minute soak without errors.
With the default DSN the same copy timed out on login. Evidence:
`~/.local/state/conf/444-navidrome-20261108/`.

## Upgrade and rollback

0.64 migrates the schema (canonical IDs, artwork tables). Rolled out on
2026-11-09 (11:33 UTC, conf push `1fc3dfa`). A one-time `pre-upgrade-backup`
init container copied the closed 0.63.2 DB to
`/data/navidrome-before-0.64.2.db` and compared the copy before the first
0.64.2 start. It was removed once the rollout was verified; the backup file
stays (integrity ok, schema 20260703013908). The f0 snapshot
`zdata/enc/nfsdata@444-navidrome-before-0642-20261109` (hold
`444-navidrome-upgrade`) was taken just before. Release the hold once
rollback is no longer wanted, because holds block zrepl pruning.

Verified after the rollout:
- Argo Synced/Healthy; pod Ready on digest `38dc2727`, 0 restarts.
- `Loaded configuration file=/config/navidrome.toml`; migration to 20260922230428.
- Startup 3.8 s, no error lines.
- On the node, one exclusive POSIX lock on `navidrome.db` and no `-shm`.
- `/ping` 200; a wrong login was rejected with 401 in 0.04 s.
- Both ingress hosts answer.

0.64 changes song and playlist IDs to canonical ones. Library IDs, users,
passwords, stars, play counts and play queues are kept.

Rollback to 0.63.2:

1. Disable Argo auto-sync for `navidrome`, scale the Deployment to 0.
2. In `/data/nfs/k3svolumes/navidrome/data`: move `navidrome.db`, `-wal` and
   `-shm` aside, copy `navidrome-before-0.64.2.db` to `navidrome.db` (and its
   `-wal` if present).
3. Revert the chart commit (image, configmap, env), push, re-enable auto-sync.

Writes made under 0.64.2 (plays, stars, playlists, users) are lost on
rollback. Never roll back the shared NFS dataset for this.

## Lyrics

Navidrome 0.64.2 reads existing lyrics from matching sidecar files or embedded
metadata. Its default `LyricsPriority` is
`.ttml,.yaml,.yml,.elrc,.lrc,.srt,.txt,embedded`; the first available source wins.
For `Artist/Album/Song.mp3`, place `Song.lrc` or `Song.txt` in that same folder.
LRC provides line timestamps; plain text is available through the lyrics API.
The bundled web player renders synchronized lyrics.
Richer formats can include word timing and multiple voices. The web player and
OpenSubsonic lyrics API consume these local sources. Navidrome does not fetch
missing lyrics automatically. The shared Beets job currently fetches artwork
only (`fetchart embedart`).

Historical (October 2026 trials, superseded by
[Upgrade and rollback](#upgrade-and-rollback)): a pre-upgrade SQLite backup is
retained in `/data` as `navidrome-before-0.64.2-20261015.db` (SQLite online
backup, quick_check passed). It was the restore point for those trials. For
the current upgrade, the rollback source is `navidrome-before-0.64.2.db`,
written by the (since removed) init container.

The 2026-10-15 trial of 0.64.2 stalled authentication and API requests while
its new artwork workers contended in SQLite on this deployment. Private-cache and rollback-
journal trials did not resolve that (the private-cache DSN was set through the
ignored `ND_DBPATH`). The deployment then stayed on 0.63.2 until task 444
found the cause; see [SQLite on NFS](#sqlite-on-nfs-unix-excl).
The trial database is preserved as
`/data/navidrome-after-0.64.2-trial-20261015.db` before restoring the pre-upgrade
backup and migrating it to 0.63.2.

Verified on 2026-10-15: ArgoCD Synced/Healthy, normal account login, album
browsing and real audio playback, an existing track with 98 synchronized lyric
lines through `getLyricsBySongId`, lyric display and seek updates in the web
player, an empty result for a track without lyrics, and the expected error for
a missing track. The temporary verification account was removed by restoring
the pre-upgrade database. No music files were modified.

On 2026-10-17, another 0.64.2 trial passed `/ping` but an authenticated web
login timed out after 20 seconds. The temporary account was removed with the
native user CLI while the server was stopped. The deployment returned to
0.63.2 with the database and WAL restored from f0's held snapshot
`zdata/enc/nfsdata@zrepl_20261017_084600_000` (08:46 UTC, before the upgrade).
Database changes after that snapshot are absent from the restored database;
the music volume was not changed. The failed trial's database files remain
under `/data/qf3-trial-0.64.2-20261017/`, and its stopped-service snapshot is
`zdata/enc/nfsdata@qf3-navidrome-failed-0642-20261017` (held).

After restoring 0.63.2, a temporary account passed web login (0.04 seconds),
Subsonic album/song browsing, and an original audio stream (65,536 bytes).
Invalid credentials and a missing track were rejected. The account was then
deleted with the native CLI while the server was stopped.

Rechecked upstream on 2026-10-19 and again on 2026-11-08: 0.64.2 is still the
latest stable release (digest `sha256:38dc2727...`).
