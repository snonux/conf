# Player v0.18.0 image acceptance — 2026-11-08

Both Player and xplayer use registry.lan.buetow.org:30001/player:7466f0e.
Source release v0.18.0:7466f0e8bb48c8e4db3e68cccafa00c7179ac746.
OCI config ID:sha256:cad1ae66cc3f5c9c7671d4b151e8f5dd0060b9986d91bfd9edce31d72b16638a.
Registry manifest:sha256:b45891f1552397e2c5f8f41f8cc212e79c505c7b92157d25800d38bc5f89fbd5.

The adjacent Dockerfile refreshes the upstream release's builder/runtime to
pinned Go1.26 Alpine and Alpine3.24.4 bases. Build context is that exact
release's player-server directory, with its .dockerignore; never the monorepo
or a live media directory. The accepted build used --cpu-period100000,
--cpu-quota200000, --memory8g, labels org.opencontainers.image.version=0.18.0
and org.opencontainers.image.revision=<full source SHA>. The Dockerfile limits
Go compilation to2processes. Existing tags/latest were not overwritten; verify
absence before publishing a NEW immutable tag when building again.

Offline release JS118 and full Go-race tests passed. Production repository
Open plus retention startup ran on consistent copies of BOTH databases:
all41tables' original-column fingerprints preserved; two retention columns
added. Disposable nonroot image health/ready/version/auth-negative tests passed.
Actual image config ID was verified in all3nodes before stopping either writer.

Each instance was upgraded serially after a stopped-writer backup and held
ZFS snapshot. Both now report0.18.0 with existing login, sets and ranged media
bytes matching baseline, users-row fingerprints unchanged and SQLite checks OK.
Xplayer alltable counts unchanged. Player's startup scan found43 real additional
files (media4934→4977), removed no old rows, and changed only one file-size,
four thumbnail-path and four music metadata fields in pre-existing rows.
Changed size matches the actual file; users/sets/progress/podcast catalog counts
preserved. This documented scan activity is not claimed as exactalltable equality.

Snapshot names on f0's zdata/enc/nfsdata:

- j34-before-xplayer-v018-20261108, holdj34-xplayer-v018-upgrade
- j34-before-player-v018-20261108, holdj34-player-v018-upgrade

NEVER roll back the shared NFS dataset. Recovery requires quiescing the selected
instance and restoring its selected data/tree, preserving post-upgrade writes;
coordinate with schema compatibility. Private DB archives, hashes, native
receipts and phase journals are under
~/.local/state/conf/j34-app-upgrades-20261107/player-v018-*.
Those files contain private user data and must never enter Git.

Chart/source commit5b7e22e was published to GitHub+Forgejo before restoring
both original automatic-sync policies. Both apps accepted Synced/Healthy and
all40appsHealthy/3nodesReady. Full j34 audit/upgrades/review remains incomplete.
