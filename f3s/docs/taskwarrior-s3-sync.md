# Taskwarrior S3 sync: feasibility on the Fedora laptop

Investigation of whether Taskwarrior can replace the current Syncthing-based
task sync with a native S3 backend, and whether the version we run is recent
enough. Written 2026-09-01.

## Decision (2026-09-01)

**Stock Fedora `task` package. No custom build.** Off-LAN sync reaches Garage
through the OpenBSD frontends and relayd, and Garage is reconfigured to serve
virtual-hosted-style requests so the stock package can address it by hostname.

| | |
|---|---|
| Package | Fedora's `task` (3.4.2), **not** a custom build |
| Bucket | `taskwarrior-garage` |
| Endpoint | `https://f3s.buetow.org` |
| Garage | `root_domain = ".f3s.buetow.org"` |
| Hostname on the wire | `taskwarrior-garage.f3s.buetow.org` |

**The hostname is not freely chosen.** The AWS SDK derives it as
`<bucket>.<endpoint-host>`, so the hyphen has to come from the *bucket name* —
hence bucket `taskwarrior-garage` with `f3s.buetow.org` as the endpoint, rather
than bucket `taskwarrior` with `garage.f3s.buetow.org` (which would produce the
four-label `taskwarrior.garage.f3s.buetow.org`). The three-label form fits the
existing `@f3s_hosts` pattern in `frontends/Rexfile`, which auto-generates DNS
records, Let's Encrypt certs and relayd keypairs.

Tested before committing to it:

* Stock 3.4.2 + hostname endpoint + Garage with `root_domain` → full
  two-replica round-trip.
* Genuinely virtual-hosted, not a silent fallback: Garage logged `GET /salt`
  and `GET /?list-type=2` with **no bucket in the path**.
* The hyphenated shape works: bucket `taskwarrior-garage`, endpoint
  `test.local`, `root_domain = ".test.local"` → round-trip.
* **The bare endpoint host is never contacted.** Sync still succeeded with
  `test.local` removed from resolution entirely, so the SDK only ever talks to
  `<bucket>.<endpoint>`. Using `f3s.buetow.org` as the endpoint therefore has
  zero interaction with the `f3s.buetow.org` landing page, which relayd routes
  to `<f3s_static_proxy>`.
* **Enabling `root_domain` is non-breaking.** Path-style access still works
  alongside it (`GET /taskwarrior/…`, bucket in the path), so the existing
  `watchos-app` bucket consumer is unaffected.

Rejected alternatives:

* **WireGuard IP routing** — would work with the stock package (an IP endpoint
  gets path-style for free), but off-LAN traffic is to go through the frontends,
  not direct over wg.
* **Custom build with `sync.aws.force_path_style`** — proven working against the
  bare `garage.f3s.buetow.org` with no DNS, cert or relayd changes at all, but
  means maintaining a forked package for the rest of its life. Kept as the
  fallback if the vhost route hits an obstacle.

Known trade-off: with vhost-style the bucket name becomes part of the DNS name,
so each future bucket needs its own `@f3s_hosts` entry, DNS records and cert —
acme-client uses HTTP-01 and cannot issue wildcards. And `root_domain =
".f3s.buetow.org"` makes Garage read *any* `Host: X.f3s.buetow.org` as bucket
`X`; that is contained only because relayd forwards just the matched hosts to
the `<garage>` table.

Implementation steps live in task `f91`.

## Verdict

**Yes — and the stock Fedora package is enough on the LAN.** A custom build is
only needed for the off-LAN path.

1. The laptop runs **task 2.6.2**, which has no cloud sync backend at all.
2. The newest packaged version, **task 3.4.2** (Fedora 44 — and 43, 45 and
   Rawhide), has an AWS S3 backend but lacks `sync.aws.endpoint_url` and
   `sync.aws.force_path_style`, which landed upstream on 2026-08-19, **after**
   the v3.5.0 tag.
3. **That turns out not to matter on the LAN.** The AWS SDK's own
   `AWS_ENDPOINT_URL_S3` environment variable redirects stock 3.4.2 at Garage,
   and because the endpoint is an IP literal the SDK uses path-style addressing
   automatically. Tested: a full two-replica round-trip. **No build required.**
4. **It does matter off-LAN.** With a hostname endpoint
   (`garage.f3s.buetow.org`) the SDK switches to virtual-hosted addressing and
   fails DNS. Only `sync.aws.force_path_style` fixes that — so the custom build
   is needed for the off-LAN case, not for the LAN one.
5. **A custom build from upstream `develop` has both settings** and works over
   both paths — LAN and the public relayd edge — in both directions. Built in a
   throwaway Fedora 44 container in ~6.5 minutes. See
   [Custom build](#custom-build-verified-working) and
   [Sync round-trip against Garage](#sync-round-trip-against-garage--verified).

So the decision is narrower than it first looked, and the custom build is a
**last resort rather than the plan**. Three routes reach the same place, and two
of them use the stock Fedora package:

| Route | Cost | Stock package? |
|---|---|---|
| Garage `root_domain` (vhost-style) | Garage config + DNS + cert + relayd rule | ✅ tested |
| Garage by IP over WireGuard | `AllowedIPs` + forwarding on f3 | ✅ tested (by IP) |
| Custom build (`force_path_style`) | a forked package to rebuild forever | ❌ |

## Current state

### Laptop

| | |
|---|---|
| Binary | `/usr/bin/task`, version **2.6.2** |
| Package | `task2-2.6.2-6.fc44.x86_64` (Fedora's compat package) |
| Config | `~/.taskrc`, `data.location=/home/paul/.task` |
| Data dir | `~/.task` -> `~/Documents/Taskwarrior` -> `~/syncthing/Documents/Taskwarrior` |
| Sync today | **Syncthing**, i.e. file-level sync of the raw 2.x `*.data` files |

The Syncthing approach is what motivated this investigation. At the start of
this investigation `~/.task` held **12** `*.sync-conflict-*.data` copies dated
between 2026-06-13 and 2026-07-16 — five of `backlog.data`, four of
`pending.data`, two of `undo.data` and one of `completed.data`. (They were
removed by Syncthing partway through the session; the folder has no file
versioning configured, so there is no local copy left. The live database itself
was unaffected.)

Those conflict copies are the whole problem: Syncthing has no idea how to merge
Taskwarrior records, so concurrent edits on two machines produce conflict copies
instead of a merge — exactly the weakness the upstream man page calls out under
"ALTERNATIVE: FILE SHARING SERVICES" ("Tasks are not properly merged").

### Rest of the fleet

| Host | Taskwarrior | Notes |
|---|---|---|
| Fedora laptop | 2.6.2 (`task2`) | this investigation |
| macOS work laptop | (unchanged) | **stays on local file storage — no remote sync backend, by decision** |
| OpenBSD frontends | 2.6.2p1 from ports | configured by `frontends/Rexfile` -> `frontends/etc/taskrc.tpl`, daily reminder via `scripts/taskwarrior.sh.tpl` |

OpenBSD ports is still on 2.6.2p1, so the frontends cannot follow a move to 3.x
by package. They would either drop out of the sync mesh or need a from-source
build (Taskwarrior 3.x pulls in a Rust toolchain for TaskChampion).

## What supports what

Upstream history, from the `CmdSync.cpp` commit log and release tags:

| Capability | Landed | First release |
|---|---|---|
| AWS S3 sync (`sync.aws.*`) | 2024-12-17 | **v3.3.0** (2024-12-19) |
| Git sync | 2026-06-07 | **v3.5.0** (2026-08-16) |
| S3-**compatible** endpoints (`sync.aws.endpoint_url`, `sync.aws.force_path_style`) | 2026-08-19 | **unreleased** (post-3.5.0, i.e. 3.6.0) |

Config keys actually compiled into the Fedora 44 `task-3.4.2-3.fc44` binary
(`strings /usr/bin/task | grep '^sync\.'`):

```
sync.aws.access_key_id      sync.gcp.bucket
sync.aws.bucket             sync.gcp.credential_path
sync.aws.default_credentials
sync.aws.profile            sync.local.server_dir
sync.aws.region             sync.server.client_id
sync.aws.secret_access_key  sync.server.origin   (deprecated)
sync.encryption_secret      sync.server.url
```

`sync.aws.endpoint_url` and `sync.aws.force_path_style` are **absent** from that
binary, confirming 3.4.2 predates S3-compatible support.

Fedora 44 offers exactly one version: `task.x86_64 3.4.2-3.fc44`. There is no
3.5.x or 3.6.x to upgrade to — and not just on 44: Fedora 43, 45 and **Rawhide**
are all on 3.4.2 as well. So neither git sync (3.5.0) nor S3-compatible
endpoints (3.6.0) are reachable from Fedora packaging today, on any release.

## Why Garage will not work with 3.4.2 (but does with a custom build)

Even ignoring the missing `endpoint_url` key, our Garage deployment is
path-style only:

* `f3s/garage/etc/garage.f0.toml` sets `[s3_api] s3_region = "garage"` and
  **no `root_domain`**. Without `root_domain`, Garage cannot resolve
  virtual-hosted-style requests (`<bucket>.garage.f3s.buetow.org`) to a bucket.
* The edge only routes the bare host: `frontends/etc/relayd.conf.tpl` matches
  `Host: garage.f3s.buetow.org` and forwards to the `<garage>` table on port
  3900. A `<bucket>.garage.f3s.buetow.org` request matches no rule.
* TLS is terminated by relayd for `garage.f3s.buetow.org`. A wildcard for
  `*.f3s.buetow.org` does not cover `<bucket>.garage.f3s.buetow.org` — DNS
  wildcards are single-label — so vhost-style would also need a second cert.

### …except it does work, via the SDK environment variable

The above reasons from the config keys alone. **Tested, the stock Fedora 3.4.2
package does sync to Garage** — the missing `sync.aws.endpoint_url` is not
actually fatal.

TaskChampion builds its S3 client with `aws_config::defaults(BehaviorVersion::latest())`,
which loads the AWS Rust SDK's standard configuration chain. That chain honours
`AWS_ENDPOINT_URL_S3` (and the global `AWS_ENDPOINT_URL`), so the endpoint can
be redirected without any Taskwarrior setting at all:

```sh
$ AWS_ENDPOINT_URL_S3=http://192.168.1.130:3900 task sync
Syncing with AWS bucket taskwarrior
Success!
Sent 5 local operations to the server
```

A second stock replica then pulled the task straight back down. Both env var
names work.

**Why `force_path_style` turns out not to be needed here:** S3 endpoint
resolution falls back to path-style addressing when the endpoint is an **IP
literal**, because a bucket name cannot be prefixed onto an IP. Addressing
Garage as `http://192.168.1.130:3900` therefore gets path-style for free.

**The catch — this only works by IP.** With a hostname the SDK uses
virtual-hosted addressing and the request never leaves the machine:

```sh
$ AWS_ENDPOINT_URL_S3=https://garage.f3s.buetow.org task sync
unhandled error: dispatch failure: io error: error trying to connect:
dns error: failed to lookup address information: Name or service not known
```

That is it trying to resolve `taskwarrior.garage.f3s.buetow.org`.

**But that is a Garage configuration gap, not a Taskwarrior one.** Garage can
serve virtual-hosted style; it just is not set up for it. With `root_domain`
set, the stock package syncs over a **hostname** endpoint too — tested against a
local Garage v1.0.1 with `root_domain = ".s3.test.local"`, a full two-replica
round-trip. It was genuinely vhost-style, not a silent fallback: Garage logged

```
GET /salt?x-id=GetObject
GET /?list-type=2&prefix=s-
```

with **no bucket in the path** — path-style would be `/taskwarrior/salt` — so the
bucket was resolved from the Host header.

So `sync.aws.force_path_style`, and the custom build that supplies it, is needed
only if Garage cannot be reconfigured. Enabling vhost-style on the real cluster
costs: `root_domain` in `f3s/garage/etc/garage.f0.toml` (+ f1/f2) and a
`rex garage_deploy`; a DNS record for `taskwarrior.garage.f3s.buetow.org`; TLS
cert coverage for that name (the `*.f3s.buetow.org` wildcard does **not** cover
it — DNS wildcards are single-label, so prefer `*.garage.f3s.buetow.org`); and a
relayd match rule, since `frontends/etc/relayd.conf.tpl` matches only the bare
host today. The trade-off is that the bucket name becomes part of the DNS name,
so each future bucket needs a record and cert coverage.

**Shared config does not help.** `endpoint_url` in a `~/.aws/config` profile is
ignored, and so is the documented `[services]` / `s3 = endpoint_url` form —
both fall back to real AWS and fail DNS. Only the environment variable works,
which means it has to be set by a wrapper or shell config rather than living in
`taskrc`.

Making *3.4.2* + Garage work would mean setting `root_domain` in Garage, issuing
a second-level wildcard cert, and adding relayd rules — a lot of homelab surgery
to work around one missing setting. Note that a
[custom build](#custom-build-verified-working) avoids all of it: with
`force_path_style` available, path-style-only Garage is usable exactly as it is
configured today, with no Garage, DNS, cert or relayd changes at all.

## Cost of the 2.6.2 -> 3.x upgrade

Worth knowing regardless of which sync backend wins, because *every* 3.x sync
option requires this migration first.

* **Package conflict.** `task2` and `task` both own `/usr/bin/task`. Installing
  `task-3.4.2` removes `task2`, which is a dependency of the installed
  `vit-2.3.4-1.fc44` package.
* **No automatic migration.** Pointing 3.4.2 at a 2.x data directory does *not*
  import the old files: it silently creates an empty `taskchampion.sqlite3` and
  reports `count` = 0. This matches the upstream 3.0.0 release note — "users
  must export their task database from 2.x and re-import it into 3.x" — so the
  migration is an explicit `task export` (2.6.2) followed by `task import` (3.x).
  Upstream also warns that **hooks run during `task import`** and should be
  disabled for the operation; we have `hooks=1` but no `~/.task/hooks`
  directory, so there is nothing to disable today.
* **The formats are incompatible, but the migration is reversible.** 2.6.2
  keeps flat files (`pending.data`, `completed.data`, `undo.data`,
  `backlog.data`); 3.x keeps `taskchampion.sqlite3`. Neither reads the other's,
  and pointing 3.x at a 2.x directory reports **0 tasks with no warning** while
  creating its own empty database alongside — see
  [Format compatibility](#format-compatibility-tested-both-directions) for what
  was actually tested. JSON export/import bridges them in **both** directions,
  so a migration can be walked back; what does not survive either way is the
  undo history.
* **`ask` / hexai — verified compatible.** The `ask` CLI
  (`github.com/snonux/hexai` v0.42.6) shells out to whatever `task` is first in
  `PATH` (`exec.LookPath("task")`) and parses `task export` JSON. It was run
  against the migrated 3.4.2 database and worked unmodified; see the rehearsal
  below.
* **Syncthing must be retired for this data** at the same time, otherwise
  Syncthing keeps file-syncing a SQLite database between machines, which is
  strictly worse than what we have now.

### Migration rehearsed (measured)

The migration was actually run, against a **copy** of the live database in a
scratch directory — the extracted 3.4.2 binary, its own `TASKRC` and its own
`data.location`. `~/.task` was never written to.

| Step | Result |
|---|---|
| `task export` from 2.6.2 | 0.26s, 17,868 records, 12 MB of JSON |
| `task import` into 3.4.2 | **15m 32s** |
| Resulting `taskchampion.sqlite3` | 69 MB |
| Counts after | 242 pending / 9,124 completed / 8,396 deleted — **identical to 2.6.2** |
| `ask` against 3.4.2 | **works unmodified** — `ask ready` rendered correctly, including the started flag |

Two things to take away:

* **The data survives intact**, and `ask` needs no change: it found the 3.4.2
  binary via `PATH` and parsed its export JSON without complaint. That removes
  the biggest unknown from the upgrade.
* **The import is slow and gets slower** as the database grows — a quarter of an
  hour for ~18k tasks, on an SSD. Budget for it, run it once, and do not plan to
  repeat it. (Most of that bulk is history: only 242 of the 17,868 records are
  pending. Importing only pending+completed would be far quicker if the deleted
  tasks are expendable.)
* `undo.data` is **not** carried over — the 2.x undo history (120 MB of it) is
  lost at the boundary. That is inherent to the format change, not a bug.


## Format compatibility (tested, both directions)

The 2.6.2 and 3.x databases are **not** compatible, and the failure mode is
silent rather than loud.

Pointing 3.5.0 at a directory holding a 2.6.2 database:

```
$ task rc:rc3 count        # data.location = a 2.6.2 data dir
0
$ ls
backlog.data  completed.data  pending.data  taskchampion.sqlite3  undo.data
```

It reports **zero tasks**, prints no warning, and creates its own empty
`taskchampion.sqlite3` beside the flat files. Anyone who installs 3.x over
`task` without migrating will conclude their task list has been destroyed.

It is, however, **non-destructive**: running 2.6.2 against the same directory
afterwards still shows all its tasks. The two versions coexist in one directory
as two entirely independent task lists — a footgun, not a feature.

The bridge is JSON, and it works **both ways**:

| Direction | Result |
|---|---|
| 2.6.2 → 3.4.2 | 17,868 records, counts identical (242/9,124/8,396); 15m32s |
| 3.5.0 → 2.6.2 | tasks, **annotations** and **dependencies** all survived; completed status preserved |

For the downgrade test, 3.5.0 exported `depends` as a JSON *list*; 2.6.2's
`import` accepted it and the dependency relationship came through intact. So
migrating to 3.x is **reversible** — the one thing that does not survive in
either direction is the undo history, since each format keeps its own.

## Custom build (verified working)

Since no release has the S3-compatible settings, the remaining route is to build
upstream `develop` ourselves. This was actually done, in an **isolated
container** so nothing on the laptop was touched:

```sh
# build.sh, run inside a throwaway Fedora 44 container
dnf install -y gcc-c++ cmake make git rust cargo libuuid-devel
git clone --depth 50 --branch develop \
  https://github.com/GothenburgBitFactory/taskwarrior.git /src
cd /src
cmake -S . -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build -j"$(nproc)"
```

```sh
podman run --rm -v "$PWD":/work:z -v "$PWD":/out:z fedora:44 bash /work/build.sh
```

Results:

| | |
|---|---|
| Upstream `develop` HEAD | `d4a78896` (2026-08-28) |
| Build time | **~6.5 min** total, including `dnf install` and the clone |
| Version string | reports `3.5.0` — `develop` has not bumped it to 3.6.0 yet |
| Binary size | 56 MB (unstripped) |
| Runs on the host? | **yes** — built against Fedora 44's own glibc, so it runs natively |

The sync keys the custom build has that 3.4.2 does not:

```
sync.aws.endpoint_url        <- the one we need
sync.aws.force_path_style    <- the one we need
sync.git.branch              <- git sync (3.5.0), a bonus
sync.git.git_path
sync.git.local_only
sync.git.local_path
sync.git.remote
```

And it accepts the Garage-shaped configuration, verified against the migrated
database:

```
$ task rc.sync.aws.endpoint_url=https://garage.f3s.buetow.org \
       rc.sync.aws.force_path_style=true \
       rc.sync.aws.region=garage rc.sync.aws.bucket=taskwarrior show
sync.aws.bucket            taskwarrior
sync.aws.endpoint_url      https://garage.f3s.buetow.org
sync.aws.force_path_style  true
sync.aws.region            garage
$ task status:pending count
242
```

### Sync round-trip against Garage — verified

This was subsequently tested for real, against the homelab Garage cluster
(f0/f1/f2, all three nodes healthy, `cargo:2.3.0`).

Setup on f0 (the `garage` CLI needs `doas`, as `/usr/local/etc/garage.toml`
is not world-readable):

```sh
doas garage bucket create taskwarrior
doas garage key create taskwarrior-sync
doas garage bucket allow --read --write --owner taskwarrior --key taskwarrior-sync
```

Two throwaway replicas were then pointed at it with the custom build:

| Step | Result |
|---|---|
| P: `task add` + `task sync` | `Success! Sent 6 local operations to the server` |
| Q: `task sync` | pulled P's task down |
| Q: complete P's task, add its own, `task sync` | `Success! Sent 10 local operations` |
| P: `task sync` | saw **both** Q's new task and Q's completion |
| Q via `https://garage.f3s.buetow.org` (relayd + TLS) | `Success!` |
| P via `http://192.168.1.130:3900` (LAN direct) | picked up what Q pushed through the edge |

So **both** access paths work and share one bucket state: LAN-direct to
`f0:3900`, and the public edge where relayd terminates TLS and forwards to the
`<garage>` table. The off-LAN path — the one that actually matters for a laptop
— needs no new edge configuration; the existing `garage.f3s.buetow.org` rule is
enough. (`GET /` on that host returns 403, which is just Garage refusing an
unsigned anonymous request.)

**Path-style was the whole ballgame**: with `sync.aws.force_path_style=true`,
Garage as currently deployed — no `root_domain`, one relayd rule, one cert — is
usable unchanged. No Garage, DNS, TLS or relayd work is needed.

**The stored objects are opaque.** The bucket holds `latest`, `salt` and a chain
of `v-<parent>-<child>` version objects. Fetching a version object back over the
S3 API and grepping it finds no task text — TaskChampion encrypts client-side
with `sync.encryption_secret`, so Garage (and anyone with bucket access) sees
ciphertext only. Verified: the strings `hello` and `replica` from the test task
descriptions are absent from the stored bytes.

The test objects were deleted afterwards, so the bucket is **empty and ready for
real use**. Note that the test used a throwaway `sync.encryption_secret`; a real
deployment must pick its own, and everything in the bucket must share it.

Before this was run against the homelab, the identical procedure was rehearsed
against a **single-node Garage v1.0.1 in a local container** configured the same
way (no `root_domain`, so path-style only). It behaved identically, which makes
it a cheap way to test changes without touching the cluster.

### Packaging gotchas found while testing

* **The bare binary is not enough.** 3.5.0 moved the default theme into a
  separate data file, so a copied-out `task` binary dies with
  `Could not find file in CWD, directory of config file or search paths
  'default.theme'`. It needs a real `cmake --install` (or an RPM) that also
  places `/usr/share/task/*`. Copying just the executable is not a deployment.
* **Still collides with `task2`/`vit`.** A custom build installed to `/usr/local`
  sidesteps the RPM file conflict, but then `/usr/local/bin/task` vs
  `/usr/bin/task` ordering in `PATH` decides which one `ask` and `vit` get.
  Better to build a **proper RPM** that `Obsoletes: task2`, and serve it from our
  own repo — `f3s/pkgrepo/` already serves `/freebsd/`, `/openbsd/`, `/netbsd/`
  and `/rockylinux/`, so a `/fedora/` location is a small addition to
  `helm-chart/templates/configmap-nginx.yaml`.
* **You own the maintenance.** An unreleased `develop` build gets no security or
  correctness updates unless we rebuild it, and it holds every task we have.

## Options


### 1. Do nothing (keep Syncthing)

Zero work, keeps the conflict files. Acceptable only as a holding position.

### 2. Custom build now, then S3 -> Garage  <- gets you what you asked for

Proven to build and to accept the configuration (see
[Custom build](#custom-build-verified-working)). Do it as an **RPM** rather than
a loose binary, so `task2`/`vit`/`PATH` stay sane and the theme data files land
in the right place. Then:

```
task config sync.encryption_secret     <secret>
task config sync.aws.region            garage
task config sync.aws.bucket            taskwarrior
task config sync.aws.endpoint_url      https://garage.f3s.buetow.org
task config sync.aws.force_path_style  true
task config sync.aws.access_key_id     <key>
task config sync.aws.secret_access_key <secret>
```

The bucket, the key and the round-trip are **already done and verified** — the
`taskwarrior` bucket and `taskwarrior-sync` key exist on the cluster and the
bucket is empty, waiting for a real `sync.encryption_secret`. What is left is
packaging (an RPM rather than a loose binary) and deciding who rebuilds it when
upstream moves.

### 3. Wait for task 3.6.0, then S3 -> Garage

The literal thing this task asked about, and it becomes straightforward the
moment `sync.aws.endpoint_url` / `sync.aws.force_path_style` ship:

```
task config sync.encryption_secret     <secret>
task config sync.aws.region            garage
task config sync.aws.bucket            taskwarrior
task config sync.aws.endpoint_url      https://garage.f3s.buetow.org
task config sync.aws.force_path_style  true
task config sync.aws.access_key_id     <key>
task config sync.aws.secret_access_key <secret>
```

Same configuration as option 2, but from a supported package. Blocked on an
upstream release *and* on Fedora packaging it — and Fedora is still on 3.4.2
even in Rawhide, so this is not a short wait. No action now.

### 4. taskchampion-sync-server in k3s

The purpose-built backend, and it works with the **already packaged** 3.4.2:

```
task config sync.encryption_secret <secret>
task config sync.server.url        https://task.f3s.buetow.org
task config sync.server.client_id  <uuid>
```

It fits the shape of services we already run — `f3s/anki-sync-server/` and
`f3s/kobo-sync-server/` are the same pattern (docker image + helm chart +
ingress + PV), and the edge already terminates TLS for `*.f3s.buetow.org`. The
server never sees plaintext: Taskwarrior encrypts client-side with
`sync.encryption_secret`.

Upstream ships prebuilt images, so there is nothing to build:
`ghcr.io/gothenburgbitfactory/taskchampion-sync-server` (SQLite) and
`…-postgres`. Latest release is `taskchampion-sync-server-0.2.1` (2026-06-10).
The SQLite image plus one NFS-backed PV is the obvious fit. New users need no
server-side setup — the server creates a user the first time it sees a new
`client_id`.

### 5. Git sync (3.5.0)

Attractive because we already run Forgejo, but no Fedora release — Rawhide
included — packages 3.5.x, so it is not reachable today either without building
from source.

## Recommendation

The honest framing is that this is a choice between *the backend you asked for*
and *supported software*, and both are now known to work:

* **If LAN-only sync is acceptable, use the stock Fedora package.** `dnf install
  task` plus `AWS_ENDPOINT_URL_S3=http://192.168.1.130:3900` in the shell
  environment is the whole configuration. No build, no package to maintain, no
  unreleased code. This is the cheapest option by a wide margin and it is
  tested.
* **If sync must work away from home**, take option 2 (custom build). Only
  `sync.aws.force_path_style` makes a hostname endpoint work, and that is in no
  release. Package it as an RPM — and note that Fedora's own SRPM already
  carries the spec, the vendored-crates tarball and a `create-vendored-tarball.sh`
  script, so this is a version bump and rebuild rather than a hand-written spec.
* **If "stop losing edits to Syncthing conflicts" is the point**, option 4
  (`taskchampion-sync-server` in k3s) gets there on the **packaged** 3.4.2, with
  no custom build to maintain, using upstream's prebuilt image and the same
  shape as `f3s/anki-sync-server/`. It just is not S3.

Note that option 2 does not need Garage's `root_domain` or any relayd/TLS work —
`force_path_style` is precisely what makes the existing path-style-only Garage
usable as-is. That was the blocker, and the custom build removes it.

Whichever wins, these are prerequisites and deserve their own tasks:

1. **Migrate 2.6.2 -> 3.x.** Verified to work end-to-end (counts identical,
   `ask` unmodified), but it takes ~15 minutes and loses the undo history.
2. **Deal with `task2`/`vit`.** Installing any 3.x `task` removes `task2`, which
   `vit-2.3.4` depends on.
3. **Retire Syncthing for `~/Documents/Taskwarrior`.** Leaving it on would
   file-sync a SQLite database between machines — worse than today.
4. **Leave the macOS work laptop out of it** (see below).

### macOS work laptop

The work laptop **keeps local file storage** and gets no sync backend. In
practice that means its `~/.taskrc` must not set any `sync.*` key. If the taskrc
ever becomes shared/templated, branch on `uname`:

```sh
# only on the Linux side
[ "$(uname)" = "Darwin" ] || task config sync.server.url https://task.f3s.buetow.org
```

Note the consequence: it must also leave the Syncthing share, or it will keep
receiving 2.x `*.data` files that its own Taskwarrior no longer matches.

## Reproducing this

```sh
# what the laptop runs
task --version; rpm -qf /usr/bin/task

# what Fedora offers, and which sync keys it was built with
dnf list --showduplicates task
dnf download task
rpm2cpio task-3.4.2-*.rpm | cpio -idm
strings ./usr/bin/task | grep '^sync\.' | sort -u
zcat usr/share/man/man5/task-sync.5.gz | groff -Tutf8 -man | col -bx

# upstream: when did S3-compatible support land, and in which release?
gh api "repos/GothenburgBitFactory/taskwarrior/commits?path=src/commands/CmdSync.cpp" \
  --jq '.[] | "\(.commit.author.date[0:10]) \(.commit.message | split("\n")[0])"'
gh api "repos/GothenburgBitFactory/taskwarrior/releases" --jq '.[] | "\(.tag_name) \(.published_at[0:10])"'
```

```sh
# custom build, fully isolated (see the Custom build section for build.sh)
podman run --rm -v "$PWD":/work:z -v "$PWD":/out:z fedora:44 bash /work/build.sh
strings ./task-custom | grep -E '^sync\.' | sort -u

# garage admin on f0 -- note ssh is on port 22, not the *.buetow.org default of 2,
# and the garage CLI needs doas because garage.toml is not world-readable
ssh -p 22 paul@f0.lan.buetow.org 'doas garage status; doas garage bucket list'
```

The 3.x migration was rehearsed against a **copy** of the real database in a
scratch directory (`TASKRC` pointing at a throwaway taskrc with its own
`data.location`, and the extracted 3.4.2 binary run directly out of the RPM).
`~/.task` was never touched, and the custom build was compiled and run entirely
inside a container except for the final "does it run on the host" check.
