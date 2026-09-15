# Unattended security upgrades — implementation plan

**Status: implementation plan only — nothing implemented yet.** All shell code in this repo must be **ksh** (house rule), hence the wrapper script uses `#!/bin/ksh`.

Companion doc: [`unattended-upgrades.plan.md`](./unattended-upgrades.plan.md) — design rationale, research summary, and decisions. This document is the build & rollout playbook.

## 1. Deliverables

| # | Artifact | Target on host | Repo location | Via |
|---|---|---|---|---|
| 1 | Wrapper script (`ksh`) | `/usr/local/sbin/unattended-upgrade` (0755 root:wheel) | `frontends/scripts/unattended-upgrade.sh` (plain file, identical on both hosts) | new Rex task |
| 2 | Daemon restart list | `/etc/unattended-upgrade-services` (0644) | inline in Rex task | new Rex task |
| 3 | Staggered cron lines (root) | root crontab | generated in Rex task | new Rex task (rsync/dns-failover idiom) |
| 4 | Log rotation | `/var/log/unattended-upgrade.log` | one line in `etc/newsyslog.conf` | already-deployed file |
| 5 | Root mail routing | `root: paul` | **already done** (`etc/mail/aliases`, deployed with `newaliases` on change) | — |
| 6 | State/log dirs | `/var/run/unattended-upgrade`, `/var/run/unattended-upgrade.lock`, `/var/db/unattended-upgrade` | created by script | — |

No secrets involved.

## 2. Target state per host (daily, decided schedule)

| Job | blowfish (morning) | fishfinger (evening) |
|---|---|---|
| base (`syspatch`) | 06:10 | 22:10 |
| pkgs (`pkg_add -Iu` + restarts) | 06:40 | 22:40 |
| reboot-if-needed | 07:10 | 23:10 |

Jitter (+0–20 min) happens **inside the script**, skipped for interactive runs. Windows sit outside gogios's 08:00–22:00 cron window; blowfish's run is the day's canary for fishfinger's evening run.

Cron lines as they must land in root's crontab (OpenBSD cron; **no `-n` flag** — we want the mail; note other tasks use `-ns`, which would suppress mail):

```
# blowfish
10 6 * * * /usr/local/sbin/unattended-upgrade base
40 6 * * * /usr/local/sbin/unattended-upgrade pkgs
10 7 * * * /usr/local/sbin/unattended-upgrade reboot
# fishfinger
10 22 * * * /usr/local/sbin/unattended-upgrade base
40 22 * * * /usr/local/sbin/unattended-upgrade pkgs
10 23 * * * /usr/local/sbin/unattended-upgrade reboot
```

## 3. The wrapper script — full draft (ksh)

Deploy as `frontends/scripts/unattended-upgrade.sh` → `/usr/local/sbin/unattended-upgrade`.

```ksh
#!/bin/ksh
#
# unattended-upgrade — unattended security-only updates for OpenBSD.
#
#   base    apply base-system errata via syspatch(8)
#   pkgs    update packages via pkg_add(1) -u, restart affected daemons
#   reboot  reboot if a patched kernel is pending
#
# Intended for root's crontab. Everything it prints is mailed to root by
# cron AND appended to /var/log/unattended-upgrade.log (rotated via
# newsyslog(8)). Silence = clean no-op; mail = change or failure.
# Companion docs: frontends/docs/unattended-upgrades*.md

PATH=/usr/bin:/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin
export PATH

LOG=/var/log/unattended-upgrade.log
SERVICES=/etc/unattended-upgrade-services
RUNDIR=/var/run/unattended-upgrade
LOCK=/var/run/unattended-upgrade.lock

mode=${1:-}
case $mode in
base|pkgs|reboot) ;;
*) print -u2 "usage: $0 base|pkgs|reboot"; exit 64 ;;
esac

mkdir -p "$RUNDIR" || exit 1

# Jitter only for cron runs (no tty); manual runs are instant.
[[ -t 0 ]] || sleep $((RANDOM % 1200))

# Whole-job lock; an overlapping cron run just exits quietly.
if ! mkdir "$LOCK" 2>/dev/null; then
    logger -t unattended-upgrade "skipped $mode: another run holds the lock"
    exit 0
fi
trap 'rmdir "$LOCK" 2>/dev/null' EXIT

# Emit to stdout (cron mails it) and append the same lines to the log file.
log() {
    printf '[%s] %s\n' "$(date '+%Y-%m-%dT%H:%M:%S')" "$*" | tee -a "$LOG"
}

restart_services() {
    [[ -f $SERVICES ]] || return 0
    while IFS= read -r svc; do
        [[ -n $svc && $svc != \#* ]] || continue
        if rcctl check "$svc" >/dev/null 2>&1; then
            log "restarting $svc"
            rcctl restart "$svc" >/dev/null 2>&1 \
                || log "WARNING: rcctl restart $svc failed"
        else
            log "not running: $svc — skipped"
        fi
    done <"$SERVICES"
}

case $mode in

base)
    # syspatch rc: 0 = patches applied, 2 = clean no-op,
    # >0 = failure (mirror outage, release EOL, ...).
    before=$(syspatch -l)
    out=$(syspatch 2>&1)
    rc=$?
    case $rc in
    2) exit 0 ;;
    0)
        log "syspatch applied base patches:"
        printf '%s\n' "$out" | tee -a "$LOG"
        if [[ $before != $(syspatch -l) ]]; then
            if grep -qi reboot <<<"$out"; then
                touch "$RUNDIR/needs-reboot"
                log "kernel patched — reboot queued for 'reboot' mode"
            else
                log "non-kernel base patch — restarting base daemons"
                restart_services
            fi
        fi
        ;;
    *)
        log "syspatch FAILED (rc=$rc) — see message below"
        printf '%s\n' "$out" | tee -a "$LOG"
        exit 1
        ;;
    esac
    ;;

pkgs)
    avail=$(df -k / | awk 'NR==2 {print $4}')
    if [[ $avail -lt 1048576 ]]; then
        log "pkgs aborted: only ${avail}KB free on / (need 1GB)"
        exit 1
    fi
    out=$(pkg_add -Iu 2>&1)
    rc=$?
    if [[ $rc -ne 0 ]]; then
        log "pkg_add -u FAILED (rc=$rc) — investigate"
        printf '%s\n' "$out" | tee -a "$LOG"
        exit 1
    fi
    [[ -z $out ]] && exit 0     # nothing to update
    log "pkg_add -u updated:"
    printf '%s\n' "$out" | tee -a "$LOG"
    restart_services
    ;;

reboot)
    if [[ -f $RUNDIR/needs-reboot ]]; then
        log "rebooting to activate the patched kernel"
        rm -f "$RUNDIR/needs-reboot"
        sync
        sleep 2
        reboot
    fi
    ;;
esac

exit 0
```

Behavior notes (verified points to re-check during dry-run):

- `syspatch` exit 2 = "requested but no additional patch was installed" → treated as success. The `before`/`after` diff via `syspatch -l` guards against false "applied" reports.
- The kernel-detection `grep -qi reboot` keys on syspatch's reboot-required message; **verify the exact wording at the first real kernel errata** during the observation phase.
- Reboot flag lifecycle is safe against double reboots: the flag is only set when a *new* kernel patch is applied in that run, and it is removed before `reboot`. `/var/run` leftovers are moot because the flag is only consumed by `reboot` mode.
- `$RANDOM` is supported by ksh; cron has no tty so jitter applies to scheduled runs only.
- OpenBSD `pkg_add -u` on a no-op day prints nothing and exits 0 — confirm; if it ever emits noise, the mail just shows that noise (harmless).

## 4. Daemon restart list

Draft `/etc/unattended-upgrade-services` (deployed by the Rex task):

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
#dserver
#gorum
```

Curation at implementation time (per host): compare with `doas rcctl ls on`, add custom rc.d daemons (`dserver`, `gorum`) if they run there, remove entries that `rcctl check` reports as not enabled. Rationale:

- `relayd`, `httpd`, `nsd`, `smtpd`, `sshd`, `inetd` are **base** daemons — restarted after non-kernel base patches. `sshd` is a frequent errata target, restarting it never kills existing sessions, and inetd re-execs per connection anyway.
- `uptimed` is a **package** daemon (`pkg_scripts` in `rc.conf.local`) — restarted after package updates.
- Restarting a service that wasn't touched is harmless churn and only happens on errata days, never daily.

## 5. Rexfile task (draft, mirrors existing idioms)

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
- Rexfile task `unattended_upgrades` (§5)
- `etc/newsyslog.conf` rotation line (§6)
- Mark both docs as implemented after rollout.

### Phase 2 — deploy + manual dry-run on **blowfish** only

```
rex -H blowfish.buetow.org unattended_upgrades   # single-host deploy (adapt to repo's Rex invocation)
ssh rex@blowfish.buetow.org 'doas /usr/local/sbin/unattended-upgrade base'
ssh rex@blowfish.buetow.org 'doas pkg_add -Inu'   # DRY RUN: what would update?
ssh rex@blowfish.buetow.org 'doas /usr/local/sbin/unattended-upgrade pkgs'
tail -20 /var/log/unattended-upgrade.log
```

Checks:

- First runs may clear a **backlog** (many syspatches at once) — that's why manual runs come *before* cron is enabled; expect a first scheduled reboot on the next kernel-patch day.
- `syspatch -l` shows the applied patches; log file has timestamped entries.
- Lock test: `mkdir /var/run/unattended-upgrade.lock` then run `base` → logs "skipped ... lock held", exit 0; remove lock dir afterwards.
- `reboot` mode test with **no** flag present → instant silent exit. (Test the real reboot path only when a kernel patch is actually pending.)

### Phase 3 — enable cron on blowfish, observe

- The task already installed all three lines; verify with `crontab -l -u root`.
- Observe the next morning cycle: log entries, root mail (only if something happened), all sites green on the gogios page, `uptime` unchanged (unless kernel patch), services up via `rcctl check`.

### Phase 4 — fishfinger

- Deploy + manual dry-run during the day (same as Phase 2), then the evening cycle validates automatically. The gogios window (08:00–22:00) is over by 22:40, so restart blips can't trigger false alerts.

### Phase 5 — review

- After 1–2 weeks (and ideally one real errata day): check logs for noise, tune the restart list, update the docs, done.

## 8. Acceptance criteria

- [ ] Both hosts: `sysctl -n kern.version` is a supported `-release`; `/etc/installurl` sane.
- [ ] Manual `base` run: silent no-op when nothing to do; log + mail when patches applied.
- [ ] Manual `pkgs` run: silent no-op; on updates prints list and restarts listed daemons (`rcctl check` respected).
- [ ] Second concurrent run exits 0 with "skipped" (lock works).
- [ ] Cron fires at 06:10/06:40/07:10 (blowfish) and 22:10/22:40/23:10 (fishfinger) — verify in `/var/cron/log` and the log file.
- [ ] Job output reaches the Proton mailbox via the existing root alias (do **not** use cron's `-n` flag).
- [ ] newsyslog rotates `/var/log/unattended-upgrade.log` (verify entry parses; watch first size-triggered rotation).
- [ ] gogios page stays green through a full cycle; no false CRITICALs from the windows.
- [ ] `needs-reboot` lifecycle: set only on kernel-patch days, consumed exactly once by `reboot` mode.

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
Remove the three crontab lines (redeploy without them or `doas crontab -e`), optionally remove the script + service-list file.

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

1. **Auto-reboot**: plan assumes yes for both hosts (staggered windows). If not wanted, comment out the third cron line per host — the flag then only surfaces in mail… (actually: flag would linger; in that case also make `base` mode mail "reboot needed" loudly instead of just queuing).
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