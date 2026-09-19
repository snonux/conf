# Unattended upgrades — FreeBSD f0–f3

**Status: LIVE on f0–f3 (2026-09-16 via gonf `freebsd_*`).** Rocky-style
hourly stamp gate on FreeBSD hypervisors, driven by root cron.

Companion scripts/recipes:

- [`frontends/scripts/unattended-upgrade-freebsd.sh`](../scripts/unattended-upgrade-freebsd.sh)
- [`gonf/freebsd/unattended.go`](../../gonf/freebsd/unattended.go)

## Why hourly + stamp

f0–f3 can be powered off for prolonged stretches. A fixed daily cron would
miss those days. An **hourly** root cron runs
`/usr/local/sbin/unattended-upgrade-freebsd daily`; a persistent stamp
`/var/lib/unattended-upgrade/last-daily` (`date +%F`) ensures at most one
successful `pkg upgrade` attempt per calendar day. Partner-down or failure
leaves the day unstamped so the next hour retries.

## Behaviour

1. Lock `/var/run/unattended-upgrade.lock` (steal if older than 2 h).
2. Stamp == today → skip pkg; still run reboot check.
3. Else `pkg upgrade -y` (official always; custom `pkgrepo` probe-gated —
   when `https://pkgrepo.f3s.buetow.org/freebsd/<ABI>/latest/` is down or
   shows relayd “Server turned off”, run
   `pkg upgrade -y -r FreeBSD-ports -r FreeBSD-ports-kmods` only — the
   stock 15.x repo names; no repo is named `FreeBSD` anymore).
4. Restart daemons listed in `/etc/unattended-upgrade-services` (never `vm`
   / networking).
5. Write stamp on success.
6. Reboot check every tick (see below).

v1 does **not** automate `freebsd-update` (TTY / prompts). Kernel pending is
still detected so a later *manual* base patch can trigger f0–f2 reboot, and
f3 only logs it.

## Partner ping — reboot only

Pkg upgrades are **not** partner-gated (a sibling off on purpose must not
block patches). Reboots on f0–f2 require **all** sibling FreeBSD hosts
pingable (`ping -c1 -t 3`, 3 tries).

| Host | Hourly minute | Reboot partners | Auto-reboot |
|------|---------------|-----------------|-------------|
| f0 | `:05` | f1 + f2 | yes |
| f1 | `:25` | f0 + f2 | yes |
| f2 | `:45` | f0 + f1 | yes |
| f3 | `:15` | none | **never** (log only) |

Weekday stagger is **offset from r0/r1/r2** so hypervisor and guest do not
share a reboot day: `date +%u % 3` → f0 eq 2, f1 eq 0, f2 eq 1.

## Safe reboot

Per f3s console notes:

1. `vm stopall`; wait until `vm list` shows none `Running`.
2. **`reboot`**, never `shutdown -r` (watchdog → single-user trap).
3. `last-reboot` stamp: at most one unattended reboot per calendar day.
4. If guests still Running after the wait → refuse reboot and log.

## Deploy

```bash
./gonf.sh -list | grep freebsd
./gonf.sh cluster freebsd-hosts freebsd
```

Soak order: **f3 → f1 (CARP backup) → f2 → f0 last** (storage MASTER).

## Validation checklist

- [x] Stamp written after first successful pkg on f3 (`2026-09-16`); upgrades + service restarts observed
- [x] Second run same day is a silent no-op (stamp skip)
- [x] Failed pkg leaves no stamp; next hour retries
      (f3/f2 2026-09-19: 12 unstamped rc=3 failures each, hourly retry,
      stamped only after the repo came back)
- [ ] Custom repo down → WARNING + official-only upgrade still stamps
      (broken 2026-09-16..19: `-r FreeBSD` matched no 15.x repo → rc=3 on
      every window; f3 went ~21 h uncovered on 2026-09-18. Fixed 2026-09-19
      to `-r FreeBSD-ports -r FreeBSD-ports-kmods` (dry-run verified on
      f0–f3); re-check on the next k3s-off window)
- [ ] f3 with kernel pending → log only, no reboot
- [ ] f0–f2 reboot only with partners up + weekday slot + clean `vm stopall`
- [x] newsyslog line present for `/var/log/unattended-upgrade.log`
- [x] Hourly cron with PATH + per-host minutes on all four hosts

Script shebang is `#!/usr/local/bin/ksh93` (FreeBSD `shells/ksh` default
install; the `KSH` port option that would provide `/usr/local/bin/ksh` is off).
