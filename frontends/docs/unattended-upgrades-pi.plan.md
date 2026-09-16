# Unattended upgrades — pi0/pi1 (NetBSD) & pi2/pi3 (Rocky)

**Status: PLAN ONLY — nothing implemented yet. Deployment will be gonf-only** (per
paul's directive: no manual host manipulation or installation; the one-time
gonf-binary bootstrap per host is the sole exception, identical to how the
OpenBSD frontends were bootstrapped). Companion doc for the OpenBSD frontends:
[`unattended-upgrades.implementation.md`](./unattended-upgrades.implementation.md).

Companion facts verified live on 2026-09-16 (read-only probes).

## 1. Goals & scope

| Pair | Hosts | OS | Partner gate ("operational") | In scope |
|---|---|---|---|---|
| static-site pair | pi0, pi1 | NetBSD 11.0 (evbarm-aarch64) | partner **online AND serving HTTP**: fetch `http://<partner>.lan.buetow.org/` (resolvable via `/etc/hosts`, bozohttpd listens on `*.80`) and require a non-empty HTTP body | pkgsrc packages (`pkgin -y upgrade`), custom fleet packages (`pkg_add -u` from pkgrepo — dserver/DTail), reboot on kernel change |
| Pi-hole/LAN-DNS pair | pi2, pi3 | Rocky Linux 9.7 (aarch64) | partner **reachable**: `ping -c1 -W3 <partner-IP>` | dnf updates (official Rocky repos + the probe-gated `f3s-dtail` repo), reboot when `needs-restarting -r` says so |

Out of scope: Pi-hole container updates (`pihole -up` — deliberate, manual);
NetBSD base-system updates in phase 1 (see §3 research item).

Everything is deployed and managed via gonf. Cron, scripts, packages and
reboots are automation-managed; no runtime flags.

## 2. Verified host facts (2026-09-16, read-only)

| | pi0 | pi1 | pi2 | pi3 |
|---|---|---|---|---|
| OS | NetBSD 11.0 evbarm | same | Rocky 9.7 aarch64 (SIG/AltArch) | same |
| IP (LAN) | 192.168.1.125 | 192.168.1.126 | 192.168.1.127 | 192.168.1.128 |
| privilege | pkgsrc doas, `permit nopass :wheel` ✓ | ✓ | passwordless `sudo -n` ✓ | ✓ |
| shell | `/bin/ksh` (base) ✓ | ✓ | ksh **✗** (dnf installable) | ✗ |
| package tooling | `pkgin` ✓, `/usr/sbin/pkg_add` | ✓ | `dnf` ✓, `needs-restarting` ✓ | ✓ |
| fleet packages | dserver `/usr/local/bin` (custom pkgrepo) | same | dtail RPM (`f3s-dtail.repo`, baseurl `https://pkgrepo.f3s.buetow.org/rockylinux/9/$basearch/`, enabled) | same |
| partner gate | `http://pi1.lan.buetow.org/` | `http://pi0.lan.buetow.org/` | `ping 192.168.1.128` | `ping 192.168.1.127` |
| disk free (/) | 21 GB | 16 GB | 27 GB | 27 GB |
| root cron | 11 entries | 11 | — | — |

DNS/SSH gotchas verified live:

- pi0/pi1 do **not** resolve `*.wg0.wan.buetow.org` (the wg zone lives in the
  frontends' DNS) — partner checks must use `piN.lan.buetow.org`, which
  `/etc/hosts` maps ✓. bozohttpd serves it fine (`*.80`).
- pi2/pi3 do not resolve `piN.lan.buetow.org` — the ping gate uses the partner
  IP directly.
- `~/.ssh/config` maps `Host *.buetow.org` → **Port 2**: the gonf fleet Host
  entries for the Pis **must** set `WithSSHPort(22)` explicitly (discovered
  live; an unqualified ssh times out on port 2).

## 3. Update mechanism per OS

### NetBSD (pi0/pi1)

- **pkgsrc packages**: `pkgin -y upgrade` (34 packages installed). Unattended,
  non-interactive via `pkgin -y`.
- **Custom fleet packages** (dserver/DTail): `pkg_add -u
  https://pkgrepo.f3s.buetow.org/netbsd/11.0/packages/aarch64/dserver-<v>.tgz`
  (the f3s-pkgrepo client flow — explicit URL, no PKG_PATH). **Probe-gated**
  like the OpenBSD `pkgs` mode: the relayd front serves an HTTP-200 "Server
  turned off" page when the k3s backend is down → skip with a logged WARNING
  and exit 0. The repo is k3s-backed and the cluster sleeps for power saving,
  so custom-package windows only occur while it is awake (by design).
- **Base system (phase 2, optional)**: NetBSD has no syspatch. Option:
  install `sysutils/sysupdate` (via a gonf Package task) and track the
  `NetBSD-daily/netbsd-11/latest/evbarm-aarch64` autobuilds — kernel-set
  updates queue a partner-gated reboot. Phase 1 ships **without** base
  updates; release upgrades (11.0 → 11.1) stay deliberate manual `sysupgrade`
  work (out of unattended scope).
- **Reboot detection**: version compare — `what(1)` on `/netbsd` vs the booted
  version in `/var/run/dmesg.boot` line 1 (the same KARL-safe pattern as the
  OpenBSD script; NetBSD has no relink, so the compare is even simpler).
  NetBSD `dmesg.boot` lives at the same path ✓. All rc.d services
  (bozohttpd, wireguard-go, npf, uptimed, dserver) come back automatically.

### Rocky (pi2/pi3)

- `dnf -y upgrade` (official Rocky repos; the f3s-dtail repo **probe-gated**:
  when `pkgrepo.f3s.buetow.org/rockylinux/9/$basearch/` is down →
  `--disablerepo=f3s-dtail` + WARNING, official repos are unaffected).
- **Reboot**: `needs-restarting -r` (exit 1 = reboot required, typically after
  kernel updates) → partner-gated reboot (ping check).
- Pi-hole runs in Docker with a restart policy; OS updates and reboots do not
  require Pi-hole interaction. LAN DNS loss on one Pi is covered by the other
  (both serve `*.f3s.lan.buetow.org`).

## 4. Scripts

Two new ksh scripts mirroring the OpenBSD `unattended-upgrade.sh` skeleton
(log, lock, jitter, partner gate, probe gate, mode dispatch, no runtime flags):

- `frontends/scripts/unattended-upgrade-netbsd.sh` —
  `pkgin -y upgrade` / `pkg_add -u <custom url>` (probe-gated) / kernel
  version-compare reboot.
- `frontends/scripts/unattended-upgrade-rocky.sh` —
  `dnf -y upgrade` (f3s-dtail probe-gated via `--disablerepo`) /
  `needs-restarting -r` reboot gate.

Rocky has no ksh by default: a gonf `Package("ksh")` task installs it first
(dnf carries ksh) — keeping the ksh house rule. Alternative: a bash Rocky
script (decision point, see §7).

## 5. gonf integration

- Fleet `Host`s (in `internal/fleet`): pi0/pi1 (`paul@piN.lan.buetow.org`,
  port 22, `WithPrivilege(PrivilegeDoas)`) and pi2/pi3 (`paul@…`, port 22,
  `WithPrivilege(PrivilegeSudo)`). **`WithSSHPort(22)` is mandatory** (the
  `~/.ssh/config` wildcard maps `*.buetow.org` to port 2).
- gonf binaries: cross-compile for **netbsd/arm64** and **linux/arm64**;
  one-time bootstrap install to `/usr/local/bin/gonf` per host (the sole
  manual step; everything after via gonf).
- Tasks in `internal/frontends` (or a dedicated struct), gated
  `WhenHostname("pi0")` / `("pi1")` / `("pi2")` / `("pi3")`:
  - script install (per-OS path: NetBSD `/usr/local/sbin`, Rocky
    `/usr/local/sbin`),
  - cron install (gonf `Cron`, marker-based),
  - `ksh` + optionally `sysutils/sysupdate` via gonf `Package` (NetBSD pkgin;
    `Package` supports it),
  - log rotation: NetBSD `newsyslog.conf` `WithLine` (same as OpenBSD),
    Rocky `logrotate.d` file (gonf `File`).
- Privilege: pi0/pi1 `PrivilegeDoas`, pi2/pi3 `PrivilegeSudo` — both
  passwordless (verified).

## 6. Cron schedule proposal (staggered; decision for paul)

| Host | base/pkgsrc | packages | reboot |
|---|---|---|---|
| pi0 | 05:10 (+jitter) | 05:30 | 05:50 |
| pi1 | 20:10 (+jitter) | 20:30 | 20:50 |
| pi2 | 12:10 (+jitter) | — (single dnf window) | 12:30 (only when needed) |
| pi3 | 12:40 (+jitter) | — | 13:00 (only when needed) |

Windows are clear of the frontends' morning/evening cycles (06:10–07:10,
22:10–23:10) and of the gogios check window.

## 7. Rollout stages

0. Cross-compile + bootstrap-install the gonf binary on all four Pis (the one
   manual step).
1. Deploy via gonf to **pi0** (script, cron, log rotation) → validate one
   cycle → **pi1** (the partner gates make the pair safe: an update runs only
   while the partner is up and serving).
2. Deploy via gonf to **pi2** → validate → **pi3**.
3. Observe full cycles (logs, mail, gogios green).

## 8. Acceptance criteria

- [ ] Partner gates verified live on all four (pi0 blocked while pi1's HTTP is
      down-simulated; pi2 blocked while pi3 is ping-unreachable-simulated).
- [ ] `pkgin -y upgrade` logged; custom fleet packages update when the repo is
      up and are skipped with a WARNING when the k3s cluster is asleep.
- [ ] `dnf -y upgrade` logged; f3s-dtail repo skipped with a WARNING when the
      cluster is down; official Rocky repos unaffected.
- [ ] Kernel updates trigger the partner-gated reboot on one NetBSD Pi and one
      Rocky Pi; no HTTP downtime on the static site, no LAN DNS loss.
- [ ] No manual host steps anywhere after the gonf-binary bootstrap.

## 9. Open decisions for paul

1. Cron schedule table (§6) — accept or adjust.
2. NetBSD base updates: phase-2 `sysupdate` from the netbsd-11 autobuilds, or
   out of scope (release upgrades stay manual)?
3. Pi-hole container updates (`pihole -up`) — in or out of unattended scope?
4. Rocky script shell: ksh (via a gonf `Package("ksh")` install) or bash?

## 10. Risks & mitigations

| Risk | Mitigation |
|---|---|
| Partner down during a window | Partner gate skips (logged + mailed); updates wait for a healthy partner |
| k3s cluster asleep during `pkgs` | Probe skips custom-repo packages with a WARNING; official errata unaffected |
| Kernel update bricks a Pi | NetBSD: keep `/onestep`/release-media recovery handy; version-compare re-verifies before rebooting. Rocky: `dnf history rollback` |
| Pi-hole blip during Rocky reboot | Both Pis serve `*.f3s.lan.buetow.org`; clients hold the other as secondary |

## 11. Changelog

- **2026-09-16**: initial plan written from live host probes (no changes made
  to any Pi — everything will go through gonf per paul's directive).

## 12. Reuse on r0/r1/r2 (on-demand hosts): systemd hourly + once-per-day

The Rocky recipe is intended for reuse on the k3s hosts r0/r1/r2. Those hosts
are **online only occasionally** (powered off for energy savings), so a fixed
cron time would mostly miss them. Instead they get a **systemd timer that
fires hourly** and a **once-per-day gate**:

```ini
# /etc/systemd/system/unattended-upgrade-rocky.timer
[Unit]
Description=Hourly unattended-upgrade check (updates once per day)

[Timer]
OnBootSec=10min          # first check shortly after an on-demand boot
OnUnitActiveSec=1h       # then every hour while the host is up
RandomizedDelaySec=15min
Persistent=true          # catch up after a missed schedule

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

Script behaviour for the new `daily` mode (Rocky script):

1. Read the stamp `/var/lib/unattended-upgrade/last-daily` (a plain
   `date +%F` string, persistent across reboots).
2. Stamp equals today → **skip silently** (exit 0) — "skip until next day".
3. Otherwise run the full update flow exactly once: partner/cluster gate,
   pkgrepo probe (skip the f3s-dtail repo with a WARNING when the cluster is
   down), `dnf -y upgrade`.
4. On **success** → write the stamp. On **failure** → no stamp, so the next
   hourly tick retries automatically.
5. The **reboot check runs on every tick** (not daily-gated): if
   `needs-restarting -r` reports a pending kernel and the gates allow it, the
   host reboots at the next hourly tick — a kernel updated at 09:05 reboots at
   10:05, not a day later.

The hourly unit replaces a fixed cron for on-demand hosts: a daily cron at a
fixed time would simply miss most days, while the hourly tick + stamp means
the update happens within an hour of the host coming online, exactly once per
day.

**Cluster safety for reboots (proposal):** r0/r1/r2 are k3s server nodes (HA
tolerates one node down). Reboots are additionally staggered **by weekday**
(`date +%u % 3`: r0 → day 1, r1 → day 2, r2 → day 3) so two cluster nodes
never reboot on the same day, and each host's reboot is skipped unless at
least one sibling is reachable.

**Deployment via gonf:** the two unit files go in via gonf `File` tasks +
`DaemonReload` + enabling the timer; the Rocky script gains the `daily` mode.
The gonf binary on r0/r1/r2 is bootstrapped once (the same single manual step
as everywhere else).
