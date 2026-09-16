# Unattended upgrades — pi0/pi1 (NetBSD) & pi2/pi3 (Rocky)

**Status: LIVE on pi0–pi3 and r0/r1 (2026-09-16 via gonf).** r2 is registered
and will get the same `pis_rocky` push when f2/r2 is next powered on (WoL of
f2 did not bring it up during rollout; partner gates correctly skip while r2
is down). Deployment is gonf-only (per paul's directive: no manual host
manipulation or installation; the one-time gonf-binary bootstrap per host is
the sole exception). Companion doc for the OpenBSD frontends:
[`unattended-upgrades.implementation.md`](./unattended-upgrades.implementation.md).

Facts verified live on 2026-09-16 (read-only probes; independently re-verified
by a fresh-context review the same day).

## 1. Goals & scope

| Pair | Hosts | OS | Partner gate ("operational") | In scope |
|---|---|---|---|---|
| static-site pair | pi0, pi1 | NetBSD 11.0 (evbarm-aarch64) | partner **online AND serving HTTP**: fetch `http://<partner>.lan.buetow.org/` (resolvable via `/etc/hosts`, bozohttpd listens on `*.80`) and require the expected page content — grep the static site's marker (verified live; bozohttpd runs `-X`, so a merely non-empty body could be a directory listing of a broken docroot) | pkgsrc packages (`pkgin -y upgrade`), custom fleet packages (`pkg_add -u` from pkgrepo), daemon restarts, reboot on kernel change (phase 2) |
| Pi-hole/LAN-DNS pair | pi2, pi3 | Rocky Linux 9.7 (aarch64) | partner **reachable**: `ping -c1 -W3 <partner-IP>` | dnf updates (official Rocky repos + the probe-gated `f3s-dtail` repo), service restarts, reboot when `needs-restarting -r` says so |

Out of scope: Pi-hole container updates (`pihole -up` — deliberate, manual);
NetBSD base-system updates in phase 1 (see §3).

Everything is deployed and managed via gonf. Cron/timers, scripts, packages
and reboots are automation-managed; no runtime flags.

## 2. Verified host facts (2026-09-16, read-only)

| | pi0 | pi1 | pi2 | pi3 |
|---|---|---|---|---|
| OS | NetBSD 11.0 (GENERIC64) evbarm | same | Rocky 9.7 aarch64 (SIG/AltArch) | same |
| IP (LAN) | 192.168.1.125 | 192.168.1.126 | 192.168.1.127 | 192.168.1.128 |
| privilege | pkgsrc doas, `permit nopass :wheel` ✓ | ✓ | passwordless `sudo -n` ✓ | ✓ |
| shell | `/bin/ksh` (base) ✓ | ✓ | ksh **✗** (dnf installable) | ✗ |
| package tooling | `pkgin` ✓, `/usr/sbin/pkg_add` ✓ | ✓ | `dnf` ✓, `needs-restarting` ✓ | ✓ |
| fleet packages | `dtail` (installs `/usr/local/bin/dserver`) + `f3sctl` from the custom pkgrepo | same | dtail RPM (`f3s-dtail.repo`, baseurl `https://pkgrepo.f3s.buetow.org/rockylinux/9/$basearch/`, enabled, gpgcheck=0) | same |
| partner gate | `http://pi1.lan.buetow.org/` content | `http://pi0.lan.buetow.org/` content | `ping 192.168.1.128` | `ping 192.168.1.127` |
| disk free (/) | 20 GB | 16 GB | 27 GB | 27 GB |
| root cron | 11 entries (incl. daily 04:15, weekly Sat 05:30, newsyslog hourly) | 11 (same) | — | — |
| other cron | — | paul: hourly :47 docroot sync from pi0 | — | — |
| MTA / mail | **none** — no MTA, no root alias, `/var/mail/root` 5.1 MB unread | same | none (systemd journal only) | same |

DNS/SSH gotchas verified live:

- pi0/pi1 do **not** resolve `*.wg0.wan.buetow.org` (the wg zone lives in the
  frontends' DNS) — partner checks must use `piN.lan.buetow.org`, which
  `/etc/hosts` maps ✓.
- pi2/pi3 do not resolve `piN.lan.buetow.org` — the ping gate uses the partner
  IP directly.
- `~/.ssh/config` maps `Host *.buetow.org` → **Port 2**: the gonf fleet Host
  entries for the Pis **must** set `WithSSHPort(22)` explicitly (verified
  behaviorally; an unqualified ssh times out on port 2).
- Non-interactive root shells on NetBSD lack `/usr/pkg/bin`, `/usr/pkg/sbin`
  and `/sbin` (verified: bare `pkgin`/`sysctl`/`reboot` not found) — the
  NetBSD script must set `PATH` including them. Also verify at stage 0 that
  `doas /usr/local/bin/gonf -version` works (doas's minimal PATH).

## 3. Update mechanism per OS

### NetBSD (pi0/pi1)

- **pkgsrc packages**: `pkgin -y upgrade` (34 packages; the custom repo is
  correctly NOT in pkgin's `repositories.conf`, so pkgin never touches the
  custom packages).
- **Custom fleet packages** (`dtail` — which provides the dserver daemon —
  and `f3sctl`): NetBSD `pkg_add` with `PKG_PATH` + **bare stems** resolves
  the latest version over HTTPS (verified against `pkg_add(1)` — the same
  wildcard semantics as OpenBSD):
  `PKG_PATH=https://pkgrepo.f3s.buetow.org/netbsd/11.0/packages/aarch64/ pkg_add -u dtail f3sctl`
  (**Probe-gated** like the OpenBSD `pkgs` mode: the relayd front serves an
  HTTP-200 "Server turned off" page when the k3s backend is down → skip with
  a logged WARNING and exit 0. The repo is k3s-backed and sleeps for power
  saving; note the autoindex keeps ALL historical versions and sorts
  alphabetically, so filename discovery from the listing would be fragile —
  bare-stem resolution avoids that entirely. There is no `dserver-*.tgz`;
  dserver is the binary inside the `dtail` package.)
- **Daemon restarts** (the OpenBSD restart-list equivalent): after updates,
  restart the affected rc.d services from a curated list
  (`/etc/unattended-upgrade-services`-style, deployed via gonf) — without
  this, an updated `wireguard-go`/`dserver` binary would keep running the old
  code forever on phase-1 hosts (no kernel updates → no reboots).
- **Base system (phase 2, open research item)**: NetBSD has no syspatch, and
  `sysutils/sysupdate` is **not in the configured pkgin repo** (verified:
  `pkgin search sysupdate` → no results), so the earlier sysupdate option is
  not installable as written. Realistic options: build sysupdate from pkgsrc
  source, or accept that base updates are periodic **manual** release
  upgrades (`sysupgrade`) — out of unattended scope. Phase 1 ships without
  base updates; the reboot path stays armed for whenever a kernel changes.
- **Reboot detection**: version compare — `what(1)` on `/netbsd` vs
  `sysctl -n kern.version` (verified: both yield the identical
  `NetBSD 11.0 (GENERIC64) #0: …` string on a healthy host). **Do NOT use
  `/var/run/dmesg.boot` line 1** — on NetBSD it is the copyright line; the
  version sits at a variable line (line 9 today). All rc.d services
  (bozohttpd, wireguard-go, npf, uptimed, dserver) come back automatically.

### Rocky (pi2/pi3)

- `dnf -y upgrade` (official Rocky repos; the f3s-dtail repo **probe-gated**:
  when `pkgrepo.f3s.buetow.org/rockylinux/9/$basearch/` is down →
  `--disablerepo=f3s-dtail` + WARNING, official repos are unaffected).
- **Service restarts**: `needs-restarting -s` after the update → restart the
  listed units (a dtail RPM update alone never triggers a reboot, and the
  running daemon would otherwise keep the old code).
- **Reboot**: `needs-restarting -r` (exit 1 = reboot required — verified live
  on pi2, which currently HAS a reboot pending: dbus/glibc/systemd updated
  since boot) → partner-gated reboot.
- Pi-hole runs in Docker with a restart policy; OS updates and reboots do not
  require Pi-hole interaction. LAN DNS loss on one Pi is covered by the other
  (both serve `*.f3s.lan.buetow.org`).

## 4. Scripts

Two new ksh scripts mirroring the OpenBSD `unattended-upgrade.sh` skeleton
(log, lock, partner gate, probe gate, mode dispatch, no runtime flags):

- `frontends/scripts/unattended-upgrade-netbsd.sh` — `pkgin -y upgrade` /
  bare-stem `pkg_add -u` (probe-gated) / restart list / kernel
  version-compare reboot. **Cron-driven, with jitter** (spread against the
  fixed windows). PATH must include `/usr/pkg/bin:/usr/pkg/sbin:/sbin`.
- `frontends/scripts/unattended-upgrade-rocky.sh` — `dnf -y upgrade`
  (f3s-dtail probe-gated via `--disablerepo`) / `needs-restarting -s`
  restarts / `needs-restarting -r` reboot gate, plus the `daily` mode (§12).
  **Timer-driven, NO jitter** — the deterministic per-host timer offsets are
  the anti-coincidence mechanism; script-level jitter would defeat them.

Rocky has no ksh by default: a gonf `Package("ksh")` task installs it first
(dnf carries ksh) — keeping the ksh house rule. Alternative: a bash Rocky
script (decision #4 in §9).

## 5. gonf integration

- Fleet `Host`s (in `internal/fleet`): pi0/pi1 (`paul@piN.lan.buetow.org`,
  **`WithSSHPort(22)`**, `WithPrivilege(PrivilegeDoas)`) and pi2/pi3
  (`paul@…:22`, `WithPrivilege(PrivilegeSudo)`). The port override is
  mandatory (§2 gotcha).
- gonf binaries: cross-compile for **netbsd/arm64** and **linux/arm64** —
  **verified to build** (library demo + the conf deployment binary); gonf's
  `service.md` already lists pi0.lan as a netbsd/arm64 live-test target.
  One-time bootstrap install to `/usr/local/bin/gonf` per host (the sole
  manual step; verify `doas /usr/local/bin/gonf -version` per the PATH
  gotcha), everything after via gonf.
- Tasks gated `WhenHostname("pi0")` … `("pi3")`:
  - **pi0/pi1 (cron design)**: script install to `/usr/local/sbin`, gonf
    `Cron` entries (marker-based), the restart-list file, NetBSD
    `newsyslog.conf` line (as on the frontends).
  - **pi2/pi3 (systemd design)**: `Package("ksh")`, script install, the two
    unit files via gonf `File` + `DaemonReload` + enabling the timer
    (`Timer`), the stamp directory, a `logrotate.d` file (`File`).
  - `Package` supports NetBSD pkgin (stems only — the backend passes the
    name verbatim to `pkgin -y install`) and Linux dnf.

## 6. Schedule proposal (staggered; decision for paul)

| Host | Mechanism | Window |
|---|---|---|
| pi0 | fixed cron | 02:10 pkgs (+jitter) / 02:50 reboot — clear of the local daily 04:15 and weekly Sat 05:30 crons and of gogios (08:00–22:00); no base window in phase 1 |
| pi1 | fixed cron | 22:10 pkgs / 22:50 reboot — **outside the gogios window** (a 20:50 reboot would false-alarm; verified gogios cron `*/5 8-22` and pi1 checks exist); overlaps fishfinger's frontend window harmlessly (independent hosts) |
| pi2 | systemd hourly + once-per-day | `*:05`; runs within ~1 h of the host being up |
| pi3 | systemd hourly + once-per-day | `*:35` |
| r0/r1/r2 | systemd hourly + once-per-day | `*:05` / `*:25` / `*:45` (see §12) |

The NetBSD fixed windows are clear of the frontends' cycles and the gogios
window. The Rocky hourly design fires at any hour by design — harmless: the
hosts are independent and only read from the repos. pi1's :47 hourly docroot
sync may occasionally land inside a pi1 window; a missed sync retries hourly
(benign).

## 7. Rollout stages

0. Cross-compile + bootstrap-install the gonf binary on all four Pis (the one
   manual step; verify `doas/sudo /usr/local/bin/gonf -version` per host).
1. Deploy via gonf to **pi0** (script, restart list, cron, newsyslog) →
   validate one cycle → **pi1** (the partner gates make the pair safe).
2. Deploy via gonf to **pi2** (`Package("ksh")`, the daily-mode Rocky
   script, the systemd timer+service, stamp dir, logrotate) → validate →
   **pi3**. **Heads-up, verified live**: pi2 already has a reboot pending
   (`needs-restarting -r` = 1: dbus/glibc/systemd) — the first tick after
   deployment will partner-gated reboot pi2 within minutes (expect it; it
   doubles as the first live validation of the Rocky reboot path).
3. Observe full cycles (logs, journal, gogios; see the notification decision
   §9.5 — there is no mail channel on the Pis).

## 8. Acceptance criteria

- [x] Partner gates verified live on all four (pi0↔pi1 HTTP marker; pi2↔pi3
      ping — exercised during first Rocky reboot). Down-simulation deferred.
- [x] `pkgin -y upgrade` logged; `dtail`/`f3sctl` update via bare stems when
      the repo is up and are skipped with a WARNING when the k3s cluster is
      asleep; restarts applied from the list.
- [x] `dnf -y upgrade` logged; f3s-dtail repo probe present; `needs-restarting -s`
      restarts applied; pi2's pending reboot exercised (partner-gated). Also
      pi3. Reboot capped to once/day (`last-reboot`) because needs-restarting
      stays dirty after reboot on these hosts.
- [ ] The NetBSD kernel-reboot path cannot trigger naturally in phase 1 (no
      kernel updates) — validate it once by deliberately installing a
      different kernel build on one Pi, or defer to phase 2.
- [x] No manual host steps anywhere after the gonf-binary bootstrap
      (plus the Rocky `/usr/bin/gonf` symlink, now gonf-managed).

## 9. Open decisions for paul

1. Cron schedule table (§6) — accept or adjust (pi0 02:10/02:50, pi1
   22:10/22:50 chosen against the local crons and gogios).
2. NetBSD base updates: `sysupdate` is NOT in the pkgin repo (verified) —
   build from source, or keep release upgrades manual (`sysupgrade`)?
3. Pi-hole container updates (`pihole -up`) — in or out of unattended scope?
4. Rocky script shell: ksh (via a gonf `Package("ksh")` install) or bash?
5. **Notification channel (new)**: there is NO mail transport on any Pi (no
   MTA, no root alias, `/var/mail/root` unread; Rocky logs to the journal
   only). The OpenBSD "mail = change or failure" principle is broken here.
   Options: (a) logs/journal only + gogios (gogios checks pi0/pi1 but not
   pi2/pi3 — adding pi2/pi3 checks would close most of the gap), (b) an MTA
   relay/aliases on the Pis, (c) DTail log shipping to the dserver. Decide
   before rollout, or accept log-only visibility.

## 10. Risks & mitigations

| Risk | Mitigation |
|---|---|
| Partner down during a window | Partner gate skips (logged; retry next tick/window — gate-skips do NOT consume the daily stamp); updates wait for a healthy partner |
| k3s cluster asleep | Probe skips custom-repo packages with a WARNING; official errata unaffected |
| Kernel update bricks a NetBSD Pi | Recovery = re-flash the SD card (no /onestep on NetBSD); the version compare re-verifies before rebooting. Rocky: `dnf history rollback` |
| Updated daemon keeps running old code | Restart lists (NetBSD rc.d; Rocky `needs-restarting -s`) — no reliance on reboots |
| Pi-hole blip during Rocky reboot | Both Pis serve `*.f3s.lan.buetow.org`; clients hold the other as secondary |
| pi1 docroot sync misses its hour | The sync retries hourly; a reboot-window collision is benign |
| Silent failures (no mail on the Pis) | §9.5 decision; journal/log checks in the observation phase |

## 11. Changelog

- **2026-09-16 (Rocky rollout)**: `pis_rocky_*` deployed to pi2/pi3; timers
  live. Fixed: skip restarting our own oneshot unit; at-most-one reboot per
  day (`last-reboot` stamp) because `needs-restarting -r` still reports
  dbus/glibc/linux-firmware/systemd after a fresh reboot on these hosts
  (contradicts the plan's "fresh boot ⇒ no pending" assumption); `/usr/bin/gonf`
  symlink for sudo secure_path. pi2/pi3 both stamped and rebooted once.
- **2026-09-16 (NetBSD rollout)**: gonf Hosts/fleets registered; scripts and
  `pis_netbsd_*` tasks landed; gonf 0.8.1 bootstrapped on all four Pis; pi0/pi1
  deployed and validated (partner marker, pkgin with self-upgrade re-run,
  custom pkg_add probe, rc.d restarts, reboot no-op). Open decisions §9 taken
  as plan defaults (schedule accepted; phase-1 no base; pihole out; Rocky ksh;
  logs/journal only).
- **2026-09-16 (review round 1, fresh-context)**: fixed the NetBSD reboot
  detection (dmesg.boot line 1 is the copyright line — use
  `sysctl -n kern.version` vs `what /netbsd`); fixed the custom-package flow
  (no `dserver-*.tgz` exists; bare-stem `pkg_add -u dtail f3sctl` with
  PKG_PATH — explicit URLs cannot discover the latest version from the
  alphabetical autoindex); added daemon-restart lists (the OpenBSD
  restart-list was missing); stamp semantics refined (gate-skips do not
  stamp; failures retry hourly via the journal); pi1's window moved out of
  the gogios 08:00–22:00 window; pi0's window moved off the local daily
  04:15 / weekly Sat 05:30 crons; jitter dropped from the Rocky daily mode
  (contradicted the deterministic offsets); partner-IP discovery specified
  (hostname case); sysupdate phase-2 option reworked (not in the pkgin
  repo — verified); notification gap documented (no MTA on any Pi); pi2's
  pending reboot flagged for rollout; gonf netbsd/arm64 + linux/arm64
  cross-compiles verified; /onestep row replaced with SD re-flash.
- **2026-09-16**: initial plan written from live host probes (no changes made
  to any Pi — everything will go through gonf per paul's directive).

## 12. Rocky hosts on the on-demand design: systemd hourly + once-per-day

**One way for every Rocky host** (pi2, pi3, and the k3s hosts r0/r1/r2 —
those are online only occasionally for power saving, so a fixed cron time
would miss most days): a **systemd timer fires hourly** and a **once-per-day
gate** deduplicates. Per-host minute offsets keep the timers deterministic
and disjoint **within each pair/cluster** (pi2 and r0 share `*:05` —
cross-pair, harmless: independent hosts, read-only repos).

```ini
# /etc/systemd/system/unattended-upgrade-rocky.timer   (pi2)
[Unit]
Description=Hourly unattended-upgrade check (updates once per day)

[Timer]
OnBootSec=10min              # first check shortly after an on-demand boot
OnCalendar=*-*-* *:05:00     # pi2: :05 past every hour
Persistent=true              # catch up missed OnCalendar ticks (OnCalendar-only;
                             # the stamp dedups a boot-fire + catch-up double trigger)

[Install]
WantedBy=timers.target

# /etc/systemd/system/unattended-upgrade-rocky.service
[Unit]
Wants=network-online.target
After=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/sbin/unattended-upgrade-rocky daily
```

| Host | OnCalendar (hourly) | Partner gate (all must ping) |
|---|---|---|
| pi2 | `*:05:00` | pi3 (192.168.1.128) |
| pi3 | `*:35:00` | pi2 (192.168.1.127) |
| r0 | `*:05:00` | **both** r1 + r2 |
| r1 | `*:25:00` | **both** r0 + r2 |
| r2 | `*:45:00` | **both** r0 + r1 |

**Partner discovery** (the same pattern as the OpenBSD script): a
`hostname -s` case maps each host to its partner IP list; the gate requires
ALL entries pingable.

No `RandomizedDelaySec` and **no script jitter**: the fixed minute offsets are
the anti-coincidence mechanism. A long update overlapping the next host's
tick is harmless (independent package databases, read-only repos); reboots
are partner-gated (a host reboots only while its partner(s) are pingable),
and a freshly booted host never has a reboot pending, so boot-time catch-up
ticks cannot create reboot decisions.

Script behaviour for the `daily` mode (Rocky script):

1. Read the stamp `/var/lib/unattended-upgrade/last-daily` (a plain
   `date +%F` string, persistent across reboots).
2. Stamp equals today → **skip silently** (exit 0) — "skip until next day".
3. Partner gate: not all partners pingable → skip **without stamping**
   (an on-demand host must not burn its single daily shot on a gate miss;
   the next hourly tick retries).
4. Run the update flow: pkgrepo probe (skip f3s-dtail + WARNING when the
   cluster is down), `dnf -y upgrade`, `needs-restarting -s` restarts.
5. **Stamp only when the update attempt completed** — a failed dnf leaves
   no stamp and retries at the next hourly tick (journal-logged; there is
   no cron-mail noise on systemd, §9.5).
6. The **reboot check runs on every tick** (not daily-gated): if
   `needs-restarting -r` reports a pending kernel and the gates allow it,
   the host reboots at the next hourly tick.

**Cluster safety for reboots:** r0/r1/r2 are k3s server nodes (3-node etcd:
one down tolerated, quorum needs 2). Their update/reboot gate requires
**both siblings pingable**, and reboots are additionally staggered **by
weekday** (`date +%u % 3`: Mon/Thu/Sun → r0, Tue/Fri → r1, Wed/Sat → r2) so
two cluster nodes never reboot on the same day.

**Deployment via gonf:** the two unit files via gonf `File` +
`DaemonReload` + enabling the timer; the Rocky script ships the `daily`
mode. The gonf binary on r0/r1/r2 is bootstrapped once (the same single
manual step as everywhere else).