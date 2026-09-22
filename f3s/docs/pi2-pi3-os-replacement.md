# pi2/pi3: replacing Rocky SIG AltArch (task 852)

> **Decision (2026-09-22):** the owner rejected the Debian migration below;
> pi2/pi3 stay on Rocky 9 SIG AltArch. The follow-up tasks l82, m82, n82
> and o82 were dropped, and the l82 recipes were reverted (conf 0c88b37).
> The mitigation in place is the daily kernel CVE audit (task 752,
> `rocky-kernel-audit`) plus the p82 reboot-loop fix.


**Status: recommendation, nothing changed.** Research and read-only probes
done on 2026-09-22. No host has been reinstalled or reconfigured. Carrying out
the migration needs paul's explicit approval (follow-up tasks are listed in
§8).

**Recommendation: reinstall pi2 and pi3 with Debian 13 "trixie" arm64, using
the official Debian cloud-team `raspi` image and the stock Debian kernel
(`linux-image-arm64`).** Pi-hole keeps running as it does today, in Docker
(Docker CE apt repo, `network_mode: host`). Do pi3 first on a fresh SD card and
keep the current Rocky card as the rollback. Runner-up: Ubuntu Server 26.04
LTS. There is no supported Rocky/EL kernel path for a Raspberry Pi 3.

## 1. Problem

Facts checked on both hosts on 2026-09-22 (read-only SSH):

| | pi2 | pi3 |
|---|---|---|
| Hardware | Raspberry Pi 3 Model B Plus Rev 1.3, 1 GB RAM (909 MiB usable), 32 GB SD | same |
| OS | Rocky Linux 9.8 (Blue Onyx) aarch64, SIG/AltArch image | same |
| Running kernel | `6.1.31-v8.1.el9.altarch`, built **2023-06-10** | same |
| Installed kernel RPMs | `raspberrypi2-kernel4` 6.1.23 and 6.1.31, `raspberrypi2-firmware` 6.1.31 | same |
| Workload | `pihole/pihole:latest` (Docker 29.8.1, `network_mode: host`, `cap_add: NET_ADMIN`), `~/pihole` | same |
| Other units | firewalld (22, 53 tcp/udp, http, 2222), SELinux enforcing, chronyd, uptimed (+`time-sync.conf` drop-in), dserver drop-in + `dserver-update-keycache.timer`, `goprecords-upload.timer`, `unattended-upgrade-rocky.timer` | same |

The kernel comes from the `altarch-rockyrpi` repo. Its Rocky 9 package
directory holds just one kernel build, `raspberrypi2-kernel4-6.1.31-v8.1`,
dated 2023-06-10
([listing](https://dl.rockylinux.org/pub/sig/9/altarch/aarch64/altarch-rockyrpi/Packages/r/)).
The userland gets Rocky errata through `dnf updateinfo`. The kernel is
upstream 6.1.31 plus Raspberry Pi patches, and it has had no fixes in over
three years, so every kernel CVE since mid-2023 is still open on both DNS
servers. `dnf updateinfo` cannot show this, because the SIG kernel was never
part of Rocky's errata stream. Task 752 audits the current exposure; this
document is about ending it.

## 2. Requirements

1. Security fixes for the **kernel and base system**, shipped promptly and
   announced on an **authoritative advisory channel** (CVE to fixed-package
   mapping) that the unattended-upgrade and audit tooling can use.
2. **Raspberry Pi 3B+ (BCM2837B0, Cortex-A53, arm64) support** from the vendor,
   lasting several years.
3. **Docker Engine** on arm64 with host networking, so the current Pi-hole
   compose file, bind mounts and dnsmasq wildcard
   (`f3s/pihole/docker-pi/`) carry over unchanged.
4. Room for the partner-gated **unattended upgrades with reboot**
   (`frontends/docs/unattended-upgrades-pi.plan.md`) managed through gonf.
5. **Low migration effort, easy rollback**, and fits in 1 GB of RAM.

## 3. Candidates at a glance

| Candidate | Pi 3B+ support | Kernel source and update cadence | Advisory channel | Security lifetime | Docker + host-net Pi-hole | Migration effort | Verdict |
|---|---|---|---|---|---|---|---|
| **Rocky 9 SIG AltArch** (status quo) | Yes (Rocky 9 images) | `raspberrypi2-kernel4` 6.1.31, **no update since 2023-06** | None for the kernel | Userland until 2032; **kernel already dead** | Yes | none | **Reject**, unfixable |
| **Rocky 10 SIG AltArch** | **No**. Rocky 10 supports only Pi 4/5 ([The Register](https://www.theregister.com/2025/06/14/rocky_alma_and_rhel_10/)) | `kernel-rpi` 6.12.103 (2026-08-19) | None | n/a on Pi 3 | n/a | n/a | **Reject** |
| **Stock EL9 kernel** (`kernel` from BaseOS) | **No**. Rocky 9.8 aarch64 `kernel-core-5.14.0-687.49.1` config has `# CONFIG_ARCH_BCM2835 is not set` (checked 2026-09-22) | RHEL cadence | RLSA/updateinfo | 2032 | n/a | n/a | **Reject**, does not boot the SoC |
| **AlmaLinux 9 RPi kernel** (same package name `raspberrypi2-kernel4`) | Yes | 6.12.96 (2026-07-31). Irregular: 6.12.47 was the latest from 2025-10 to 2026-04 ([listing](https://repo.almalinux.org/almalinux/9/raspberrypi/aarch64/os/Packages/)) | **None**. Not in ALSA errata; Dirty Frag CVE-2026-43284 stayed open in it after the main kernel was fixed ([list thread, 2026-05-09](https://lists.almalinux.org/mailman3/lists/security.lists.almalinux.org/thread/SBXN3YTSZV3KLRIH5EBHO7MRLK5GRMWQ/)) | Best effort | Yes | Low (swap the repo), but mixes vendors | **Reject** as a destination; at most a short bridge (§6) |
| **Debian 13 trixie, official `raspi` image** | Yes. Debian builds `raspi` images for "families 3, 4 and 5" ([raspi.debian.net](https://raspi.debian.net/), [Debian wiki](https://wiki.debian.org/RaspberryPiImages)) | Stock `linux-image-arm64`. The 2026-09-14 image ships `6.12.107+deb13` ([image manifest](https://cloud.debian.org/images/cloud/trixie/latest/debian-13-raspi-arm64.json)) | **DSA/DLA + Debian security tracker**, arm64 covered | Security to **2028-08-09**, LTS to **2030-06-30** ([trixie](https://www.debian.org/releases/trixie/), [LTS](https://wiki.debian.org/LTS)) | Yes. Docker CE supports trixie arm64 ([docs](https://docs.docker.com/engine/install/debian/)); Pi-hole supports Debian ([prereqs](https://docs.pi-hole.net/main/prerequisites/)) | Medium (reflash) | **Recommended** |
| **Raspberry Pi OS Lite 64-bit** (trixie) | Yes, first-class; the 3B/3B+ are listed ([downloads](https://www.raspberrypi.com/software/operating-systems/)) | Raspberry Pi kernel (6.18 in the 2026-09-15 release), shipped through `archive.raspberrypi.com` | Debian DSA for Debian packages. The Raspberry Pi kernel and firmware get **no per-CVE advisories**; the [Raspberry Pi security page](https://www.raspberrypi.com/security/) covers only issues in its own products | Tracks Debian; no stated EOL for the kernel overlay | Yes (Pi-hole's reference platform) | Medium (reflash) | Good fallback if the Debian kernel has Pi 3 problems |
| **Ubuntu Server 26.04 LTS** | Yes, 3B/3B+ are "S" (server supported) ([Ubuntu matrix](https://ubuntu.com/hardware/docs/boards/how-to/ubuntu_supported/raspberry-pi/)) | `linux-raspi`, on the SRU cycle | **USN** covers `linux-raspi` (e.g. [USN-8659-1](https://ubuntu.com/security/notices/USN-8659-1)) | Standard to 2031-04; ESM to 2036 with Ubuntu Pro (free for up to 5 machines, [release cycle](https://ubuntu.com/about/release-cycle)) | Yes | Medium (reflash) | **Runner-up** |
| **Fedora 44** | Yes, 3-series supported ([Fedora wiki](https://fedoraproject.org/wiki/Architectures/ARM/Raspberry_Pi)) | Fedora kernel, fast | Bodhi updates | About 13 months per release, so two major upgrades a year | Yes (moby/Docker CE) | Medium, plus recurring upgrades | Reject: too much churn for a DNS pair |
| **Alpine 3.24** | Yes (`linux-rpi`) | Alpine kernel | Alpine secdb | `main` 2 years (until 2028-06); **`community` (where docker lives) only until the next release**, about 6 months ([releases](https://alpinelinux.org/releases/)) | Yes, but Docker support is short | Medium-high (diskless/lbu model, musl) | Reject |
| **NetBSD 11** (as on pi0/pi1) | Yes | NetBSD kernel | NetBSD SAs, few and slow for the kernel | Release branch | **No Docker**. Pi-hole would have to become plain dnsmasq/unbound plus blocklists | High (DNS stack rewrite) | Reject for this role, despite matching pi0/pi1 |
| New hardware (Pi 4/5) + Rocky 10 | Pi 4/5 only | `kernel-rpi` 6.12.x | None for the SIG kernel | Same SIG risk as today | Yes | Buys hardware; keeps a SIG kernel | Out of scope; still no advisory channel |

## 4. Why Debian 13 official over the others

- **The kernel is covered by an authoritative security process.** The Pi 3
  runs Debian's own `linux` source package, the same one amd64 servers run, so
  every kernel CVE is tracked on the
  [Debian security tracker](https://security-tracker.debian.org/tracker/source-package/linux)
  and fixed through DSA (later DLA under LTS). No other Pi 3 option offers
  both a distro-maintained kernel and a CVE-level channel, except Ubuntu.
- **Long enough.** Security support until 2028-08, then LTS (arm64 has been
  covered in every LTS cycle so far) until 2030-06. An in-place
  `apt full-upgrade` to Debian 14 is a supported path, so no reflash is needed
  later.
- **Light.** No snapd and no Ubuntu Pro enrolment. The image is a minimal
  cloud-team build (`raspi-firmware`, `linux-image-arm64`,
  `firmware-brcm80211`), which suits a 1 GB board that only runs Docker and
  Pi-hole.
- **Tooling fits.** `unattended-upgrades` with a security `Origins-Pattern`,
  plus the `/run/reboot-required` flag, map directly onto the partner-gated
  script design in `unattended-upgrades-pi.plan.md`. For task 752-style
  auditing, `debsecan` works against the tracker.
- **Why not Ubuntu 26.04:** it has a longer lifetime and USN coverage of
  `linux-raspi`, and would be an equally sound choice on security. It loses on
  footprint (snapd, cloud-init) and on the trend: Ubuntu keeps narrowing its
  Pi 3 support, and the matrix now marks the 3B+ as server-only. If the Debian
  kernel shows Pi 3 problems during validation, pick Ubuntu 26.04 next.
- **Why not Raspberry Pi OS:** it has the best hardware support, but its
  kernel and firmware come from Raspberry Pi's own archive without per-CVE
  advisories. That repeats, in a milder form, the problem this task is
  meant to remove.

Known trade-offs of the Debian kernel on a Pi 3B+: the Raspberry Pi
`config.txt` overlays and the vendor video stack are not used, and the onboard
Wi-Fi/Bluetooth needs `firmware-brcm80211`, which the image already ships.
None of this matters for a headless, wired DNS server. The 3B+ Ethernet
(`lan78xx`) and the SD controller are in mainline and must be verified in
stage 1.

## 5. What changes in the homelab

| Area | Today (Rocky) | After (Debian) |
|---|---|---|
| Packages | dnf, EPEL, `docker-ce-stable` rpm repo | apt, Docker CE apt repo (`download.docker.com/linux/debian trixie stable`) |
| Unattended upgrades | gonf `rocky.Unattended` (`gonf/rocky/unattended.go`), cluster `NameRockyPis`/`NameRockyAll` | New gonf `debian.Unattended` and cluster `NameDebianPis`; pi2/pi3 leave `NameRockyAll` |
| Reboot signal | `needs-restarting -r` | `/run/reboot-required` (+ `needrestart` for services) |
| Firewall | firewalld (22, 53, http, 2222) | nftables ruleset with the same ports, managed by gonf |
| MAC | SELinux enforcing | AppArmor (Debian default) |
| DTail | `f3s-dtail` rpm repo on pkgrepo | Needs an arm64 `.deb` or a gonf-managed binary + systemd unit (the pkgrepo has no Debian repo today; see the `f3s-pkgrepo` skill) |
| uptimed / goprecords | rpm + `time-sync.conf` drop-in | Debian `uptimed` package + the same `chronyc waitsync` drop-in |
| Pi-hole | `~/pihole` compose, bind mounts, `.env` | Unchanged; copy the directory over |
| Networking | NetworkManager static IP | Static IP via systemd-networkd or ifupdown (whichever the image uses); same IPs 192.168.1.127 / .128 |
| Privilege | `sudo` (inventory `PrivilegeSudo`) | `sudo`, unchanged |
| SD-card writes | No wear-specific settings in this repo | Keep write volume low: journald `Storage=volatile` (or a small `SystemMaxUse`), Docker `json-file` log limits (`max-size`/`max-file`), Pi-hole query-log retention kept short (`database.maxDBdays` in `/etc/pihole/pihole.toml`), `noatime` on the root mount; apply the same settings through gonf |

## 6. Interim risk until the migration

Until the reflash, the 2023 kernel stays exposed. Mitigating factors: both
hosts are LAN-only (no WireGuard, no public exposure), run no untrusted local
users, and serve DNS and the Pi-hole UI only. Local privilege escalations
(Dirty Frag class) matter less than remotely reachable network-stack bugs.
Cross-installing the AlmaLinux `raspberrypi2-kernel4` build would bring a
2026 kernel, but it mixes vendors, has no advisory channel, and its own history
shows months-long gaps. It is **not recommended** except as an explicitly
approved one-off bridge if the migration slips by months. Task 752 keeps
tracking the exposure in the meantime.

## 7. Staged migration and validation plan

Each Pi gets a **new SD card**. The current Rocky card is kept untouched as
the rollback: power off, swap the card back, and the host returns to its
current state in minutes. At every stage one Pi-hole stays in service
(clients use pi2, then pi3, then the router, per `f3s/pihole/README.md`).

**Stage 0: prepare in the repo (no host changes).**
Write the gonf Debian unattended-upgrade recipe and add the inventory
(cluster `NameDebianPis`, move pi2/pi3 out of `NameRockyPis`/`NameRockyAll`
once each is migrated). Provide DTail for Debian arm64. Write a short
reinstall runbook (flash, first boot, user `paul` + SSH key + sudo, static IP,
hostname).

**Stage 1: pi3 (the secondary resolver).**
1. Back up `~/pihole` from pi3 (`etc-pihole`, `etc-dnsmasq.d`, `.env`,
   `docker-compose.yml`) and take a Pi-hole Teleporter export.
2. Flash `debian-13-raspi-arm64` onto the new card and boot. Set the hostname
   `pi3.lan.buetow.org`, static IP 192.168.1.128, user `paul`, SSH key and
   passwordless sudo.
3. Run the one-time gonf bootstrap, then deploy the Debian recipes: Docker CE,
   nftables, uptimed + drop-in, dserver, goprecords timer, unattended upgrades.
4. Restore `~/pihole` and run `docker compose up -d`.
5. Validate:
   - `uname -r` shows a `+deb13-arm64` kernel
   - `apt-get -s upgrade` is clean
   - `debsecan --suite trixie --only-fixed` is empty
   - `dig @192.168.1.128 foo.f3s.lan.buetow.org +short` returns `192.168.1.138`
   - an external name resolves
   - the admin UI works
   - SSH on 22 and dserver on 2222 work
   - a reboot comes back with Pi-hole healthy and uptimed started after chrony sync
   - a manual partner-gated unattended run succeeds
   - the SD-card write settings are active (`journalctl --header` lists
     journal files under `/run/log/journal` for volatile storage, or
     `systemd-analyze cat-config systemd/journald.conf` shows the
     `SystemMaxUse` cap; `findmnt /` shows `noatime`; `docker inspect`
     shows the log limits)
6. Soak for at least 7 days, watching DNS answers, memory (`free -m`) and
   the unattended logs.

**Stage 2: pi2 (the primary resolver).** Repeat stage 1 for pi2
(192.168.1.127). Clients fall back to pi3 during the swap.

**Stage 3: clean up.** Drop pi2/pi3 from `rocky.Unattended` and the Rocky
clusters. Update the `f3s` and `f3s-raspberry-pi` skills (they say "Rocky
Linux 9"), `f3s/pihole/README.md` and `unattended-upgrades-pi.plan.md`. Close
or retarget the Rocky SIG kernel audit from task 752. Keep the old Rocky cards
for one month, then wipe them.

**Abort criteria (roll back to the Rocky card):** Ethernet or SD
instability, a kernel that does not boot the 3B+, Pi-hole unable to bind 53
with host networking, or memory pressure. If the Debian kernel itself is the
problem, repeat stage 1 with Ubuntu Server 26.04 LTS.

## 8. Follow-up tasks

Created in `ask` (project `conf`). Carry them out only after paul approves
the recommendation:

- **l82**: gonf Debian unattended-upgrade recipe + `NameDebianPis` inventory
  (stage 0)
- **m82**: DTail on Debian arm64 (stage 0)
- **n82**: migrate pi3 to Debian 13 (stage 1; depends on l82 and m82; needs
  approval and a physical SD card swap)
- **o82**: migrate pi2 to Debian 13 and clean up (stages 2 and 3; depends on
  n82)

## Sources

- Rocky 9 SIG RPi kernel listing: <https://dl.rockylinux.org/pub/sig/9/altarch/aarch64/altarch-rockyrpi/Packages/r/>
- Rocky 10 SIG RPi kernel listing: <https://dl.rockylinux.org/pub/sig/10/altarch/aarch64/altarch-rockyrpi/Packages/k/>
- Rocky 10 drops Pi 3: <https://www.theregister.com/2025/06/14/rocky_alma_and_rhel_10/>
- Rocky 9 BaseOS aarch64 kernel config (`kernel-core-5.14.0-687.49.1.el9_8`): <https://dl.rockylinux.org/pub/rocky/9/BaseOS/aarch64/os/Packages/k/>
- AlmaLinux 9 RPi repo: <https://repo.almalinux.org/almalinux/9/raspberrypi/aarch64/os/Packages/>
- AlmaLinux Dirty Frag RPi kernel thread: <https://lists.almalinux.org/mailman3/lists/security.lists.almalinux.org/thread/SBXN3YTSZV3KLRIH5EBHO7MRLK5GRMWQ/>
- Debian trixie lifecycle: <https://www.debian.org/releases/trixie/>; LTS: <https://wiki.debian.org/LTS>
- Debian Raspberry Pi images: <https://wiki.debian.org/RaspberryPiImages>, <https://raspi.debian.net/>, <https://cloud.debian.org/images/cloud/trixie/latest/>
- Debian security tracker (linux): <https://security-tracker.debian.org/tracker/source-package/linux>
- Raspberry Pi OS: <https://www.raspberrypi.com/software/operating-systems/>; security: <https://www.raspberrypi.com/security/>
- Ubuntu Raspberry Pi support matrix: <https://ubuntu.com/hardware/docs/boards/how-to/ubuntu_supported/raspberry-pi/>; release cycle: <https://ubuntu.com/about/release-cycle>; USN-8659-1: <https://ubuntu.com/security/notices/USN-8659-1>
- Fedora on Raspberry Pi: <https://fedoraproject.org/wiki/Architectures/ARM/Raspberry_Pi>
- Alpine releases: <https://alpinelinux.org/releases/>
- Docker Engine on Debian: <https://docs.docker.com/engine/install/debian/>
- Pi-hole prerequisites: <https://docs.pi-hole.net/main/prerequisites/>
