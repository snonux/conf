# Unattended upgrades — pi0/pi1 (NetBSD) & pi2/pi3 (Rocky)

**Status: LIVE on pi0–pi3 and r0/r1/r2 (2026-09-16 via gonf).** Deployment is
gonf-only (per paul's directive: no manual host manipulation or installation;
the one-time gonf-binary bootstrap per host is the sole exception). Companion
doc for the OpenBSD frontends:
[`unattended-upgrades.implementation.md`](./unattended-upgrades.implementation.md).

Facts verified live on 2026-09-16 (read-only probes; independently re-verified
by a fresh-context review the same day).

## 1. Goals & scope

| Pair | Hosts | OS | Partner gate ("operational") | In scope |
|---|---|---|---|---|
| static-site pair | pi0, pi1 | NetBSD 11.0 (evbarm-aarch64) | partner **online AND serving HTTP**: fetch `http://<partner>.lan.buetow.org/` (resolvable via `/etc/hosts`, bozohttpd listens on `*.80`) and require the expected page content — grep for **`Hello, it works`** (verified live 2026-09-16; bozohttpd runs `-X`, so a merely non-empty body could be a directory listing of a broken docroot) | pkgsrc packages (`pkgin -y upgrade`), custom fleet packages (`pkg_add -u` from pkgrepo), daemon restarts, reboot on kernel change (phase 2) |
| Pi-hole/LAN-DNS pair | pi2, pi3 | Rocky Linux 9.7 (aarch64) | partner **reachable**: `ping -c1 -W3 <partner-IP>` | dnf updates (official Rocky repos + the probe-gated `f3s-dtail` repo), service restarts, reboot when `needs-restarting -r` says so |

Out of scope: Pi-hole container updates (`pihole -up` — deliberate, manual);
NetBSD base-system updates in phase 1 (see §3). Base-system and package
vulnerability *status* is audited daily on pi0/pi1 (§14), but remediation of
base findings stays manual.

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
  The daily `netbsd-vuln-audit` (§14) reports when the installed release
  needs such a manual upgrade (advisory, newer point release, or an
  unsupported series); it never upgrades anything itself.
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
- **Reboot**: `needs-restarting -r` (exit 1 = reboot required), **re-checked
  against the kernel boot time** → partner-gated reboot. needs-restarting
  compares install times with systemd's `UnitsLoadStartTimestamp`, which on
  the RTC-less Pis is taken before chronyd steps the clock (systemd starts
  at its build epoch, 2026-09-16 00:00:01 for systemd-252-67.el9_8.6), so
  its list is not trusted as is: the script reboots only when a listed
  package's newest `%{INSTALLTIME}` is later than `btime` from `/proc/stat`
  (task p82, see §12). The SIG kernel `raspberrypi2-kernel4` is not in
  needs-restarting's list at all; the script reboots when the most recently
  installed one differs from `uname -r` (once per target kernel). Until
  chronyd has synced the RTC-less clock, the whole run (dnf included) is
  skipped.
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

- Fleet `Host`s (in `cluster`): pi0/pi1 (`paul@piN.lan.buetow.org`,
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
   doubles as the first live validation of the Rocky reboot path). (That
   "pending" turned out to be the stale boot time of task p82, §12.)
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
      pi3. Reboot capped to once/day (`last-reboot`) as a loop backstop; the
      "stays dirty after reboot" cause is fixed by the btime re-check (p82).
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
   Stable markers a future check can key off in
   `/var/log/unattended-upgrade.log` (Rocky, task p82):
   `WARNING: unattended-upgrade skipped: clock not NTP-synchronised`
   (a Pi whose clock has not synced since boot; once per boot and day) and
   `WARNING: already rebooted for raspberrypi2-kernel4` (a kernel that did
   not come up after its reboot; once a day).

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

- **2026-09-22 (task 652)**: added the daily NetBSD vulnerability audit for
  pi0/pi1 (§14): pkgsrc packages via the pkgsrc-security
  `pkg-vulnerabilities` list and `pkg_admin audit` (the list was absent on
  both Pis, so `pkg_admin audit` failed), and the base system via the
  NetBSD supported-release list and security advisories, with UNKNOWN for
  any coverage gap. Not deployed yet (needs approval).

- **2026-09-22 (task p82)**: pi2/pi3 rebooted every day at 00:05/00:35.
  Root cause, verified read-only on both Pis: `needs-restarting -r` listed
  dbus-broker/glibc/linux-firmware/systemd right after each boot because it
  takes the boot time from systemd's `UnitsLoadStartTimestamp` =
  `2026-09-16 00:00:01` (the systemd-252-67.el9_8.6 build epoch the RTC-less
  Pi clock starts at before chronyd syncs; `KernelTimestamp` is 1970), while
  `btime` in `/proc/stat` was the real boot (2026-09-22 00:06:11 on pi2).
  systemd itself was installed 2026-09-17, after that epoch, so every boot
  "needed" a reboot; the `last-reboot` stamp only capped it at once a day.
  Fix in `unattended-upgrade-rocky.sh`: the listing is re-checked against
  `btime` (reboot only if a listed package was installed after it; unknown
  install time or no parsable list still reboots; unreadable btime defers),
  plus a `raspberrypi2-kernel4` vs `uname -r` check because
  needs-restarting never covered the Pi kernel. r0/r1/r2 unaffected (clock
  synced, `needs-restarting -r` = 0).
  Review round 1: on the Pis only (identified by the installed
  `raspberrypi2-kernel4`, i.e. RTC-less), the whole run — dnf included — is
  skipped without stamping until the clock is NTP-synchronised, because a
  dnf run before the clock steps would record install times before the
  real boot and the btime re-check would then never reboot for a genuine
  update; the r-nodes' RTC-backed clocks are not gated, so a stopped chronyd
  there cannot defer a genuine reboot forever. The timer unit keeps
  `OnBootSec=10min` without `time-sync.target` ordering (chrony-wait is not
  enabled on these hosts); the script gate retries hourly instead. The Pi
  kernel pick sorts ties in install time by version (`sort -k1,1n
  -k2,2V`), and a kernel reboot stamps its target in `last-kernel-reboot`:
  a mismatch that survives that reboot is logged as a WARNING once a day
  instead of rebooting daily. Test: 21 cases incl. real r0 cases (both
  siblings pinged, weekday stagger, no Pi kernel query), passing under bash
  and ksh93 on pi2/pi3.
  Review round 2: the Pi clock gate now latches "synchronised once this
  boot" (boot id in `clock-synced-boot`), and an already-stamped day's
  reboot check runs before the gate, so a later NTP blip blocks neither;
  only a Pi that has not synced since boot skips dnf (and the reboot
  check, whose date and btime would be wrong), with a once-per-boot-and-day
  `WARNING: unattended-upgrade skipped: clock not NTP-synchronised` line.
  The stuck-kernel WARNING is emitted before the partner/once-a-day gates.
  All stamps are written via temp + mv. Test: 25 cases.
- **2026-09-16 (SystemdTimer + r2)**: gonf **0.9.0** adds declarative
  `SystemdTimer` (plan schema v7); Rocky `Units` uses it instead of
  hand-maintained `.service`/`.timer` files under `frontends/systemd/`.
  Bootstrapped 0.9.0 on pi2/pi3/r0/r1/r2; `fleet rocky-all pis_rocky` — r2
  first full apply (timer *:45 live); others converged (unit content unchanged).
- **2026-09-16 (r0/r1/r2)**: registered `root@rN.lan.buetow.org` Hosts + `rocky-k3s` /
  `rocky-all` fleets; timer offsets *:05/*:25/*:45; `yum-utils` added to
  Rocky packages task; gonf linux/amd64 bootstrapped and `pis_rocky` deployed
  on r0/r1 (partner gate skipped with r2 down — as designed). r2 completed
  later the same day once f2 came back.
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

```text
# Installed by gonf SystemdTimer("unattended-upgrade-rocky", …) — not
# hand-maintained unit files. Example for pi2 (OnCalendar minute varies
# per host; see table below):

# /etc/systemd/system/unattended-upgrade-rocky.timer
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
Description=Unattended upgrade (Rocky daily mode)
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
and a freshly booted host has no reboot pending once the listing is
re-checked against `btime` (step 6), so boot-time catch-up ticks cannot
create reboot decisions; the `last-reboot` stamp (at most one unattended
reboot per day) remains as the backstop.

Script behaviour for the `daily` mode (Rocky script):

1. Read the stamp `/var/lib/unattended-upgrade/last-daily` (a plain
   `date +%F` string, persistent across reboots).
2. Stamp equals today → skip the daily upgrade ("skip until next day"), but
   still run the reboot check (step 6) before exiting 0.
3. Partner gate: not all partners pingable → skip **without stamping**
   (an on-demand host must not burn its single daily shot on a gate miss;
   the next hourly tick retries).
4. Run the update flow: pkgrepo probe (skip f3s-dtail + WARNING when the
   cluster is down), `dnf -y upgrade`, `needs-restarting -s` restarts.
5. **Stamp only when the update attempt completed** — a failed dnf leaves
   no stamp and retries at the next hourly tick (journal-logged; there is
   no cron-mail noise on systemd, §9.5).
6. The **reboot check runs on every tick** (not daily-gated). With the gates
   open (not yet rebooted today, partners up, r-node weekday), the host
   reboots when a reboot is **genuinely** required:
   - Pi kernel: the most recently installed `raspberrypi2-kernel4` (ties in
     install time broken by version) differs from `uname -r`;
     needs-restarting does not know this package name. This **assumes** the
     Pi firmware boots the image the latest transaction wrote to `/boot`:
     pi2/pi3 have no `/boot/config.txt`, so the firmware boots the default
     `/boot/kernel8.img`, which each `raspberrypi2-kernel4` posttrans
     overwrites with its own image (verified 2026-09-22: `kernel8.img` is
     byte-identical to `kernel-6.1.31-v8.1.el9.altarch.img`). If a
     `config.txt` is added that pins another image (e.g. via
     `config-kernel.inc`'s `kernel` line), or the new kernel fails and the
     firmware falls back, the
     mismatch survives the reboot: the reboot's target is stamped in
     `/var/lib/unattended-upgrade/last-kernel-reboot`, and for the same
     target the script logs a WARNING (once a day,
     `last-kernel-warning`) instead of rebooting again. A newer target
     reboots again.
   - `needs-restarting -r` exits 1 **and** at least one package it lists has
     a newest `rpm %{INSTALLTIME}` later than `btime` in `/proc/stat`.
     needs-restarting's own boot time (systemd `UnitsLoadStartTimestamp`)
     is wrong on the RTC-less Pis — it is the systemd build epoch, not the
     boot — which made pi2/pi3 reboot daily (task p82). `btime` is derived
     by the kernel from the current clock minus the uptime, so it is right
     once the clock is synced.
   - Conservative cases: a listed package whose install time rpm cannot
     report, or an exit 1 without a parsable `  * <pkg>` list, still
     reboots. Deferred (logged, retried next tick): `btime` is unreadable.
     A needs-restarting error (rc other than 0/1) never reboots.
   - **Clock gate (Pis only)**: a host with `raspberrypi2-kernel4`
     installed (RTC-less) trusts its clock once it has been NTP-synchronised
     (`timedatectl show -p NTPSynchronized`) during the current boot; that
     is latched per boot id in `/var/lib/unattended-upgrade/clock-synced-boot`,
     so a later NTP blip changes nothing. Until then the dnf run (step 4) is
     skipped without stamping — otherwise dnf would record install times
     from the pre-sync clock (before the real boot) and the btime re-check
     would never reboot for that update — and so is the reboot check (the
     date and btime are wrong too). The skip logs
     `WARNING: unattended-upgrade skipped: clock not NTP-synchronised …`
     once per boot and day (`last-clock-warning`; later ticks note it in
     the journal only), a stable marker for a future check (§9.5). An
     already-stamped day goes to the reboot check (step 2) before this gate.
     r0/r1/r2 have an RTC-backed clock and are never gated.
   - The stuck-kernel WARNING is emitted before the reboot gates (partner,
     once-a-day, weekday), so an outage of the partner cannot hide it.
   - A stale listing is noted in the journal only (not in the log file, to
     avoid a line per hour).
   Tests: `frontends/scripts/tests/unattended-upgrade-rocky.ksh`.

**Cluster safety for reboots:** r0/r1/r2 are k3s server nodes (3-node etcd:
one down tolerated, quorum needs 2). Their update/reboot gate requires
**both siblings pingable**, and reboots are additionally staggered **by
weekday** (`date +%u % 3`: Mon/Thu/Sun → r0, Tue/Fri → r1, Wed/Sat → r2) so
two cluster nodes never reboot on the same day.

**Deployment via gonf:** the two unit files via gonf `File` +
`DaemonReload` + enabling the timer; the Rocky script ships the `daily`
mode. The gonf binary on r0/r1/r2 is bootstrapped once (the same single
manual step as everywhere else).

## 13. Rocky pi2/pi3: Raspberry Pi kernel CVE audit (task 752)

**Why:** pi2/pi3 boot `raspberrypi2-kernel4` from the SIG AltArch
`altarch-rockyrpi` repo (running `6.1.31-v8.1.el9.altarch`, built 2023-06-10;
verified 2026-09-22 that it is also the newest build in the repo). The Rocky
`kernel` package is not installed, and Rocky publishes no updateinfo for the
AltArch kernel, so an empty `dnf updateinfo --security` proves nothing about
it. The ordinary dnf path of §3/§12 stays as it is and still covers every
other package.

**Advisory source:** the OSV.dev `Linux` ecosystem export,
`https://osv-vulnerabilities.storage.googleapis.com/Linux/all.zip` (≈59 MB,
re-exported several times a day). Its CVE records come from the Linux kernel
CNA (cvelistV5) and carry per-stable-branch `ECOSYSTEM` ranges
(`introduced`/`fixed`/`last_affected` upstream versions). That is the
authoritative upstream source, and it can be evaluated offline. The OSV query
API is not usable for this: a `version` query for `Linux/Kernel` returns no
matches (verified 2026-09-22), so the script evaluates the export itself.

**Mapping:** the upstream base version of the running kernel (`uname -r` up
to the first `-`, i.e. `6.1.31`) is checked against every CVE record. The
running kernel must belong to the `raspberrypi2-kernel4` package
(`rpm -q raspberrypi2-kernel4-$(uname -r)`), otherwise the result is UNKNOWN.
Ranges are evaluated in the OSV way (sorted events; `introduced` turns the
range on, `fixed` or anything after `last_affected` turns it off), with one
branch-aware correction. About 160 records (2026-09-22 feed) list several
stable fixes in one range, e.g. CVE-2023-52462
`[introduced 5.16.0, fixed 6.1.75, fixed 6.6.14]`.
Plain OSV evaluation would treat 6.2–6.6.13 as fixed by 6.1.75. Here a `fixed`
event closes the range only on its own major.minor branch, except the range's
last `fixed`, which also covers every later branch.

**Coverage caveats:** the affected count is an **upper bound** for kernel-CNA
CVEs, and it misses CVEs assigned by other CNAs:

- ECOSYSTEM ranges start at the branch base (`X.Y.0` or `0`), not at the
  commit that introduced the bug. CVE-2024-26581, for example, is
  `5.16.0 → 6.1.78` although the bug entered 6.1 in 6.1.43. The 752 review
  estimated about 430 of the 8479 matches were introduced after 6.1.31.
- Kernel config and architecture (drivers not built for the Pi) and
  downstream Raspberry Pi patches are not modelled.
- CVEs from other CNAs (mostly pre-2024, for example CVE-2024-1086) are not in
  the feed.

So VULNERABLE means "at least one kernel-CNA CVE whose fix is not in this
upstream version", and OK means "no known kernel-CNA CVE". Neither result is
a proof of exploitability or of safety.

**Implementation:** `frontends/scripts/rocky-kernel-audit.sh` is installed as
`/usr/local/sbin/rocky-kernel-audit` and runs as a gonf `SystemdTimer`
`rocky-kernel-audit` once a day (pi2 `06:15`, pi3 `06:45`, `Persistent=true`,
per-host `cluster.ValueKernelAuditOnCalendar`). The gonf tasks are
`rocky_kernel_audit_*` (packages `ksh`/`unzip`/`jq`/`curl`, script, state dir,
units; cluster `rocky-pis`). They have their own `rocky_kernel_audit`
aggregate and also belong to the `rocky` aggregate. Each run:

1. makes a conditional download (`curl -z`, 304 → reuse) into
   `/var/lib/rocky-kernel-audit/osv-linux-all.zip`. A download that fails or
   is corrupt (`unzip -t`) keeps the previous copy. `fetch=` in the status
   record is `updated`, `unchanged`, `failed`, `corrupt`, or `skipped` when
   the kernel check failed before any fetch.
2. streams the records through jq (`unzip -p | jq`, about 90 s on a Pi 3)
   and counts affected / not affected / unassessable CVE records. Withdrawn
   and legacy `GSD-*` records are ignored.
3. compares the affected list with the **baseline** `affected-cves`, i.e. the
   list from the last OK/VULNERABLE run. CVEs not in the baseline go to
   `new-cves`. Baseline CVEs that no longer affect the kernel (a range
   change, or a withdrawn record) go to `removed-cves`. A baseline that is
   unsorted or holds anything other than CVE ids is treated as empty, with a
   warning. Otherwise it could never recover, because UNKNOWN runs never
   replace it.
4. writes `/var/lib/rocky-kernel-audit/status` via a temp file. The record is
   key=value: `checked_at`, `status`, `previous_status`, `reason`,
   running/installed/repo-newest kernel, `fetch`, `feed_newest_record`,
   `cve_records`, `affected`, `new_cves`, `removed`, `unassessable`,
   `baseline_cve_records`, `drop_streak`. `drop_streak` is the pending
   streak from `drop-candidate`, frozen as-is on runs that stop before the
   drop check: stale feed, floor, unknown kernel, or fetch/corrupt
   failures.
5. commits, only after the status write succeeded and never after an
   UNKNOWN result. The dated batch goes into `new-cves.history` first. Only
   then are the baseline `affected-cves` and its record count
   `baseline-cve-records` replaced, and a pending `drop-candidate` is
   cleared. If any step fails, the status record is rewritten as UNKNOWN
   with a reason naming the file, so it matches the failed unit:
   "cannot update the new-CVE history …, baseline left unchanged", "cannot
   update the baseline …" or "cannot update the baseline count …". A history
   failure leaves the baseline untouched. CVEs that appear during an outage,
   or during a run that could not record its result, are therefore still
   reported as new later. A run whose commit failed still logs its
   `NEW:`/`RESOLVED:` lines before the UNKNOWN line, so the history entry
   it wrote belongs to a run that actually alerted.

**Retention:** `new-cves` holds only the latest run's batch and is emptied
by the next quiet run, and `removed-cves` behaves the same way.
`new-cves.history` keeps `<YYYY-MM-DD> <CVE>` lines for 90 days. It is
rebuilt atomically on every committing run: old entries are pruned, and
there is one line per CVE carrying the date of the first run that alerted
it, so a re-report after a failed commit cannot duplicate or re-date it.
The first run adds the whole initial list. A batch therefore stays visible
after it has left `new-cves` and the journal. A CVE that is resolved and
then affects the kernel again within 90 days keeps its first date.

**Truncation guards:** the feed must hold at least 10000 CVE records, an
absolute floor for a broken first download (about 15.8k in 2026-09). After
that, a drop of more than 20% against `baseline-cve-records` also counts as
truncated. The count only grows in normal operation, because the kernel CNA
rejects few records. The drop check runs only on fresh, above-floor feeds.
A legitimate shrink recovers by itself:

- Each drop run records `<count> <streak>` in `drop-candidate` via a temp
  file, and the status record carries `drop_streak=`. A malformed
  `drop-candidate` is discarded with a warning and the streak restarts.
- The streak continues while the count stays within 95% of the previous
  drop run.
- On the 3rd consecutive drop run (about 3 days) the lower count is
  accepted, with a warning `accepting N CVE records (was M) after 3
  consecutive runs`, and becomes the new baseline count. Until then the run
  reports UNKNOWN.
- A normal run in between clears the streak.

**Operator overrides** (as root in `/var/lib/rocky-kernel-audit`):

- Delete `baseline-cve-records` to accept a lower count at once. The drop
  guard is off until the next successful run records a new count.
- Delete `affected-cves` to re-baseline. The next run reports every
  affected CVE as new once.

| Status | Exit | When |
|---|---|---|
| OK | 0 | every CVE record assessable, none affects the running version |
| VULNERABLE | 0 | at least one range covers the running version (upper bound, see above) |
| UNKNOWN | 3 | kernel not a `raspberrypi2-kernel4` build / unparsable version; feed never fetched; corrupt archive or jq failure; < 10000 CVE records, or > 20% fewer than the last assessment until the lower count held for 3 consecutive runs (truncated); newest record older than 72 h (stale feed or stale cache after download failures); unassessable records with no affected match; state directory, new-CVE list, status record, history or baseline cannot be written (a failed commit rewrites the record as UNKNOWN, naming the file) |

**Alerting:** there is no MTA on the Pis (§9.5), so the journal, the unit
state and `/var/log/unattended-upgrade.log` (lines tagged `kernel-audit:`)
are the alert surface. Journal priorities come from the sd-daemon `<N>`
stdout prefix. VULNERABLE is the steady state while no fixed AltArch kernel
exists, so it must not look like broken coverage:

| Signal | Meaning | Where |
|---|---|---|
| unit **failed**, err (`<3>`) | UNKNOWN: coverage is broken and needs fixing | `systemctl --failed`, `journalctl -p err -u rocky-kernel-audit` |
| warning (`<4>`) `NEW: n CVE(s) newly affect the kernel: …` | CVEs not in the baseline (the first run reports the whole list) | `journalctl -p warning -u rocky-kernel-audit`, `new-cves`, `new-cves.history`, `new_cves=` |
| warning (`<4>`) `status changed: A -> B` | any status transition (e.g. UNKNOWN → VULNERABLE, VULNERABLE → OK after a kernel fix) | same |
| warning (`<4>`) `WARNING: feed download failed` / `not a valid zip` | fetch problem; still assessed from a fresh cache | same |
| warning (`<4>`) `WARNING: baseline … unsorted or malformed` | baseline reset: every affected CVE is reported as new once | same |
| notice (`<5>`) `RESOLVED: n CVE(s) no longer affect the kernel …` | baseline CVEs no longer matched (range change or withdrawn record) | `journalctl -u rocky-kernel-audit`, `removed-cves`, `removed=` |
| notice (`<5>`) / info (`<6>`) | unchanged VULNERABLE / OK summary | `journalctl -u rocky-kernel-audit` |

Nothing reports OK without a fresh, complete, fully assessed feed.

**Known gap: no off-host alerting.** Every signal above stays on the Pi.
Nothing forwards it: there is no MTA and no gogios check of pi2/pi3 unit
state or journal priorities. DTail can read the log on demand, but it alerts
on nothing. A timer that stops firing (unit disabled or masked, timer lost,
host clock broken) goes undetected as well, because `checked_at` in the
status record has no consumer yet. Closing this means taking the §9.5
notification decision, e.g. option (a): a gogios check that reads
`status`/`checked_at` from both Pis and alerts on UNKNOWN, on new CVEs, or on
a `checked_at` older than about 36 h.

**First result (2026-09-22, run as paul under ksh93 on pi2 against the live
feed):** `VULNERABLE: 8479 of 15793 kernel CVEs affect upstream 6.1.31`, with
repo newest = installed = running `6.1.31-v8.1.el9.altarch`, so no fixed
AltArch kernel is available. The branch-aware rule leaves the 6.1.31 count
unchanged. Remediation (a newer SIG AltArch or Raspberry Pi kernel, or
another OS image) is a separate decision. This audit only makes the exposure
visible and auditable.

**Tests:** `frontends/scripts/tests/rocky-kernel-audit.ksh` uses synthetic
feeds with fake `uname`/`rpm`/`dnf`/`curl`; `unzip`, `zip` and `jq` are real.
It ran green under bash on the workstation and under ksh93 on pi2, with
Python `zip`/`unzip` shims via `TEST_EXTRA_PATH`. It covers:

- range evaluation: OK; VULNERABLE; the `last_affected` boundary; running ==
  `fixed` (not affected); several ranges where only a later one matches;
  `-rc` versions; explicit `versions` lists (hit, and a miss that still
  counts as assessed); an unparsable event (→ unassessable); multi-fix
  ranges on 6.1.31/6.4.5/6.6.13 (affected) and 6.1.80/6.6.14/6.8.1 (not
  affected); withdrawn and GSD records ignored.
- baseline alerting: first-run NEW warning; an unchanged rerun raises no
  warning; a new CVE is reported alone; a stale UNKNOWN run leaves the
  baseline untouched and coverage restored reports nothing new; status-change
  warnings; err priority for UNKNOWN; the conditional GET (`-z` only with a
  cached copy); the dated history (no duplicates, 90-day pruning, survives
  a quiet run that empties `new-cves`); an unsorted and a malformed
  baseline (warning, treated as empty, then recovered); a failed baseline
  commit (consistent UNKNOWN record; the next run reports the CVEs again);
  a failed history write (its own reason, baseline untouched, the CVEs
  re-reported and recorded once); a failed baseline commit after the
  history was written (that run still alerts NEW; the later success leaves
  exactly one history line, keeping the first alerting date); resolved
  CVEs (`removed=`, `removed-cves`, RESOLVED notice, no warning).
- coverage failures: unassessable records; a stale feed; an unreachable
  source with no cache, a fresh cache and a stale cache; 304; a corrupt
  download that keeps the cache; a truncated feed (floor); a > 20% record
  drop against the baseline count: a one-off drop (the streak restarts at 1
  after a normal run), a persistent drop accepted on the 3rd run (and the
  new count adopted), the operator override, the frozen `drop_streak`
  on stale and unknown-kernel runs, and a malformed `drop-candidate`
  (warning, streak restarts, temp-file rewrite); an unknown kernel
  (`fetch=skipped`); a held lock; a failed status write that keeps the old
  record and does not consume new CVEs; an uncreatable state directory.

**Deployment:** not yet deployed. It needs explicit approval (task 752
acceptance). When approved, run it from a tree that builds against the
committed gonf module version: `./gonf.sh cluster rocky-pis rocky_kernel_audit`.
Then verify on each Pi with `sudo systemctl start rocky-kernel-audit.service;
sudo cat /var/lib/rocky-kernel-audit/status`. The expected result is
VULNERABLE, with the unit succeeding and the first run's NEW warning in the
journal.

## 14. NetBSD pi0/pi1: package and base-system vulnerability audit (task 652)

**Why:** `pkgin -y upgrade` finding nothing to do does not mean the installed
packages are free of known vulnerabilities. The pkgsrc tool for that,
`pkg_admin audit`, failed on both Pis because
`/usr/pkg/pkgdb/pkg-vulnerabilities` had never been fetched (the stock
`/etc/daily` leaves `fetch_pkg_vulnerabilities` unset). The base system and
kernel are deliberately not updated unattended (§3), so they had no auditable
security signal at all.

**Sources** (all HTTPS, verified reachable from pi0/pi1 on 2026-09-22):

| Component | Source | Integrity / freshness |
|---|---|---|
| pkgs | `https://cdn.NetBSD.org/pub/NetBSD/packages/vulns/pkg-vulnerabilities.gz`, the pkgsrc-security list (≈30k lines, revision 1.794 of 2026-09-16) | `pkg_admin check-pkg-vulnerabilities` checks the format and the embedded SHA512 hash, which also catches truncation. The list's own `$NetBSD` date must be at most 30 days old. A server copy with a lower revision is never installed. |
| base | "Supported Releases" section of `https://www.netbsd.org/releases/` | must contain the section and end with `</html>` (only whitespace, CR included, after it) |
| base | advisory index `https://cdn.NetBSD.org/pub/NetBSD/security/advisories/` plus each in-scope `NetBSD-SA*.txt.asc` | index: ends with `</html>` (only whitespace after it; the real index has CRLF lines and a trailing empty CRLF line), at least 250 advisories (289 in 2026-09), every cached in-scope advisory still listed |

The embedded OpenPGP signature of `pkg-vulnerabilities` is not verified. That
needs the pkgsrc-security key in a netpgp keyring (`GPG_KEYRING_PKGVULN`),
and `pkg_admin -s` fails without one (checked on pi0). HTTPS from the NetBSD
CDN plus the hash check is the trust model. If the key is ever shipped, the
audit can add `-s`.

**Package mapping:** the verified list is installed where pkg_admin looks for
it (`pkg_admin config-var PKGVULNDIR` = `/usr/pkg/pkgdb`, 0644), so
interactive `pkg_admin audit` and the `/etc/security` check in `/etc/daily`
work again as well. `pkg_admin audit` covers every installed package,
including the custom fleet packages (`dtail`, `f3sctl`), which simply have no
entries. Each output line `Package <name>-<version> has a <type>
vulnerability, see <url>` becomes the finding `pkg <pkgbase> <url>`. With
`CHECK_END_OF_LIFE=yes` (the NetBSD 11 default, checked on pi0) pkg_admin
also prints `Package <name>-<version> has reached end-of-life (eol), see
<url>/eol-packages`, which becomes `pkg <pkgbase> eol`: an end-of-life
package gets no more fixes. The key has no version, so an update that is
still vulnerable does not re-alert, and a fixed advisory is reported as
resolved. Any other output line makes pkgs UNKNOWN.

**Base mapping:** the installed formal release (`uname -r` = `11.0`) and the
kernel build date (`uname -v`, 2026-07-30) are the inputs. A kernel that is
not a formal `X.Y` release (for example `11.0_STABLE`) is UNKNOWN. Findings:

- `base eol NetBSD-11.x`: the series is not in the supported list, so it gets
  no security fixes.
- `base release NetBSD-11.1`: the supported list names a newer release of the
  series. Branch fixes, security fixes included, reach a formal release only
  through the next point release.
- `base advisory NetBSD-SA<id>`: an advisory from the kernel build year minus
  2 or later concerns the release. The first matching rule decides:
  1. a `Version:` line for the release itself (`NetBSD 11.0:`): affected
     (also partially, or "affected prior to", since a release is not
     patched in place) or not;
  2. otherwise the `Fixed:` line `NetBSD-11 branch:`: a fix date after the
     kernel build date means affected; N/A or "not affected" means not;
  3. otherwise a series line (`NetBSD 11.*:` / `NetBSD 11:`);
  4. an advisory that names the series some other way (for example only
     `NetBSD 11.0_RC1`), or has no `Version:` block, cannot be assessed and
     makes base UNKNOWN.

  Replayed against the real 2016–2024 advisories, this gives the expected
  answers for 9.2, 9.3 and 10.0: for example SA2024-002 (regreSSHion)
  affects 10.0, and SA2022-002 (fixed on the 9 branch after 9.3 was built)
  affects 9.3. No advisory has been published since SA2024-002, so the
  advisory feed is currently silent for 11.0. The release-list checks are
  the working part of the base signal today.

**Implementation:** `frontends/scripts/netbsd-vuln-audit.sh` is installed as
`/usr/local/sbin/netbsd-vuln-audit` (ksh, runs under NetBSD's pdksh). It runs
from root cron once a day, after the host's pkgs/reboot window and before
`/etc/daily`: pi0 03:40, pi1 23:40 (`cluster.ValueVulnAuditCron`
`{minute, hour}`). The gonf tasks are `pis_netbsd_vuln_audit_packages`
(`curl`), `pis_netbsd_vuln_audit_script`, `pis_netbsd_vuln_audit_state_dir`
(`/var/db/netbsd-vuln-audit`, 0700) and `pis_netbsd_vuln_audit_cron`. They
are `VulnAudit*` methods on `netbsd.Unattended`, in `gonf/netbsd/vulnaudit.go`,
so they are part of the `pis_netbsd` aggregate. Each run:

0. waits up to 5 minutes for ntpd to report a synchronised clock (`ntpq -c
   rv`: a `sync_<source>` other than `sync_unspec`, no `leap_alarm`). The
   Pis have no RTC, so after a power-off the clock starts behind until ntpd
   syncs, and every age check below depends on it. Without sync both
   components are UNKNOWN, nothing is fetched and no contact is recorded
   (`clock_synced=no`). Independently, a contact stamp in the future, or a
   list dated more than 1 h in the future, is UNKNOWN ("clock behind"), so
   a negative age never passes as fresh.
1. refreshes the three sources with conditional GETs. A failed, corrupt or
   older download keeps the previous copy with a warning. Only a verified
   answer (or HTTP 304) is recorded as contact, in `contact.<source>`.
   More than 36 h without contact makes that component UNKNOWN: one missed
   daily run is tolerated, the second one is not. A deleted list in
   `/usr/pkg/pkgdb` is restored from the cache.
2. assesses each component on its own (`pkg_status`, `base_status`). pkgs is
   also UNKNOWN when `pkg_info` lists no packages, when `pkg_admin audit`
   writes to stderr or prints a line the parser does not know, or when it
   exits non-zero without findings. `pkg_info` and `pkg_admin audit` run
   under `unattended-upgrade-netbsd`'s lock `/var/run/unattended-upgrade.lock`
   (held for seconds only, not during downloads), so the audit never reads
   a pkgdb that pkgin is changing. If the upgrade job holds it for more than
   30 minutes, the run is skipped with a warning, exit 0, and the status
   record is left as it is; the next daily run retries. A lock older than
   2 h is stolen, as the upgrade job itself does.
3. compares each assessed component's findings with its lines in the
   baseline `findings`: new ones go to `new-findings`, gone ones to
   `resolved-findings`. An UNKNOWN component keeps its baseline lines and
   reports nothing, so findings that appear during an outage are reported
   once coverage returns. An unsorted or malformed baseline is treated as
   empty, with a warning. When neither component is assessed, both batch
   files are emptied (`new_findings=0`), so they never show an older run's
   batch.
4. writes `status` via a temp file, then commits: first the dated
   `new-findings.history` (90 days, one line per finding carrying its first
   alerting date), then the baseline. A failed status write keeps the old
   record and consumes nothing. A failed commit rewrites the record as
   UNKNOWN, naming the file.

| Status | Exit | When |
|---|---|---|
| OK | 0 | both components assessed from fresh, verified data; no finding |
| VULNERABLE | 0 | at least one finding (package advisory, end-of-life package, base advisory, newer release, unsupported series) |
| UNKNOWN | 3 | either component unassessed: clock not NTP-synchronised, or a contact stamp or list date in the future; a source never fetched or without contact for > 36 h, `pkg-vulnerabilities` missing, invalid or older than 30 days, pkg_admin errors or unparsable output, no packages, releases page or index unparsable, truncated, below the floor or missing a cached advisory, an advisory not downloadable or not assessable, a non-release kernel, state that cannot be written |

**Alerting:** there is no MTA on the Pis (§9.5). Every line goes to
`/var/log/unattended-upgrade.log` (tag `vuln-audit:`) and to syslog (tag
`netbsd-vuln-audit`, facility `daemon`, i.e. `/var/log/messages`), where DTail
can read it. Only `err` and `warning` lines are printed, so cron's mail to
root (should the Pis ever get a relay, §9.5 option b) carries alerts only and
a quiet run mails nothing.

| Signal | Meaning |
|---|---|
| `err`: `UNKNOWN: …`, exit 3 | coverage is broken and needs fixing |
| `warning`: `NEW: n finding(s): …` | findings not in the baseline (the first run reports all of them) |
| `warning`: `status changed: A -> B` | any status transition |
| `warning`: `WARNING: download failed` / `incomplete page` / `fails verification` / `older than the installed one` / `cannot refresh advisories` / `restored` / `baseline … malformed` | a refresh problem that still leaves usable coverage |
| `warning`: `WARNING: skipped, unattended-upgrade holds …` | the upgrade job kept its lock for > 30 min; this run was skipped, the status record is the previous one |
| `notice`: `RESOLVED: …`, unchanged VULNERABLE summary; `info`: OK summary | quiet runs |

**Known gap:** as on pi2/pi3 (§13), nothing forwards these signals off the
host, and nothing notices a cron job that stopped running (`checked_at` in
`status` has no consumer). The fix is the same §9.5 decision, for example a
gogios check reading `status`/`checked_at` from both Pis.

**Operator actions** (as root in `/var/db/netbsd-vuln-audit`):

- `cat status` shows the result. `pkg-audit.out` has the raw `pkg_admin
  audit` lines, `findings` the current findings.
- Delete `findings` to re-baseline. The next run reports everything as new
  once.
- Delete `advisories/NetBSD-SA….txt.asc` to accept that the index no longer
  lists that advisory.
- Remediation is manual: package findings go away when pkgin gets a fixed
  package (or the package is removed). A base finding means a release
  upgrade by hand (`sysupgrade` to the newer point release, or to a
  supported series), per §3/§9.2.

**First result (2026-09-22, run as paul on pi0 and pi1 against the live
sources, with the list installed into a temporary PKGVULNDIR, since nothing
may change on the hosts before deployment):** `VULNERABLE` on both Pis. pkgs
had 6 findings over 34 packages: libxml2-2.15.1 (CVE-2025-8732,
CVE-2026-0989, CVE-2026-0990, CVE-2026-0992, CVE-2026-1757) and
perl-5.42.3 (CVE-2011-4116, a permanent pkgsrc entry). Base was OK: 11.0 is
the newest release of the supported 11.x series, and neither in-scope
advisory (SA2024-001/002) concerns it. A run takes 3–5 s. pkgin had no
pending updates, so the libxml2 advisories are open upstream in pkgsrc.

**Tests:** `frontends/scripts/tests/netbsd-vuln-audit.ksh` fakes `uname`,
`pkg_admin`, `pkg_info`, `curl` and `logger`, and uses the real
gzip/awk/sort/comm/date. It passed under bash and ksh93 on the workstation
and under NetBSD's `/bin/ksh` on pi0 (in `/tmp`). It covers clean and
vulnerable runs, end-of-life packages alone and mixed with advisories, the
conditional GET and 304, deltas across package updates,
history pruning and deduplication, every advisory rule plus the
unassessable cases, a newer release, an unsupported series, and the
per-component baseline in both directions, and the upgrade lock (released
after a run, a held one skips without touching the status, a lock freed
while waiting, a stale one stolen). The negative cases: no ntpd sync or
ntpd not answering (nothing fetched, batch files emptied, baseline kept),
a contact stamp and a list dated in the future, never
fetched, one tolerated refresh failure, then UNKNOWN past 36 h, truncated
and non-gzip downloads, an older revision, a stale list, a restored list,
pkg_admin stderr, unparsable output or a non-zero exit, no packages,
incomplete and truncated pages, `</html>` present but not at the end, a
section without series, an index below the
floor or missing a cached advisory (and the operator override), an advisory
that cannot be downloaded (with and without a cache) or moved into the
cache, a non-release kernel
and an unparsable build date, invalid baselines, failed history, baseline,
resolved-list and status writes, a held lock, and an uncreatable state
directory.

**Deployment:** not yet deployed. It needs explicit approval (task 652
acceptance). When approved, run it from a tree that builds against the
committed gonf module version:
`./gonf.sh cluster netbsd-pis pis_netbsd_vuln_audit_packages pis_netbsd_vuln_audit_script pis_netbsd_vuln_audit_state_dir pis_netbsd_vuln_audit_cron`.
Then verify on each Pi with `doas /usr/local/sbin/netbsd-vuln-audit; echo
$?; doas cat /var/db/netbsd-vuln-audit/status; doas crontab -l | grep -A1
'Cron\[netbsd-vuln-audit\]'; pkg_admin audit`. The expected result is
VULNERABLE (the six package findings above), exit 0, and the first run's NEW
warning. `pkg_admin audit` now works interactively.
