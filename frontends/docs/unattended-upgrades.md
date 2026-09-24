# Unattended upgrades

Automatic package and security updates on every managed host, deployed by
gonf. Scripts live in `frontends/scripts/`, recipes in `gonf/{openbsd,netbsd,rocky,freebsd}/`.
Every script logs to `/var/log/unattended-upgrade.log` (root, 0600, rotated by
newsyslog or logrotate), takes the lock `/var/run/unattended-upgrade.lock`
(stale after 2 h), and restarts the daemons in `/etc/unattended-upgrade-services`
(`gonf/<os>/assets/unattended-upgrade-services`) after updates.

The custom repo `pkgrepo.f3s.buetow.org` sits on k3s, which is often powered
off. relayd then serves an HTTP 200 "Server turned off" page. Every update
script probes for it, skips the fleet packages (dtail, gogios, f3sctl) with a
WARNING and still applies official updates. That is expected, not a failure.

| Hosts | Script | Trigger | gonf |
|---|---|---|---|
| blowfish, fishfinger (OpenBSD) | `/usr/local/sbin/unattended-upgrade` | root cron, 4 jobs, 0-20 min jitter | `frontends_script`, `frontends_services`, `frontends_cron`, `frontends_newsyslog` |
| pi0, pi1 (NetBSD) | `/usr/local/sbin/unattended-upgrade-netbsd` | root cron, 2 jobs, jitter | `pis_netbsd` |
| pi2, pi3, r0-r2 (Rocky) | `/usr/local/sbin/unattended-upgrade-rocky daily` | systemd timer, hourly, once-per-day stamp, no jitter | `rocky` |
| f0-f3 (FreeBSD) | `/usr/local/sbin/unattended-upgrade-freebsd daily` | root cron, hourly, once-per-day stamp | `freebsd` |

Deploy:

```sh
./gonf.sh cluster frontends frontends_script frontends_services frontends_cron frontends_newsyslog
./gonf.sh cluster netbsd-pis pis_netbsd
./gonf.sh cluster rocky-all rocky
./gonf.sh cluster freebsd-hosts freebsd
./gonf.sh -privilege=doas plan -o /tmp/plan frontends_script   # inspect /tmp/plan/plan.jsonl, no apply
```

A second identical run must report 0 changed. Manual runs on a tty skip the
jitter: `ssh -p 2 -tt rex@<host> 'doas -n /usr/local/sbin/unattended-upgrade <mode>'`.

## OpenBSD frontends

| Mode | blowfish | fishfinger | Does |
|---|---|---|---|
| `base` | 06:10 | 22:10 | `syspatch`, re-run after syspatch updates itself |
| `pkgs` | 06:40 | 22:40 | `pkg_add -Iu`, custom repo probe-gated, restarts listed daemons unless only `quirks` changed |
| `audit` | 07:05 | 23:05 | read-only `pkg_add -Iun` against fresh metadata; clean result logged quietly; findings, custom repo down or errors mail root and exit non-zero |
| `reboot` | 07:35 | 23:35 | reboots only when `what /bsd` differs from `sysctl -n kern.version` |

Hours come from `openbsd.UnattendedSchedule` (host data) in `gonf/cluster/cluster.go`.
`base`, `pkgs` and `reboot` run only while the partner frontend passes the
`https://<partner>.buetow.org/index.txt` "Welcome to <partner>" check over
IPv4 and IPv6 (same as `dns-failover.ksh`); `audit` is never gated. Output is
mailed to root by cron: silence means nothing happened. Restart list: relayd
httpd nsd smtpd sshd inetd uptimed node_exporter dserver.

| Log line | Meaning |
|---|---|
| nothing | clean no-op |
| `syspatch applied base patches:` | patched; a kernel patch reboots at the next `reboot` slot |
| `pkg_add -u updated:` | packages updated, restarts follow |
| `package audit clean` | fine, not mailed |
| `package audit found ...` / `package audit FAILED ...` | investigate |
| `WARNING: https://pkgrepo... not operational` (+ `rc=1 with the custom repo skipped`) | k3s asleep, official updates applied |
| `skipped <mode>: partner <host> not operational` | retried next window |
| `pkg_add -u FAILED` / `syspatch FAILED` | investigate |
| `rebooting to activate the patched kernel` | check uptime and kernel afterwards |

Health check per host:

```sh
ssh -p 2 -o BatchMode=yes rex@<host> '
  doas -n sha256 -q /usr/local/sbin/unattended-upgrade     # vs sha256sum frontends/scripts/unattended-upgrade.sh
  doas -n crontab -l -u root | grep -A1 "BEGIN GONF"
  doas -n syspatch -c | wc -l
  doas -n tail -20 /var/log/unattended-upgrade.log'
```

Runbook:

- EOL / "release not supported" from syspatch: time for an attended
  `doas sysupgrade` (new release around April and October). `pkgs` picks up
  the new tree afterwards; one transient failure on release day is normal.
- `pkg_add` dependency conflict: run `doas pkg_add -Iu` interactively.
- Rollback: `syspatch -r` (latest patch, cumulative), tarballs in
  `/var/syspatch`, `/etc` copies in `/var/backups`.
- Daemon kept old code: compare `/etc/unattended-upgrade-services` with
  `rcctl ls on`.
- Disable: drop the cron jobs (gonf) and the script.
- Mail path: cron, root alias, Proton mailbox.

## NetBSD pi0/pi1

`pkgs` (pi0 02:10, pi1 22:10): `pkgin -y upgrade`, then
`PKG_PATH=https://pkgrepo.f3s.buetow.org/netbsd/11.0/packages/aarch64/ pkg_add -u dtail f3sctl`
(probe-gated; bare stems, pkgin never sees the custom repo), restarts
bozohttpd wireguard npf uptimed dserver sshd. `reboot` (02:50 / 22:50):
when `what /netbsd` differs from `sysctl -n kern.version` (don't use
`dmesg.boot` line 1 on NetBSD).

Both modes need the partner Pi serving the static-site marker on
`http://piN.lan.buetow.org/` (names from `/etc/hosts`). No base-system
updates: those are manual `sysupgrade` runs (see
[`f3s/pi-netbsd/README.md`](../../f3s/pi-netbsd/README.md)). No MTA on the Pis:
read the log. The script sets `PATH` to include `/usr/pkg/bin:/usr/pkg/sbin:/sbin`
because root's non-interactive PATH lacks them.

`netbsd-vuln-audit` (pi0 03:40, pi1 23:40; `pis_netbsd_vuln_audit_*`):
`pkg_admin audit` with a fresh pkg-vulnerabilities list, plus the release
against netbsd.org "Supported Releases" and NetBSD-SA advisories. State in
`/var/db/netbsd-vuln-audit`. Result OK / VULNERABLE (steady state, exit 0,
new findings logged as warnings against a baseline) / UNKNOWN (coverage
broken, exit 3). Lines go to the shared log (tag `vuln-audit:`) and syslog.
Re-baseline: delete `findings` in the state dir.

## Rocky pi2/pi3 and r0/r1/r2

systemd `unattended-upgrade-rocky.timer`: `OnBootSec=10min`, hourly
`OnCalendar` at pi2 `:05`, pi3 `:35`, r0 `:05`, r1 `:25`, r2 `:45`,
`Persistent=true`. The oneshot service runs `daily`:

1. Stamp `/var/lib/unattended-upgrade/last-daily` is today: skip to the
   reboot check.
2. Partner gate (ping): pi2 needs pi3 (192.168.1.128), pi3 needs pi2
   (.127), each r-node needs both others. Miss: no stamp, retry next hour.
3. Pis only: wait until the clock was NTP-synced this boot (no RTC); logs
   `WARNING: unattended-upgrade skipped: clock not NTP-synchronised` once.
4. `dnf -y upgrade` (`--disablerepo=f3s-dtail` when the repo is down),
   `needs-restarting -s` restarts. Stamp only on success.
5. Reboot check every tick, at most one reboot a day (`last-reboot`), partners
   up, r-nodes only on their weekday (`date +%u % 3`: r0 Mon/Thu/Sun, r1
   Tue/Fri, r2 Wed/Sat). Reboots when `needs-restarting -r` lists a package
   installed after `btime` from `/proc/stat` (its own boot time is wrong on
   the RTC-less Pis), or on the Pis when the newest `raspberrypi2-kernel4`
   differs from `uname -r` (once per target; a kernel that still isn't
   running after its reboot logs a daily WARNING instead).

Logs: journal and the log file. Test harness:
`frontends/scripts/tests/unattended-upgrade-rocky.ksh`.

`rocky-kernel-audit` (pi2/pi3 only, timer 06:15 / 06:45;
`rocky_kernel_audit`): the SIG AltArch kernel has no Rocky errata, so the
running kernel's upstream version is checked against the OSV Linux feed.
State `/var/lib/rocky-kernel-audit`. OK / VULNERABLE (normal while no fixed
AltArch kernel exists, exit 0, baseline-diffed) / UNKNOWN (exit 3, fails the
unit). Upper bound for kernel-CNA CVEs only. Re-baseline: delete
`affected-cves`; accept a feed shrink: delete `baseline-cve-records`.

## FreeBSD f0-f3

Hourly root cron at f0 `:05`, f1 `:25`, f2 `:45`, f3 `:15`:

1. Stamp `/var/lib/unattended-upgrade/last-daily` is today: reboot check only.
2. `pkg upgrade -y`; when the custom repo is down,
   `pkg upgrade -y -r FreeBSD-ports -r FreeBSD-ports-kmods` (the 15.x repo
   names). Not partner-gated.
3. Restart sshd node_exporter dserver wireguard uptimed (never `vm` or
   networking). Stamp on success; a failure retries next hour.
4. Reboot check: f0-f2 only, with all sibling f-hosts pingable, on their
   weekday (`date +%u % 3`: f0 2, f1 0, f2 1, offset from the r-nodes), at
   most once a day, after `vm stopall` has stopped every guest (else refuse),
   and with `reboot`, never `shutdown -r`. f3 never reboots (inventory
   `allow_reboot=false` and hardcoded).

No `freebsd-update` automation; base patches are manual, but a pending kernel
still triggers the f0-f2 reboot. Roll out in the order f3, f1, f2, f0 (f0 is
the storage CARP master).

Planning and rollout records:
[`docs/archive/frontends/docs/`](../../docs/archive/frontends/docs/).
