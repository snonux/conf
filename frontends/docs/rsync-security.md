# Rsync authentication and Joern mirror (task 9n3)

Deployed on 2026-10-23 after task-9n3 approval. Writable album modules now
require authentication, and fishfinger receives Joern's mirror as the locked
`_joernsync` account. Reviewed implementation `276cb16` was published to both
GitHub and Forgejo; ArgoCD reconciled the Shuriken client mount successfully.
Credentials remain outside Git. No inetd restart was needed.

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

## Inspection before migration

Read-only checks on 2026-10-22 found rsync 3.5.1 on blowfish and the managed
root mirror job on both frontends. The existing writable album directories
were `www:www` on both hosts and retain that ownership. Joern's destination on fishfinger was mixed
`root:daemon` / `_foo:daemon`, with 0755 directories; its inspected tree has
no symlinks, and every file and directory has write permission for its owner. Moving the job to
an unprivileged user therefore needs an ownership migration, not merely a
new crontab. The approved migration changed only that destination tree to
`_joernsync:_joernsync`. Its source remains blowfish. Shuriken is the only daemon-protocol album writer found in this
repository. Manual SSH publishing is a separate existing workflow.

## Rollout procedure

This sequence was completed on 23 October. A subsequent rollout or credential
change requires its own reviewed scope and approval.

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
   Complete the task only after these live checks pass.

If a gate fails, stop publishing and the named mirror schedule before repair.
Use only the saved scoped configuration/cron state for an explicitly approved
rollback. Restoring the old root mirror or anonymous write modules reintroduces
the original risks; do not silently downgrade authentication. Normal rsync
deletions remain confined to their approved website destination trees.

## Live acceptance on 23 October

| Check | Observed result |
| --- | --- |
| Credential custody | Created the previously absent, uniquely validated vault entry and Kubernetes Secret; exact byte equality verified against both servers without displaying values. Server files are root:wheel 0600; the live client sees mode 0400 on its read-only mount. |
| Server policy | Both modules on both hosts reject anonymous, wrong-password and wrong-user requests from allowed sources. Authenticated dry runs succeed. Writable modules are hidden. Correct credentials from the unlisted public source receive `Unknown module`, while the same modules work from allowed WG sources. |
| Actual publishing | One bounded Job cloned the reconciled publisher and bypassed only its unchanged-status skip. It completed all four real transfers, retained the generation lock and acknowledged both source status files only after success. |
| Failure acknowledgment | A separate successful diagnostic Job ran the actual publisher with temporary status files and no production data volume. The first mirror completed a dry run; the second rejected a deliberately wrong password with exit 5. Both old markers remained byte-identical. All transfers in this negative test were dry runs. |
| Published content | Strict TLS requests to each album's `status.json` on each frontend returned 200 and matched the corresponding NFS source hash. Both NFS publication markers match their current source status files. |
| Joern mirror | Native execution as `_joernsync` returned zero; root execution was refused. At least three normal cron invocations were observed. Root mirror jobs are absent on both hosts; unrelated root cron entries are preserved. The destination has no wrong-owner entries, unsafe links, special files or set-id bits. Both HTTP backends serve identical content. |
| Convergence and monitoring | A second named-task application succeeded on each host. All 43 Prometheus targets remain up. Normal Gogios cycles retained the existing code-certificate critical and KubeJobFailed warning, with no new failing check. |

Both owned diagnostic Jobs were removed after their results were captured.
Private operational evidence remains on earth under
`~/.local/state/conf-worktrees/9n3-rollout-20261023/ops/`; each frontend retains
private rollback files under `/root/9n3-rsync-20261023`. No credential value
occurred in the twelve checked operational logs.

Joern's public HTTPS hostname verification failed before this rollout, and
HTTP redirects into that failure. Its identical HTTP backend content and
working nonroot mirror are verified separately. Follow-up **qo3** owns the
public TLS repair; no certificate, DNS or relayd change was made for 9n3.

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
gonf. The dated live acceptance above additionally verifies native OpenBSD
account/cron/modes, the actual Kubernetes mount and real application transfers.

Protocol references: [rsyncd.conf](https://rsync.samba.org/ftp/rsync/rsyncd.conf.5),
[rsync](https://rsync.samba.org/ftp/rsync/rsync.1),
[OpenBSD chown](https://man.openbsd.org/OpenBSD-7.9/chown.8) and
[OpenBSD crontab](https://man.openbsd.org/OpenBSD-7.9/crontab.5).
