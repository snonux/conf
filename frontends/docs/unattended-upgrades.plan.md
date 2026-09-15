# Unattended security upgrades — fishfinger & blowfish

**Status: PLAN ONLY — nothing implemented yet.** No crontabs, scripts, or configs were changed on the hosts or in this repo.

## 1. Goals & scope

| In scope | Out of scope |
|---|---|
| Base OS security/reliability errata, unattended | Release upgrades (7.x → 7.y) — stays manual (sysupgrade) |
| Package security errata, unattended | Non-errata package updates / new features |
| fishfinger and blowfish, **staggered** schedules | Other hosts |
| Alerting via mail to root (+alias) | Full config-management rework |

Assumptions: both hosts run an official OpenBSD **-release** (amd64) with `/etc/installurl` set. Verify with `sysctl -n kern.version` before implementing — if either host tracks `-current`, this plan needs adjusting (syspatch does not apply to -current).

## 2. Mechanism — what OpenBSD gives us

### Base system: `syspatch(8)`
- Applies **all missing binary patches** (cumulative; no subset selection). Patches are signed (signify), fetched via `/etc/installurl`, rollback tarballs kept in `/var/syspatch/`.
- Officially cron-friendly: the man page describes `-c` (check only) as "suitable for cron(8)".
- Exit codes: `0` = patches applied, `2` = clean no-op ("no additional patch was installed"), `>0` = error.
- Prints an explicit message when a **reboot is required** (kernel patched) — parseable, don't reboot blindly.
- Only works on the **two most recent releases**; the job will start failing with a clear error once the host's release goes EOL.

### Packages: `pkg_add -u`
- `pkg_add -u` updates every installed package to the newest version for the running release. Per `pkg_add(8)`, when the path contains `%c/packages` (the installurl default) a second directory, **`packages-stable`**, is searched automatically — that is the errata tree.
- Per openbsd.org/stable.html, the `-stable` branch (base and packages) receives only **errata**: bugs "which affect many people" or drastic fixes — in practice security + critical fixes. So on a -release host, `pkg_add -u` **is** the "security only" update mechanism; there is no stricter per-CVE filter, and this should be documented honestly in the final config comment.
- `-I` forces non-interactive mode (cron has no tty anyway; `-I` makes ambiguity an error instead of a hang — failures surface in mail).

## 3. How others do it (research summary)

- **`syspatch -c`** being documented as "suitable for cron(8)" is the project's own hint that cron-driven patching is intended.
- **openbsd.pages.dev/auto-updates/** (cookbook from the Feb 2024 misc@ thread "Automatic OS updates"):
  - On -stable: `0 3 * * * root syspatch && reboot` (reboots only when syspatch actually applied something, exit-2 no-op safe).
  - On -current: nightly `sysupgrade -s`, with `pkg_add -Iu` in `/etc/rc.firsttime` via `/upgrade.site`.
- **misc@ thread positions (Feb 2024)**:
  - Theo de Raadt: most syspatches don't need a reboot; parse syspatch's "reboot required" message instead of always rebooting.
  - Stuart Henderson: for -release, syspatch is the mechanism; beware the **release-day race** — `kern.osrelease` reports the new version before `pub/OpenBSD/<new>/packages` is published, so pkg_add -u can transiently fail right after a release.
  - Fleet-scale opinion (Lyndon Nerenberg): manage the crontab from config management and watch announce@ to gate risky updates.
  - Skeptics' point worth keeping: don't update "at a bad moment blindly" — hence the fixed windows, jitter, lock, and curated restart list below.
- **M:Tier `openup(8)`** — the historic dedicated unattended-update tool (cron-driven, base+packages, signed). The company/service is **defunct** (stable.mtier.org is dead; last snapshots on archive.org). The modern answer everyone uses is the native `syspatch` + `pkg_add -u` via cron.
- **Solene's blog** (dataswamp.org/~solene, "How to trigger services restart after OpenBSD update", 2022-09-25): after package updates, running daemons keep the old shared libraries until restarted — restart the services whose packages changed.

Conclusion: native cron + `syspatch` + `pkg_add -Iu` + a wrapper for lock/jitter/logging/reboot-gating is the mainstream, well-supported pattern. No third-party tool needed.

## 4. Proposed design

### 4.1 Schedule (decided: daily — blowfish in the morning, fishfinger in the evening)

No-op runs are cheap (syspatch exits 2, pkg_add -Iu prints nothing), and with daily checks the worst-case exposure to a fresh errata is < 24 h.

| Step | blowfish (morning) | fishfinger (evening) |
|---|---|---|
| Base (`syspatch`) | 06:10 (+0–20 min jitter) | 22:10 (+0–20 min jitter) |
| Packages (`pkg_add -Iu` + service restarts) | 06:40 (+ jitter) | 22:40 (+ jitter) |
| Reboot-if-needed | 07:10 | 23:10 |

- The hosts run ~12–16 h apart: never simultaneously, failures are independently attributable, and blowfish's morning run acts as the day's canary for fishfinger's evening run.
- Jitter (`sleep $((RANDOM % 1200))`) lives inside the wrapper so the actual fetches decorrelate from the exact cron minute.
- blowfish's window sits after `daily(8)` (~03:01 root crontab) and before the gogios cron window opens (08:00). fishfinger's window starts after gogios's last run (cron runs 08:00–22:00, every 5 min), so its service restarts can't trigger gogios CRITICAL blips; if a restart blips a site briefly, the other host covers it via `dns-failover.ksh`.

### 4.2 Proposed cron entries (for later implementation)

`blowfish` root crontab — mornings (added via a new Rex task, same pattern as the rsync/dns-failover tasks):

```
10 6 * * * /usr/local/sbin/unattended-upgrade base
40 6 * * * /usr/local/sbin/unattended-upgrade pkgs
10 7 * * * /usr/local/sbin/unattended-upgrade reboot
```

`fishfinger` root crontab — evenings:

```
10 22 * * * /usr/local/sbin/unattended-upgrade base
40 22 * * * /usr/local/sbin/unattended-upgrade pkgs
10 23 * * * /usr/local/sbin/unattended-upgrade reboot
```

### 4.3 Wrapper script `/usr/local/sbin/unattended-upgrade` (design, not yet written)

A `ksh` script (`#!/bin/ksh` — house rule: shell scripts are ksh), root-only, one optional argument (`base` | `pkgs` | `reboot`):

1. **Lock**: `mkdir /var/run/unattended-upgrade.lock` (+ trap to remove); exit quietly if held.
2. **`base` mode**:
   - Pre-check: `sysctl -n kern.osrelease`; if `syspatch` reports the release unsupported → mail a loud "EOL — schedule sysupgrade" alert and skip everything else this window.
   - `before=$(syspatch -l)`; run `syspatch`; `after=$(syspatch -l)`.
   - Diff empty → exit silently. Diff non-empty → log + mail the applied patch list; if the kernel was patched (KARL-safe check: `what(1)` build version of `/bsd` vs the booted version in `dmesg.boot` line 1 — the original `/bsd`-newer-than-`dmesg.boot` mtime idea is unusable because KARL re-links `/bsd` at every boot), `touch /var/run/unattended-upgrade/needs-reboot`; non-kernel base patches (e.g. libc) instead restart the curated service list too, since daemons keep old code until restarted.
3. **`pkgs` mode**:
   - Pre-check: ≥ 256 MB free on `/`, `/usr`, `/var` each, else mail + abort (1 GB was unreachable on the frontends' small root).
   - Export `PKG_PATH=installpath:<custom fleet repo>` (root crontabs source no profile), run `pkg_add -Iu`, capture output. Nothing to do → silent. Otherwise log + mail the update list.
   - Restart services from a per-host curated list file (`/etc/unattended-upgrade-services`): for each, `rcctl check <svc> && rcctl restart <svc>`. Candidates from the repo facts: base daemons `relayd httpd nsd smtpd sshd inetd`, pkg-owned `uptimed`, optionally custom `dserver`/`gorum`. Note: gogios is cron-driven (no daemon) and rsync is inetd-spawned — the long-running `inetd` listener is listed instead. Final list from `rcctl ls on`. `acme.sh` (daily.local) needs nothing special.
4. **`reboot` mode**: if `/var/run/unattended-upgrade/needs-reboot` exists **and** the kernel check confirms it → log, `reboot` (rc scripts bring up relayd/httpd/nsd/etc.; brief downtime inside the host's own window, hosts staggered). Stale flags are cleared, never acted on. dns-failover flips zones during a reboot by design (its 3-consecutive-failure threshold tolerates mere restart blips) and flips back afterwards.
5. All output `tee`-d to `/var/log/unattended-upgrade.log` (created 0600 via `umask 077`); cron mail goes to root.
7. **Partner gate** (added 2026-09-15): every mode refuses to run while the partner frontend is not operational (same `https://<partner>/index.txt` Welcome-banner check over IPv4+IPv6 as `dns-failover.ksh`, `timeout`-wrapped, 3 consecutive failures per family) — patching/rebooting the only healthy host would cause total downtime. Skips are logged and mailed.
6. Sets its own `PATH` including `/sbin` (cron's default lacks `/usr/sbin` and `reboot(8)` lives in `/sbin`).

### 4.4 Supporting pieces

- `/etc/newsyslog.conf`: rotate `/var/log/unattended-upgrade.log` (e.g. 1 MB, 5 generations) — file already managed by the Rexfile.
- `/etc/mail/aliases`: already satisfied in this repo — `root: paul` → `paul.buetow@protonmail.com` is deployed by the mail task (with `newaliases` on change). Cron mail from the jobs will reach the Proton mailbox.
- Mirror: keep `/etc/installurl` at the default `https://cdn.openbsd.org/pub/OpenBSD`; confirm clock sync (ntpd) for TLS.

## 5. Risks & mitigations

| Risk | Mitigation |
|---|---|
| Blind update breaks a frontend | Errata-only tree, `-I` non-interactive, curated restart list, gogios checks catch breakage fast; morning/evening split makes blowfish's run the day's canary for fishfinger |
| Kernel patch needs reboot | Reboot is an explicit, version-verified, separate step — never a blind `&& reboot`; only when the on-disk kernel actually differs from the booted one |
| Partner frontend down during our window | Partner gate skips the run (logged + mailed); patches and the reboot flag wait for a healthy-partner window |
| Release-day package-tree lag (Stuart Henderson's caveat) | syspatch-unsupported/EOL check skips and mails; release bumps stay manual (sysupgrade, out of scope) |
| Overlapping/partial runs | Lock dir with 2 h stale-lock recovery (OpenBSD doesn't clear /var/run at boot); separate `base` and `pkgs` steps 30 min apart |
| Broken change needs undo | syspatch rollback tarballs in `/var/syspatch`; `syspatch -r` reverts the latest patch; daily(8) keeps `/var/backups` of `/etc` |
| Disk exhaustion during pkg updates | df pre-check |
| Root mailbox fills silently | mail alias |
| Mirror outage | Non-zero exit + mail; nothing partially applied; next window retries |

## 6. Implementation checklist (for later, via Rex task `unattended_upgrades`)

1. Verify both hosts: `-release` kernel (`sysctl -n kern.version`), `/etc/installurl`, ntpd enabled, disk headroom.
2. Write + deploy wrapper script and per-host service list file (`rcctl ls on` to curate).
3. Add newsyslog rotation entry; add root mail alias; `newaliases`.
4. **Manual dry-run on fishfinger**: `doas /usr/local/sbin/unattended-upgrade base`, then `pkgs` — confirm non-interactive behavior, log, and mail.
5. Enable blowfish crontab (morning run); observe one full cycle (incl. a real errata if timing allows).
6. Enable fishfinger crontab (evening run).
7. Calendar reminder: after each release (Apr/Oct) do the manual sysupgrade window and re-enable the jobs (they mail loudly if the release went EOL first).

## 7. Open questions (decide before implementing)

- **Auto-reboot**: acceptable on both hosts, or only flag "reboot needed" by mail? (default plan: yes, staggered)
- **fishfinger evening timing**: 22:10 is chosen to fall outside gogios's 08:00–22:00 window; an earlier slot (e.g. 20:00) is possible but risks gogios blips during service restarts.
- **Restart list**: confirm the draft (`relayd httpd nsd smtpd sshd inetd uptimed`, optional `dserver`/`gorum`) against `rcctl ls on` per host. gogios (cron-driven) needs no restarts; rsync is inetd-spawned — the `inetd` listener itself is listed.
- Mail routing: **resolved** — `root: paul` → `paul.buetow@protonmail.com` already exists in `etc/mail/aliases`.

## 8. Sources

- https://man.openbsd.org/syspatch — `-c` "suitable for cron(8)", exit codes, rollback tarballs
- https://man.openbsd.org/pkg_add — `-u` update mode, `%m`/`packages-stable` automatic search, `-I` non-interactive
- https://man.openbsd.org/daily — daily/weekly/monthly + `*.local` hooks, root-mail alias recommendation
- https://www.openbsd.org/stable.html and https://www.openbsd.org/faq/faq5.html — -stable = release + errata; only two most recent releases get base fixes
- misc@ "Automatic OS updates" thread, Feb 2024 (marc.info / mail-archive.com) — Theo de Raadt on parsing the reboot-needed message; Stuart Henderson on the release-day packages lag; cookbook at https://openbsd.pages.dev/auto-updates/
- M:Tier `openup(8)` (defunct service), via web.archive.org snapshot of stable.mtier.org
- Solene, "How to trigger services restart after OpenBSD update" (2022-09-25), https://dataswamp.org/~solene/