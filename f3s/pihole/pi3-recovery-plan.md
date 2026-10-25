# Pi3 session repair and Pi-hole upgrade (1g3)

**Execution update, 2026-10-25:** the owner approved the Pi3-only alternative
and the scoped repair, runtime bootstrap and upgrade have run. The preparation
and gates below retain their original dated context; actual acceptance follows.

Prepared on 2026-10-23 from read-only native inspection. **At preparation time no service repair,
upgrade, bootstrap, backup, reboot or power operation had run.** The proposal
then required owner approval. Task 1g3 originally called for coordinated rack AC
maintenance on or after October 26; that operation remains unperformed and
must not be inferred from approval of this Pi3-only alternative. Task wo3's
separate proposal covers rocky/r0/r2, not Pi3.

## Observed state

| Check | pi2 (192.168.1.127) | pi3 (192.168.1.128) |
|---|---|---|
| Running image | `pihole/pihole:2026.10.0` | `pihole/pihole:2026.04.0` |
| Native Core / Web / FTL | 6.5 / 6.6.1 / 6.8 | 6.4.1 / 6.5 / 6.6 |
| Container | healthy, zero restarts | healthy, zero restarts |
| login1 bus owner | present | absent |
| Bounded login1 session query | succeeds in 33 ms | immediately fails without activation |
| systemd1 bus Ping | 36 ms | 35 ms |

Fresh Pi3 key-based SSH and ordinary sudo each succeed after approximately
120 seconds. Existing multiplexed commands are prompt. The running logind
process predates the current broker process by about four seconds; its missing
bus ownership supports the same immediate session failure diagnosed in wo3.
The initiating command for that broker restart has not been established.

Native Pi3 logind properties: active/running, `KillMode=control-group`, stop
timeout 90 seconds, `RuntimeDirectoryPreserve=yes`. The inspected root admin
process and Docker/Pi-hole control groups are outside logind's control group.
This supports a scoped repair but does not guarantee every session survives.

Both Pis passed five DNS cases over both UDP and TCP: public A lookup,
`doubleclick.net` blocked to `0.0.0.0`, wildcard and apex
`f3s.lan.buetow.org` to `192.168.1.138`, and a reserved `.invalid` name returning
NXDOMAIN. Both admin endpoints return 302; bounded redirect-following GETs
reach `/admin/login` with final HTTP 200, an HTML document and a Pi-hole marker.
Unauthenticated version API requests return 401. Native authenticated `pihole api info/version` works on
Pi3, as previously verified on Pi2. Saved interactive passwords remain
unverified/rejected; this proposal preserves credentials.

Pi3's current gonf is 0.25.0; this checkout uses 0.25.2. Its local state footprint
is about 34 MiB and available filesystem space about 25 GiB. These are dated
samples, to be rechecked before a rollout. `.env` is a regular mode-0600 file;
compose is mode 0644. State is local under `/home/paul/pihole`, not NFS.

The [upstream 2026.10.0 release](https://github.com/pi-hole/docker-pi-hole/releases/tag/2026.10.0)
is the currently published stable release and matches Pi2 and the desired
tracked compose. This proposal changes only documentation: the desired image
and recipe already exist.

## Proposed scope and gates

Only Pi3 is a mutation target. Keep Pi2 answering DNS throughout. No rack power,
host/guest reboot, system-bus/sshd/Docker daemon restart, authentication-policy
change, password reset or placement configuration change is included. A Pi3
Pi-hole container recreation causes a short interruption to that resolver.

1. Recheck both Pis' DNS/UI health, Pi3's unit/bus state, free space, running
   image identity and state paths. Stop if Pi2 is unhealthy. Confirm no package
   updater or competing maintenance is running. Retain a verified private SSH
   administration connection on port 22 with strict host-key checking, and
   verify a second existing-key channel can execute commands. Confirm the
   privileged repair process is outside logind's group. Do not add keys or
   weaken PAM/sudo. An expired control socket must be re-established explicitly.
2. Acquire normal root privilege first. Allow 150 seconds for fresh SSH and
   another 150 seconds for ordinary sudo while the defect persists; allow
   420 seconds overall if both acquisition and the action share one invocation.
   Once root execution begins, bound the single
   `systemctl restart systemd-logind.service` action to 110 seconds, above the
   native 90-second stop timeout. Do not place a 110-second outer timeout around
   the preceding two-minute sudo acquisition. On error or timeout inspect state
   read-only and stop; do not repeat the restart blindly.
3. Within ten seconds of a successful restart, require a new running logind PID,
   login1 ownership, a session query under one second, and a successful systemd1
   Ping (each bus query bounded to four seconds). Require two fresh strict
   public-key SSH `true` probes with multiplexing disabled and ordinary
   `sudo -n true` through a fresh session: each must succeed in under five
   seconds, with a 15-second outer bound per probe. Require the retained admin
   channel, Docker/Pi-hole PID/start time and health, both Pis' DNS/UI checks and
   existing Pi-hole monitoring to match the baseline. Stop before upgrading if
   any required check fails. No broker restart or automatic reboot is recovery.
4. Pull `pihole/pihole:2026.10.0` explicitly on Pi3 while its old container is
   still serving; `compose pull` would otherwise pull its old live pin. Verify
   the target is the expected arm64 image and compare its image ID with Pi2's
   verified release. Retain the old image ID and do not prune images. Recheck
   disk space. Use a unique root-owned mode-0700 backup directory under
   `/var/backups/f3s-1g3-<UTC timestamp>` and preserve the existing gonf binary
   privately before bootstrap if needed.
5. Run the single-task ordinary dry run below. It may bootstrap gonf 0.25.2 and
   stage public recipe assets; these writes are part of the requested approval.
   It must report only the compose/wildcard changes and conditional container
   recreation. Inspect changes before proceeding. `-n` does not guarantee no
   writes. Strict `-preview` never bootstraps and can refuse the old runtime;
   such a refusal is not a failed live deployment. Do not substitute a cluster
   aggregate or install unrelated packages/firewall tasks.

   ```sh
   ./gonf.sh push -n -privilege=sudo -- -p22 -oBatchMode=yes -oStrictHostKeyChecking=yes paul@pi3.lan.buetow.org pihole_compose
   ```

6. Stop only Pi3's `pihole` compose service. With it stopped, create a mode-0600
   archive in that new backup directory containing `docker-compose.yml`, `.env`,
   `etc-pihole` and `etc-dnsmasq.d`, preserving numeric owners and permissions.
   Verify the archive is readable and includes all four paths before applying.
   Keep secret values and authentication hashes inside the host; record only
   comparison booleans. If backup fails, restart the unchanged old compose
   service and stop the rollout.
7. Apply only `pihole_compose` to Pi3, with the same verified SSH options and
   privilege path and without `-n`. The existing recipe installs compose as
   `paul:paul` 0644 and wildcard as root 0644, then runs `docker compose up -d`
   on change. It leaves `.env` and application state unmanaged. Require the
   new container to become healthy within 300 seconds. Do not re-run an
   uncertain apply automatically; first inspect native state.
8. Verify the expected target image and Core 6.5 / Web 6.6.1 / FTL 6.8, native
   authenticated version API, container health/restarts, both Pis' ten DNS cases
   each, and unauthenticated API 401. For each UI require the initial admin
   redirect and a bounded redirect-following GET (five seconds overall) with
   final HTTP 200, an HTML document and a Pi-hole marker; a redirect alone is
   insufficient. Require the `.env` bytes and
   persisted API password hash to match the stopped backup by private boolean
   comparisons. Confirm existing upstream/wildcard settings and ownership;
   do not recursively chown application state. Require Pi2's container identity
   and start time unchanged. Check existing monitoring without adding checks or
   silencing failures. Run the same single task again and require convergence
   with no file changes or container recreation. Repeat fresh SSH/sudo timing.
9. Record actual results and remaining interactive-login limitation. Publish
   only these documentation changes and acceptance results to GitHub and
   Forgejo if approved; never push unrelated prepared changes from the main
   working branch. Owner acceptance of this narrower repair supersedes the
   original rack-power recovery method; do not report an AC cycle as performed.

## Rollback and stop conditions

Approval should include restoring the exact stopped backup and old image if
upgrade acceptance fails. Keep Pi2 healthy, stop only Pi3 Pi-hole, move failed
state into a new root-private quarantine, then restore all four archived paths
with numeric ownership/permissions into the empty original locations. Use the
saved old compose and retained old image, start only Pi3 Pi-hole, and repeat
DNS/UI/native-version and credential-preservation checks. An image downgrade
alone is insufficient after application data migration. Do not remove backups
or quarantined state during the rollout.

If logind repair does not restore prompt access, or root access is lost, stop
before any Pi-hole mutation. There is no meaningful process rollback for the
old disconnected logind; preserve remaining access and request a separate
recovery action. If upgrade or rollback has an uncertain outcome, inspect
read-only state and stop rather than broadening changes. Physical rack AC
maintenance remains separately coordinated with the owner.

## Verification performed and still pending

Read-only inspection and DNS/UI baselines ran on both native Pis. Sanitized
JSON is kept outside Git in
`/home/paul/.local/state/conf-worktrees/1g3-pi3-inspection-20261023/`:
`pi2-inspection.json`, `pi3-inspection.json`, `pi3-detail.json`, and
`dns-baseline.json`, and `ui-baseline.json`. The final UI login-page GETs
passed on both Pis; interactive authentication was not attempted. The original
DNS parser assumed literal tabs; correcting
it to whitespace fields and re-running all twenty DNS cases passed. That was
a local parser defect, not a wildcard DNS defect.

This documentation-only proposal requires no new unit tests or product build.
At preparation time the service restart, bootstrap, stopped backup, upgrade,
rollback and post-change acceptance had not run. Task 1g3 remained open pending
approval and actual acceptance. The executed results below supersede that
preparation status; no physical rack maintenance was performed.

## Executed and accepted, 2026-10-25

The owner approved this Pi3-only alternative and the related dn3 uploader
completion. The shared maintenance lane was released after the r2 firewall
acceptance. Source was an isolated checkout of published `13a526a`; no recipe
or image-pin change was required. The prior two review rounds remain the review
record for this plan; they were not repeated.

Fresh checks confirmed that ordinary updates had not repaired Pi3: SSH took
120.638 seconds, Gonf was 0.25.0, and active logind PID 267926 lacked login1
ownership. Pi2 also showed a 121.346-second session delay and missing login1
owner. Pi2 remained a read-only healthy resolver; its logind was not repaired.
Two controller collection errors (assuming `/var/backups` existed, then a
version-JSON field type) were corrected before mutation. Neither was a host
failure or failed deployment. The existing probes reached terminal errors
before rerunning; no repair was repeated on a timeout assumption.

A retained ordinary-sudo root channel was outside logind's control group, as
was the running Pi-hole container. No package updater was active; about 25 GiB
was free. One bounded logind restart succeeded in 0.866 seconds, replacing
PID 267926 with 990574. Login1 ownership returned; session enumeration took
34 ms and systemd1 Ping 39 ms. Two fresh strict public-key SSH probes took
2.256/0.665 seconds; fresh SSH plus ordinary sudo took 0.703 seconds. The
retained administration channel survived. Pi-hole identity/PID/start time and
Docker/broker PIDs stayed unchanged through the session repair.

The target was pre-pulled while the old resolver served DNS. Its arm64/Linux
image ID exactly matched healthy Pi2:
`sha256:d34134716fc32038b052a8674c7c6879e0ff21f0e6d8af345166fffa2c27c113`.
The reviewed atomic Gonf bootstrap preserved its old executable privately and
verified version 0.25.2, plan schema 31 and strict-preview capability 1. Strict
preview then showed only the compose update and conditional `up -d`, with
wildcard already matching. Using strict preview after the explicit bootstrap
avoided needing the preparation plan's bootstrap-capable ordinary dry run.

Only Pi3 Pi-hole was stopped. The complete compose, `.env`, `etc-pihole` and
`etc-dnsmasq.d` archive was checked before applying:
`/var/backups/f3s-1g3-20261025.Lml32C/pihole-before-2026.10.0.tar.gz`.
The newly needed backup parent and archive directory are root-owned/private;
the archive is 0600. The old image and backup remain available. No rollback
was needed, and rollback execution is not claimed as tested in this run.

The single `pihole_compose` apply made exactly the expected two changes. The
new container became healthy within the 300-second gate, with zero restarts,
Core 6.5, Web 6.6.1 and FTL 6.8. Native authenticated version API succeeded.
Both Pis passed all twenty DNS cases (five cases over UDP/TCP per host), admin
302 followed to final HTML/Pi-hole 200, and unauthenticated version API 401.
Private comparisons proved Pi3 `.env`, persisted API password hash and upstream
settings identical to the stopped backup, and the LAN wildcard correct.
Host-local credentials were not reset. Interactive login with saved passwords
remains unverified, as in the accepted plan. Application state was not manually
chowned; the container itself normalized its state-directory mode during startup.

Pi2 container identity, PID, start time, image and zero restart count remained
identical. The Pi3 task reapply reported zero changes and skipped container
recreation; its container identity/start time remained identical afterward.
Final fresh SSH took 1.553/0.578 seconds and fresh SSH/sudo 0.660 seconds.
Normal monitoring views at 16:10 UTC had no critical, unknown, stale or
suppressed checks, 608 OK and the pre-existing failed-Job warning; all existing
Pi-hole checks were OK. No monitoring was silenced.

Evidence is private in
`/home/paul/.local/state/conf-worktrees/pi3-combined-20261025/live/`.
The current controller built successfully; no product code changed and no new
unit tests were added. No rack AC cycle, reboot, system-bus/sshd/Docker-daemon
restart, authentication-policy change, workload pinning or credential reset
was performed. This accepted narrower recovery supersedes the original rack
power method. The related dn3 uploader's ordinary-cycle acceptance is tracked
separately and is not a Pi-hole upgrade requirement.
