# Unattended security upgrades — implementation plan

**Status: IMPLEMENTED on blowfish (2026-09-15, deployed via gonf `frontends_*` tasks — no Rex).** fishfinger stays disabled until the gated fishfinger-enablement task after the blowfish soak. The source of truth for the wrapper is [`frontends/scripts/unattended-upgrade.sh`](../scripts/unattended-upgrade.sh); this doc no longer embeds a copy. All shell code in this repo must be **ksh** (house rule), hence the wrapper script uses `#!/bin/ksh`.

Companion doc: [`unattended-upgrades.plan.md`](./unattended-upgrades.plan.md) — design rationale, research summary, and decisions. This document is the build & rollout playbook.

## 1. Deliverables

| # | Artifact | Target on host | Repo location | Via |
|---|---|---|---|---|
| 1 | Wrapper script (`ksh`) | `/usr/local/sbin/unattended-upgrade` (0755 root:wheel) | `frontends/scripts/unattended-upgrade.sh` (plain file, identical on both hosts) | gonf `frontends_script` (Privileged) |
| 2 | Daemon restart list | `/etc/unattended-upgrade-services` (0644) | inline in the gonf task (`unattended.go` const) | gonf `frontends_services` (Privileged) |
| 3 | Staggered cron lines (root) | root crontab | `Cron` resources in the gonf tasks | gonf `frontends_cron` (Privileged; per-host schedule selected inside the body via the `WhenHostname` recipe — one task, both hosts) |
| 4 | Log rotation | `/var/log/unattended-upgrade.log` | one line in `etc/newsyslog.conf` + `WithLine` on the live file | gonf `frontends_newsyslog` (Privileged) + the Rex-deployed wholesale copy stays in sync |
| 5 | Root mail routing | `root: paul` | **already done** (`etc/mail/aliases`, deployed with `newaliases` on change) | — |
| 6 | State | `/var/run/unattended-upgrade.lock` (2 h stale-lock recovery); NO needs-reboot flag — the reboot decision is derived from the kernel version compare | created by script | — |

No secrets involved.

## 2. Target state per host (daily, decided schedule)

| Job | blowfish (morning) | fishfinger (evening) |
|---|---|---|
| base (`syspatch`) | 06:10 | 22:10 |
| pkgs (`pkg_add -Iu` + restarts) | 06:40 | 22:40 |
| package audit (`pkg_add -Iun`) | 07:05 | 23:05 |
| reboot-if-needed | 07:35 | 23:35 |

Jitter (+0–20 min) happens **inside the script**, skipped for interactive runs. Windows sit outside gogios's 08:00–22:00 cron window; blowfish's run is the day's canary for fishfinger's evening run.

Cron lines as they must land in root's crontab (OpenBSD cron; **no `-n` flag** — we want the mail; note other tasks use `-ns`, which would suppress mail):

```
# blowfish
10 6 * * * /usr/local/sbin/unattended-upgrade base
40 6 * * * /usr/local/sbin/unattended-upgrade pkgs
05 7 * * * /usr/local/sbin/unattended-upgrade audit
35 7 * * * /usr/local/sbin/unattended-upgrade reboot
# fishfinger
10 22 * * * /usr/local/sbin/unattended-upgrade base
40 22 * * * /usr/local/sbin/unattended-upgrade pkgs
05 23 * * * /usr/local/sbin/unattended-upgrade audit
35 23 * * * /usr/local/sbin/unattended-upgrade reboot
```

## 3. The wrapper script

The source of truth is `frontends/scripts/unattended-upgrade.sh` (deployed by the gonf task `frontends_script` to `/usr/local/sbin/unattended-upgrade`, 0755 root:wheel). The design below from the planning phase still describes intent; the **rollout hardening round (2026-09-15, blowfish) changed these details**:

| Planning-phase design | As deployed (verified on blowfish) |
|---|---|
| Disk guard: 1 GB free on `/`, `/usr`, `/var` | **256 MB** — 1 GB is unreachable on the frontends' small `/` (760 MB) and `/var` (956 MB) |
| Reboot detection: `/bsd` newer than `/var/run/dmesg.boot` | **KARL-safe version compare**: OpenBSD re-links `/bsd` at *every* boot, so the mtime check always reported a pending reboot; the script now compares `what(1)`'s build version of `/bsd` against the booted version in `dmesg.boot` line 1. `reboot` mode re-verifies; a wrong version string means NO reboot (fail-safe) |
| `PATH` without `/sbin` | `/sbin` added so `reboot(8)` resolves |
| Plain `pkg_add -Iu` | `PKG_PATH=installpath:<custom fleet repo>` exported in `pkgs` mode (root crontabs source no `/root/.profile`; without it `pkg_add -u` fails on custom packages) |
| No partner gate | **Update and reboot modes** refuse to run while the partner frontend is not operational — same check as `dns-failover.ksh` (`timeout`-wrapped `ftp -4/-6` fetch of `https://<partner>/index.txt` expecting the `Welcome to <partner>` banner, 3 consecutive failures per family). The read-only audit is deliberately ungated so an outage cannot hide package-security status. Skips are logged and mailed; the `needs-reboot` flag is not consumed by a skip |
| — | `umask 077` (log file mode matches the newsyslog 600 declaration before the first rotation) |
| — | Lock is released explicitly before `reboot` (the EXIT trap is not guaranteed to run under reboot(8)) |
| `needs-reboot` flag in `/var/run/unattended-upgrade` | **REMOVED** — the KARL-safe kernel version compare *is* the pending state; `reboot` mode decides purely on it (self-healing, no flag lifecycle, picks up manually applied patches too) |
| `pkg_add -u` always with the custom repo in `PKG_PATH` | **Probe-gated**: `pkgs` checks the custom repo's health first (the relayd front serves an HTTP-200 "Server turned off" page when the k3s backend is down, so the probe requires a non-empty listing without that marker); when it is down the run **skips custom-repo packages, logs a WARNING, and exits 0** — unattended upgrades must not fail because the cluster is down. Custom packages resume automatically in a later window |
| single `syspatch` run | **Re-runs once after the tool self-updates** (001_syspatch exits 2 with errata still pending) — one `base` run clears the whole backlog, no manual follow-up syspatch |

## 3.1 Package advisory audit

OpenBSD does not provide a generic installed-package CVE scanner. The
authoritative supported mechanism is the package tool's signed `quirks`
metadata: [packages(7)](https://man.openbsd.org/packages) documents that it
identifies older packages with security issues that cannot be updated.

The `audit` mode runs after `pkgs` has refreshed `quirks` and before the
planned reboot. The latest package-job jitter starts at 07:00/23:00; the
audit is scheduled at 07:05/23:05, five minutes after that latest possible
start, while the reboot moves to 07:35/23:35. If a previous job still holds
the lock, the audit reports a failed result instead of silently claiming a
clean check. It runs non-mutating `pkg_add -Iun` over both
`installpath` and the custom fleet repository and clears `PKG_CACHE` first so
the dry run cannot populate it. Its normal signed `quirks-… signed on …`
status is retained as clean metadata, not misreported as a finding. A clean
result is appended quietly to the root-only log. Any other output, an
unavailable custom repository, lock contention, or a non-zero package-tool
status is logged, mailed to root, and exits non-zero.
The custom repository is intentionally not skipped in this mode: otherwise
the audit would silently omit installed fleet packages.

This is a supported OpenBSD package-advisory/update audit, not a claim of
complete third-party CVE enumeration. Keep following OpenBSD errata and
ports-changes for advisory context.

## 4. Daemon restart list

`/etc/unattended-upgrade-services` (deployed by the gonf task `frontends_services`):

```
# Daemons to restart after unattended security updates (one per line).
# gogios is cron-driven (no daemon); rsync is inetd-spawned, so the
# inetd listener itself is listed. Restarting sshd never drops sessions.
relayd
httpd
nsd
smtpd
sshd
inetd
uptimed
node_exporter
dserver
#gorum
```

Curated against `doas rcctl ls on` on 2026-09-15: `dserver` (DTail) and `node_exporter` run on **both** frontends and are therefore restarted; `gorum` runs on neither and stays commented.

Curation at implementation time (per host): compare with `doas rcctl ls on`, add custom rc.d daemons (`dserver`, `gorum`) if they run there, remove entries that `rcctl check` reports as not enabled. Rationale:

- `relayd`, `httpd`, `nsd`, `smtpd`, `sshd`, `inetd` are **base** daemons — restarted after non-kernel base patches. `sshd` is a frequent errata target, restarting it never kills existing sessions, and inetd re-execs per connection anyway.
- `uptimed` is a **package** daemon (`pkg_scripts` in `rc.conf.local`) — restarted after package updates.
- Restarting a service that wasn't touched is harmless churn and only happens on errata days, never daily.

## 5. Original Rexfile draft — SUPERSEDED by the gonf implementation

The deployment was implemented in gonf instead (see `gonf/openbsd/unattended.go`, registered in `cmd/gonf/main.go`); the original Rex draft is kept below for history.

```perl
desc 'Unattended security upgrades: syspatch + pkg_add -Iu (docs/unattended-upgrades.implementation.md)';
task 'unattended_upgrades',
  group => 'frontends',
  sub {
    my $short = ( connection->server =~ /^([^.]+)\./ )[0] || connection->server;

    file '/usr/local/sbin/unattended-upgrade',
      source => './scripts/unattended-upgrade.sh',
      owner  => 'root',
      group  => 'wheel',
      mode   => '755';

    file '/etc/unattended-upgrade-services',
      content => "relayd\nhttpd\nnsd\nsmtpd\nsshd\ninetd\nuptimed\n#dserver\n#gorum\n",
      owner   => 'root',
      group   => 'wheel',
      mode    => '644';

    my @cron = $short eq 'blowfish'
      ? ('10 6 * * * /usr/local/sbin/unattended-upgrade base',
         '40 6 * * * /usr/local/sbin/unattended-upgrade pkgs',
         '10 7 * * * /usr/local/sbin/unattended-upgrade reboot')
      : ('10 22 * * * /usr/local/sbin/unattended-upgrade base',
         '40 22 * * * /usr/local/sbin/unattended-upgrade pkgs',
         '10 23 * * * /usr/local/sbin/unattended-upgrade reboot');

    file '/tmp/unattended-upgrade.cron',
      content => join("\n", @cron) . "\n";

    run 'crontab -l -u root 2>/dev/null | grep -vF /usr/local/sbin/unattended-upgrade > /tmp/root.cron.new || true';
    run 'cat /tmp/unattended-upgrade.cron >> /tmp/root.cron.new';
    run 'crontab -u root /tmp/root.cron.new';
    run 'rm /tmp/unattended-upgrade.cron /tmp/root.cron.new';
  };
```

Notes:

- Crontab rebuild is idempotent (same pattern as the `rsync` and dns-failover tasks).
- No mail-alias change needed: the mail task already deploys `root: paul` → `paul.buetow@protonmail.com` and runs `newaliases` on change.
- Log rotation needs **no Rexfile change**: `/etc/newsyslog.conf` is deployed wholesale from the repo copy — just add the line below there.

## 6. newsyslog rotation

Append to `frontends/etc/newsyslog.conf` (column style matched to the existing entries):

```
/var/log/unattended-upgrade.log		root:wheel	600	5	1024	*	Z
```

(600 mode, 5 generations, 1 MB cap, gzip.) Rotation is independent of the cron jobs; the script re-opens the log per run, so no HUP handling is needed.

## 7. Rollout procedure (staged)

### Phase 0 — preflight checks (both hosts)

```
sysctl -n kern.version          # must be an official -release (e.g. "OpenBSD 7.9 (GENERIC.MP)")
cat /etc/installurl             # default https://cdn.openbsd.org/pub/OpenBSD
ntpctl -s                       # clock synced
doas rcctl ls on | sort         # curate the restart list against reality
df -k /
crontab -l -u root              # see what's there; note daily(8) 03:01
```

### Phase 1 — repo commit

- `frontends/scripts/unattended-upgrade.sh` (script from §3)
- Gonf tasks `frontends_script`, `frontends_services`, `frontends_cron`, and
  `frontends_newsyslog` (§5)
- `etc/newsyslog.conf` rotation line (§6)
- Mark both docs as implemented after rollout.

### Phase 2 — deploy + manual dry-run on **blowfish** only

```
cd /home/paul/git/conf
./gonf.sh push -- -p 2 rex@blowfish.buetow.org frontends_script frontends_services frontends_cron frontends_newsyslog
ssh -t rex@blowfish.buetow.org 'doas /usr/local/sbin/unattended-upgrade base'
ssh -t rex@blowfish.buetow.org 'doas pkg_add -Inu'   # DRY RUN: what would update? (no -t => cron-style jitter applies!)
ssh -t rex@blowfish.buetow.org 'doas /usr/local/sbin/unattended-upgrade pkgs'
tail -20 /var/log/unattended-upgrade.log
```

Checks:

- First runs may clear a **backlog** (many syspatches at once) — that's why manual runs come *before* cron is enabled; expect a first scheduled reboot on the next kernel-patch day.
- `syspatch -l` shows the applied patches; log file has timestamped entries.
- **Mail delivery end-to-end**: `echo "unattended-upgrade test" | mail -s "test $(hostname)" root` → confirm it lands in the Proton mailbox (aliases exist, but cron mail only actually arrives if smtpd's outbound relay works).
- **Patch completeness**: run `syspatch -c` before applying, then compare `syspatch -l` — syspatch silently skips patches for missing filesets ("If any sets are missing, patches are skipped accordingly"), so confirm nothing was skipped.
- **DNS HA interaction**: while restarting daemons manually, watch dns-failover — it probes every minute but needs 3 consecutive failures before flipping zones, so seconds-long restarts must not flip anything (`/var/nsd/run/current_master` unchanged). A real reboot *will* flip zones by design and flip back.
- Lock test: `mkdir /var/run/unattended-upgrade.lock` then run `base` → logs "skipped ... lock held", exit 0; remove lock dir afterwards.
- `reboot` mode test with **no** flag present → instant silent exit. (Test the real reboot path only when a kernel patch is actually pending.)

### Phase 3 — enable cron on blowfish, observe

- The task installs all four lines; verify with `crontab -l -u root`.
- Observe the next morning cycle: log entries, root mail (only if something happened), all sites green on the gogios page, `uptime` unchanged (unless kernel patch), services up via `rcctl check`.

### Phase 4 — fishfinger

- Deploy + manual dry-run during the day (same as Phase 2), then the evening cycle validates automatically. The gogios window (08:00–22:00) is over by 22:40, so restart blips can't trigger false alerts.

### Phase 5 — review

- After 1–2 weeks (and ideally one real errata day): check logs for noise, tune the restart list, update the docs, done.

## 8. Acceptance criteria

- [x] blowfish: OpenBSD 7.8 -release, default installurl (fishfinger pending).
- [x] Manual `base` run on blowfish: cleared a 57-patch backlog (syspatch itself applied 001 then exited 2; completed manually with the updated tool); log + mail present.
- [x] Manual `pkgs` run on blowfish: quirks-7.147 updated; quirks-only bump correctly did NOT restart daemons.
- [ ] Second concurrent run exits 0 with "skipped" (lock works) — lock steal verified by code review; concurrency not exercised live yet.
- [ ] Cron fires at 06:10/06:40/07:05/07:35 (blowfish) and 22:10/22:40/23:05/23:35 (fishfinger) — verify in `/var/cron/log` and the log file.
- [ ] Job output reaches the Proton mailbox via the existing root alias (do **not** use cron's `-n` flag).
- [ ] newsyslog rotates `/var/log/unattended-upgrade.log` (verify entry parses; watch first size-triggered rotation).
- [ ] gogios page stays green through a full cycle; no false CRITICALs from the windows.
- [x] Test mail to root arrived at the Proton mailbox 2026-09-15 21:17 (verified via the local Bridge IMAP).
- [ ] No spurious dns-failover zone flips during restart tests; reboot-driven failover flips zones and back automatically.
- [ ] First manual `base` run applied everything `syspatch -c` advertised (no patches silently skipped due to missing filesets).
- [x] `needs-reboot` lifecycle: kernel patch day set the flag, `reboot` mode consumed it exactly once and loaded the patched kernel (#20); stale-flag self-heal verified (removed, no reboot).

## 9. Runbook

**Is the host patched?**
`tail -20 /var/log/unattended-upgrade.log`, `doas syspatch -l | tail`, pending reboot: `ls /var/run/unattended-upgrade/needs-reboot`.

**Run something manually right now** (no jitter on a tty):
`doas /usr/local/sbin/unattended-upgrade base` / `pkgs` / `reboot`.

**Failure mail triage**
- *Mirror/network error* → transient; rerun manually later.
- *"release not supported"/EOL* → see Release transition below.
- *pkg dependency/conflict* → rerun `doas pkg_add -Iu` interactively, read the error; resolve (e.g. flavor pinning), then let cron resume.

**Release transition (twice a year, ~April & October)**
When a new OpenBSD release lands, the old errata trees stop and the jobs start mailing EOL errors. That mail is the trigger for a planned, **attended** release upgrade (out of scope of this automation): `doas sysupgrade` → reboot → next pkgs run picks up the new `packages-stable` tree automatically. Expect one transient `pkg_add` failure if run during the release-day window when the new package tree hasn't fully propagated — the next run self-heals.

**Rollback**
- Base: `syspatch -r` reverts the most recent patch (patches are cumulative — use with care); rollback tarballs live in `/var/syspatch`; `/var/backups` holds daily(8) `/etc` copies.
- Packages: errata packages are official signed builds; regressions are rare — if needed, reinstall the previous version explicitly via `PKG_PATH`.

**Disable the automation**
Remove the four crontab lines (redeploy without them or `doas crontab -e`), optionally remove the script + service-list file.

## 10. Optional hardening (later)

- **Heartbeat**: silence is healthy, but a dead cron is also silent. Add to `/etc/weekly.local` (deployed via the existing `*.local` idiom):
  ```ksh
  f=/var/log/unattended-upgrade.log
  [ -f "$f" ] && [ -z "$(find "$f" -mtime -8)" ] && \
      echo "unattended-upgrade: no log writes in 8+ days — is cron disabled?"
  ```
- Log pull via the fleet's DTail/dserver (already running on the frontends) for central visibility.
- Watch `announce@`/errata feed and skip automation for specific errata (the fleet-scale pattern from the misc@ thread) — only worth it if blind-update risk ever materializes.

## 11. Timeline & ownership

| When | What |
|---|---|
| Session 1 (~1 h) | Repo commit (script + task + newsyslog line); deploy to blowfish; manual dry-runs (Phase 0–2) |
| Next morning | blowfish first live cycle observed (Phase 3) |
| Session 2 (~30 min) | Deploy + manual test fishfinger; first evening cycle observed (Phase 4) |
| +1–2 weeks | Review logs; finalize restart list; close out docs |

## 12. Open decisions

1. **Auto-reboot**: plan assumes yes for both hosts (staggered windows). If not wanted, remove the `reboot` cron line per host; the kernel-version check then needs a separate loud notification path.
2. **Restart list**: confirm per host via `rcctl ls on` (draft: `relayd httpd nsd smtpd uptimed`; optional `dserver`/`gorum`).
3. fishfinger's 22:10 window (outside gogios's 08:00–22:00) — confirm acceptable.

## 13. Alternatives evaluated (external proposal review)

A second review suggested the same three layers, but with a different packages mechanism (a "`pkg security`"-driven, per-CVE flow). Evaluation:

**Adopted:**
- Restart `sshd` and `inetd` after base patches — syspatch doesn't restart daemons whose binaries changed, and sshd is a frequent errata target. Added to the restart list (§4).

**Rejected (tooling does not exist on OpenBSD):**
- "`pkg security`" / "OpenBSD vulnerability database" — no such tool or database. `pkg_add(1)` has no security mode (its `-c` means "delete extra config files when replacing packages"), `man.openbsd.org/pkg_security` and the `portsec@openbsd.org` list both 404, and FAQ 15 documents `pkg_add -u` as the update mechanism. OpenBSD's model is errata trees, not per-CVE tracking.
- The proposed `pkg-sec-update` script is a fragile reimplementation of what `pkg_add -u` already does better: signature-verified (signify), dependency-resolved updates straight from the `packages-stable` tree that `pkg_add` searches automatically. It also uses `fetch(1)` (a FreeBSD command; OpenBSD has `ftp(1)`) and hardcodes `ftp.openbsd.org` instead of `/etc/installurl`.
- `/etc/crontab` is not used on OpenBSD — root crontab is managed via `crontab(1)` (our Rex task does exactly that).
- `make pkg-patch` is not a ports target (`make patch` / `make package` exist).

**Equivalent / already covered:**
- The "semi-attended check + notify" start mode corresponds to our staged rollout: manual dry-runs (`syspatch`, `pkg_add -Inu`) happen before cron is enabled (Phase 2).
- Release upgrades: our plan triggers via the EOL mail and uses attended `sysupgrade`; "clean reinstall" is a heavier fleet pattern, unnecessary for these two hosts.
- The `packages-stable` tree they point at is real (verified: `pub/OpenBSD/7.9/packages-stable/amd64/`, updated Sep 2026, patchlevel bumps like `apache-httpd-2.4.68p0.tgz`) — and it is exactly the tree our `pkg_add -Iu` already pulls from.

## 14. Changelog

- **Review round 1** (self-review before implementation): replaced a `<<<` here-string (OpenBSD ksh doesn't support them — the script would have failed at parse time) with POSIX-safe constructs; made reboot detection wording-independent (`/bsd` newer than `/var/run/dmesg.boot`, syspatch message as fallback); added stale-lock recovery (OpenBSD doesn't clear `/var/run` at boot) and stale-flag self-healing in `reboot` mode; extended the disk guard to `/`, `/usr`, `/var`; suppressed daemon restarts for quirks-only updates; added dry-run checks for end-to-end mail delivery, syspatch fileset completeness, and dns-failover interaction (verified in-repo: probes every minute, flips only after 3 consecutive failures — restart blips are tolerated, reboots fail over by design).

- **Rollout round (2026-09-15, blowfish only, deployed via gonf):**
  - Disk guard 1 GB → **256 MB** per mountpoint — 1 GB is unreachable on the frontends' small `/` and `/var`, so `pkgs` mode would have aborted forever.
  - **KARL-safe reboot detection**: OpenBSD re-links `/bsd` at every boot, so `/bsd` being newer than `/var/run/dmesg.boot` proved *nothing* (it is always true, including seconds after a clean boot — this caused the reboot-detection false positives during the rollout). The script now compares the on-disk kernel build version (`what(1)`) against the booted version (`dmesg.boot` line 1); unknown state means NO reboot (fail-safe). Verified live: stale flag self-heals, no-flag runs are silent no-ops.
  - `/sbin` added to `PATH` so `reboot(8)` resolves (the original PATH made the `reboot` mode fail with `reboot: not found` while still consuming the flag).
  - `pkgs` mode exports `PKG_PATH=installpath:<custom fleet repo>`: root crontabs source no `/root/.profile`, so without it `pkg_add -u` fails on custom packages (dserver/dtail/gogios). Both the official errata tree (with automatic `packages-stable` search) and the fleet repo are consulted in one run.
  - **Partner-health gate** (user requirement): update and reboot modes refuse to run while the partner frontend is not operational — same check as `dns-failover.ksh` (KISS high-availability): `timeout`-wrapped `ftp -4/-6` fetch of `https://<partner>/index.txt` expecting the `Welcome to <partner>` banner, 3 consecutive failures per family, IPv4 AND IPv6. The read-only audit remains ungated so an outage cannot hide package status. Skips are logged and mailed; the flag is not consumed.
  - `umask 077` so the log file is created 0600, matching the newsyslog 600 declaration before the first rotation.
  - Lock released explicitly before `reboot` (the EXIT trap is not guaranteed to run under reboot(8)).
  - Restart list curated against `rcctl ls on`: `node_exporter` and `dserver` added (they run on both frontends), `gorum` stays commented (runs on neither).
- **Fishfinger rollout round (2026-09-15, k22 — gate overridden by paul):** deployed the then-current full stack via gonf (evening cron 22:10/22:40/23:10 verified, blowfish schedule absent), base cleared the 57-patch backlog, pkgs updated quirks (no restarts), pending-kernel detection armed for the 23:10 reboot slot, mail verified end-to-end. This historical rollout predates the later audit job.
  - **All runtime state removed**: the `needs-reboot` flag and its `/var/run/unattended-upgrade` dir are gone — `reboot` mode decides solely on the KARL-safe kernel version compare (`what(1)` vs `dmesg.boot`), which also picks up manually applied kernel patches. `reboot` releases the lock explicitly before rebooting.
  - **syspatch self-update quirk automated**: `base` re-runs syspatch once when the first run only installed the tool update (exit 2 with errata pending) — a single run now clears an entire backlog; success is judged by the `syspatch -l` diff, not the exit code.
  - **k3s-down tolerance (2026-09-16)**: `pkgs` probes `pkgrepo.f3s.buetow.org` before running; when the k3s backend is down (relayd "Server turned off" page), the custom-repo packages are skipped with a logged WARNING, `pkg_add -Iu` runs official-only, and an rc=1 from unresolvable custom stems is downgraded to a warning (exit 0) — official errata still apply and the failure mail noise disappears. Verified live on both hosts with the cluster down.
