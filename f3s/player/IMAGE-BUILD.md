# Player image acceptance

## v0.19.0 — 2026-11-09 (task j54)

Player and xplayer run registry.lan.buetow.org:30001/player:4cf7f5d, built from
source release v0.19.0:4cf7f5d1f8927c9947dc6b55a4c72c6807265869.
OCI config ID: sha256:07fbcac00de0744c642d8f521ec6569d80fd670be06996fe53eff4ed9ce92d00.
Registry manifest: sha256:c4b2f8dc3217151767fd95a6b56b248593631ddf23313eeb8b903295b83a3734.

The adjacent Dockerfile was not changed. Its bases were still the newest:
golang:1.26-alpine resolved to the same digest (Go 1.26.10), and Alpine
3.24.4 was the latest release. The build context was the exact release's
player-server directory, with `--cpu-period=100000 --cpu-quota=200000
--memory=8g` and the labels org.opencontainers.image.version=0.19.0 and
org.opencontainers.image.revision=<full SHA>. The new tag was published
only after the registry answered 404 for it. latest was not touched.

Checks before the live upgrade:

- The full Go race suite (PLAYER_REQUIRE_FFMPEG=1) and the 117 web JS tests
  passed.
- A disposable nonroot fixture passed: version, health/ready, UID 65534 and
  401s. It also checked the new cross-origin guard: a cross-site POST with
  the session cookie gets 403, a same-origin logout gets 204.
- The exact config ID was found on r0, r1 and r2.
- v0.19.0 opened consistent copies of both databases twice. All 41 tables
  kept every original row and column. The only new object is the
  `schema_migrations` bookkeeping table. Downgrades are not supported.

Rollout was serial, xplayer first. For each instance, under the live-rollout
lock:

1. Hold Argo automated sync for that app only.
2. Stop the writer.
3. Take a held f0 snapshot.
4. Archive the stopped database, download it and check its SHA.
5. Push the chart (conf 073f34d for xplayer, e043383 for player).
6. Restore the exact `{prune: true, selfHeal: true}` policy, so Argo syncs.

Both instances then passed these checks:

- They report 0.19.0 and run the exact config ID.
- The existing login, set IDs and ranged media bytes match the baseline.
- The cross-site POST gets 403.
- The users-row fingerprint and every table count are unchanged.
- SQLite quick_check is ok.
- No active media row was retired.

Snapshots on f0's zdata/enc/nfsdata:

- j54-before-xplayer-v019-20261109, hold j54-xplayer-v019-upgrade
- j54-before-player-v019-20261109, hold j54-player-v019-upgrade

From v0.19.0 on, an admin full Rescan soft-deletes the media of a set whose
directory has gone. The startup inventory does not. Every set row had its
directory at upgrade time.

## v0.18.0 — 2026-11-08 (task j34, superseded)

The image was player:7466f0e, config sha256:cad1ae66..., manifest
sha256:b45891f1.... Its snapshots j34-before-{xplayer,player}-v018-20261108
are still held.

## Recovery

NEVER roll back the shared NFS dataset. To recover, stop the selected
instance and restore only its data or tree. Keep the writes made after the
upgrade, and check that the schema is compatible. The private DB archives,
hashes and receipts are under
~/.local/state/conf/j54-player-v019-20261109/ (v0.19) and
~/.local/state/conf/j34-app-upgrades-20261107/player-v018-* (v0.18). They
contain private user data and must never enter Git.
