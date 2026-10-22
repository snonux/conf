# Rsync authentication and Joern mirror (task 9n3)

Local preparation only. No credentials, accounts, permissions, cron entries,
daemon configurations or Kubernetes resources were changed in production.
Publication, deployment, credential provisioning, the ownership migration and
live transfer tests require task-9n3 approval after review. Prior 4n3/5n3
approval covers a different rollout. Existing system SSH authentication and
storage configuration remain outside this task.

## Effective policy

`gonf/frontends/assets/rsyncd.conf.tmpl` is rendered on the controller from
the existing WireGuard inventory. Writable album modules are hidden from
module listings, require daemon user `shuriken`, and admit exact r0/r1/r2
and frontend WG IPv4/IPv6 addresses plus loopback. Reverse DNS is disabled;
all other sources are denied. Loopback and peer IPs do not bypass password
authentication. The daemon still drops to `www:www` for album writes.

The separate read-only `joernshtdocs` module admits fishfinger's WG
addresses and loopback. Fishfinger's pull explicitly uses blowfish WG IPv4
`192.168.2.110`. It preserves recursion, links and times with
`-rlt --safe-links --delete`, without sender ownership, group, permissions,
devices or special files. Unsafe symlinks are ignored. New files follow the
receiving account's umask, 022; existing file modes remain local policy.

The mirror runs from `_joernsync`'s crontab, with failures mailed to root.
Its account has a disabled password and nologin shell; cron uses `/bin/sh`.
The script refuses root and unrelated accounts. The managed root job is
removed on both hosts, including blowfish's former no-op job. No HTTPD,
inetd, PF, SSH or dserver recipe is a dependency of `frontends_rsync`.

Gonf maps `paths.FrontendSecret("etc/rsync/shuriken.password")` to the
Password field of foostore entry `Infra/rsync-shuriken`. It refuses missing,
empty or multiline values and writes a sensitive, root-owned 0600
`/etc/rsyncd.secrets` before installing the authenticated module config.
The same password goes into out-of-Git Kubernetes Secret
`services/shuriken-rsync-auth`, key `password`, mounted read-only with mode
0400. The CronJob passes only the filename, never the password, to rsync.
It retains the previous per-site status marker until both mirrors succeed.

Rsync daemon authentication does not encrypt file contents. These connections
use the existing WireGuard mesh. Do not publish these modules on a new
transport as part of this change.

## Current-state inspection

Read-only checks on 2026-10-22 found rsync 3.5.1 on blowfish and the managed
root mirror job on both frontends. The existing writable album directories
are `www:www` on both hosts. Joern's destination on fishfinger is mixed
`root:daemon` / `_foo:daemon`, with 0755 directories; its inspected tree has
no symlinks, and every file and directory has write permission for its owner. Moving the job to
an unprivileged user therefore needs an ownership migration, not merely a
new crontab. Its source remains blowfish; no source ownership changes are
needed. Shuriken is the only daemon-protocol album writer found in this
repository. Manual SSH publishing is a separate existing workflow.

## Approved rollout sequence (not executed)

1. Review the final diff and obtain approval for the named operations below.
   Work outside the scheduled 10:30/18:30 Europe/Sofia publish windows, with
   no active `shuriken-sync` Job. Capture private rollback copies of each
   frontend config and the two named managed cron blocks; do not log their
   payloads or replace unrelated crontab entries during rollback.
2. Provision the foostore entry if absent, with a generated high-entropy
   single-line password; do not silently overwrite an existing entry. Copy
   its exact value to the named Kubernetes Secret outside Git. Use private
   temporary files or in-memory input, suppress payload output, and remove
   temporary copies. Verify names/key presence and permissions only.
3. On fishfinger only, run the by-name operational task
   `frontends_rsync_pause_joern`. It creates `_joernsync` if necessary and
   removes both named mirror schedules, without reading the rsync secret.
   Verify any old root mirror process has exited. Do not kill unrelated
   rsync sessions or move to the ownership step while a mirror is active.
4. Recheck that `/var/www/htdocs/joern` and its parent path are real
   directories. On fishfinger only, perform the approved one-time
   `doas chown -R -P _joernsync:_joernsync /var/www/htdocs/joern` with mirror
   writers stopped. `-P` prevents following symlinks. Do not change the
   source on blowfish, album trees, file contents or permission modes.
   Record ownership metadata beforehand if ownership rollback is required.
5. With the approved credential available, inspect a fresh plan and deploy
   only `frontends_rsync` to one frontend at a time. The account dependency
   and secret/config/script/cron resources are in scope; no frontend
   aggregate, PF/inetd bind change or package-wide upgrade is authorized.
   Rsync under inetd reads the daemon config per connection, so this does
   not require restarting inetd. Fishfinger's new cron runs unprivileged
   after the prepared tree is writable. Verify root has no mirror job.
6. Publish only the reviewed Shuriken runtime diff on an isolated descendant
   of the published remote tip, after verifying both remotes' ancestry and
   excluding unrelated prepared changes. Observe ArgoCD and the rendered
   Secret mount. Existing Jobs do not gain a new volume retroactively.
7. Run approved bounded acceptance: anonymous and wrong-password album
   requests must fail on both frontends from an allowed WG source; correct
   credentials must succeed; an unlisted source must remain denied.
   Confirm module listings omit writable albums. Use dry-run transfers to
   existing albums first, keeping file listings and credentials out of logs.
   Exercise the actual CronJob publish path with a specifically approved
   changed album or harmless owned fixture; unchanged markers alone do not
   prove a transfer. Confirm a failed mirror retains its old marker.
8. Verify Joern's actual pull under `_joernsync`, content availability,
   ownership and normal next cron cycle. Verify no root receiving process,
   unsafe external symlink, imported set-id mode or copied special file.
   Check both album sites and normal Gogios cycles against their baseline.
   Do not call the task done on local tests or readiness alone.

If a gate fails, stop publishing and the named mirror schedule before repair.
Use only the saved scoped configuration/cron state for an explicitly approved
rollback. Restoring the old root mirror or anonymous write modules reintroduces
the original risks; do not silently downgrade authentication. No account/key
deletion or production file deletion is part of this preparation.

## Local verification

```sh
cd gonf
go generate ./...
go build -p 1 ./...
go test -p 1 ./...
go vet ./...
errcheck ./...
cd ..
helm lint f3s/shuriken/helm-chart
shellcheck -s sh frontends/scripts/rsync.sh.tpl
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s f3s/shuriken/tests -v
PYTHONDONTWRITEBYTECODE=1 python3 frontends/scripts/tests/rsync-security.py
```

The native driver records the real task using a temporary in-memory fixture
secret provider, then runs loopback rsync from that rendered config. Only
temporary paths, uid/gid behavior, chroot and listen port are adapted for the
unprivileged local process. It tests hidden modules, anonymous/wrong-user/
wrong-password/source denial, authenticated writes, real rendered CronJob
transfers and failed status acknowledgment, safe-link/special-file/set-id
behavior, the root identity gate and malformed-secret plan refusal. Identity
commands are stubbed only for the local pull-script fixture; that is not a
native OpenBSD privilege-drop or cron proof. No `*_test.go` is added under
gonf. Native OpenBSD account/cron/modes, actual Kubernetes mounts and real
application transfers remain live acceptance gates after approval.

Protocol references: [rsyncd.conf](https://rsync.samba.org/ftp/rsync/rsyncd.conf.5),
[rsync](https://rsync.samba.org/ftp/rsync/rsync.1),
[OpenBSD chown](https://man.openbsd.org/OpenBSD-7.9/chown.8) and
[OpenBSD crontab](https://man.openbsd.org/OpenBSD-7.9/crontab.5).
