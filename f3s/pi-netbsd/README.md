# Remote in-place NetBSD install on a Raspberry Pi 3B+ (no SD removal)

Runbook + scripts to replace **Rocky Linux 9** with **NetBSD 11.0** on an f3s
Raspberry Pi **3 Model B+**, entirely over SSH, **without pulling the microSD
card**. Proven on `pi0` (2026-07-03); this document is written so the same can
be done to `pi1` (or a rebuild of `pi0`) by changing two values.

> **TL;DR of the hard-won lesson:** `kexec` is compiled out of the Rocky RPi
> kernel and the systemd shutdown-pivot does not fire on these boxes, so the
> only remote RAM-flasher that works is one loaded by the **Pi firmware** as an
> `initramfs` via a one-shot `config.txt`. See [Why the obvious methods
> fail](#why-the-obvious-methods-fail).

---

## Contents

- [Outcome / definition of done](#outcome--definition-of-done)
- [Hardware & facts](#hardware--facts)
- [Why the obvious methods fail](#why-the-obvious-methods-fail)
- [How it works (the method that does)](#how-it-works-the-method-that-does)
- [Prerequisites](#prerequisites)
- [Stage A — bake the golden image (on `earth`)](#stage-a--bake-the-golden-image-on-earth)
- [Stage B — remote in-place flash (on the Pi)](#stage-b--remote-in-place-flash-on-the-pi)
- [Doing pi1 specifically](#doing-pi1-specifically)
- [Verification](#verification)
- [Rollback / recovery](#rollback--recovery)
- [Troubleshooting & gotchas](#troubleshooting--gotchas)
- [Cleanup](#cleanup)
- [Post-install (deferred)](#post-install-deferred)
- [File manifest](#file-manifest)

---

## Outcome / definition of done

`ssh paul@<pi-ip>` lands on a **NetBSD 11.0 (GENERIC64) evbarm/aarch64** system:

- login as `paul` via **SSH key** (member of `wheel`); password auth also on as a backup
- hostname correct (`piN.lan.buetow.org`)
- **static IP on `mue0`** (the 3B+ onboard LAN78xx NIC in NetBSD), default route + DNS working
- root filesystem auto-resized to the whole card on first boot

## Hardware & facts

| Item | pi0 | pi1 |
|------|-----|-----|
| Board | Raspberry Pi 3 Model B+ | Raspberry Pi 3 Model B+ |
| Was running | Rocky Linux 9.7 aarch64 | Rocky Linux 9.x aarch64 |
| LAN IP | 192.168.1.125 | **192.168.1.126** |
| Hostname | pi0.lan.buetow.org | **pi1.lan.buetow.org** |
| WireGuard | 192.168.2.203 | 192.168.2.204 |
| RAM | 909 MiB | 909 MiB |
| SD | `/dev/mmcblk0` (30 GB): p1 `/boot` vfat, p2 swap, p3 `/` ext4 | same |
| NIC (Linux → NetBSD) | `lan78xx` → **`mue0`** | `lan78xx` → **`mue0`** |
| Gateway / DNS | 192.168.1.1 | 192.168.1.1 |

Confirmed on the RPi Rocky kernel (`6.1.31-v8.1.el9.altarch`): `CONFIG_KEXEC`
disabled, `CONFIG_KEXEC_FILE` unset; **boots with no initramfs** (kernel mounts
the ext4 root directly); `ext4`, `vfat`, `nls_cp437`, `nls_ascii`, `mmc_block`,
`sdhci`, `tmpfs` are all **built-in** (`=y`) — so a flasher initramfs needs no
extra modules to mount `/boot`, the root, or a tmpfs.

NetBSD base image used: **NetBSD 11.0 evbarm-aarch64 gzimg**
`https://cdn.netbsd.org/pub/NetBSD/NetBSD-11.0/evbarm-aarch64/binary/gzimg/arm64.img.gz`.
The bake script pins and verifies the release image's SHA-512 before
decompression, then runs `gzip -t`; a stale or partial cached download fails
closed. Update the pinned digest only after independently verifying a formal
replacement release image.
(GPT: EFI System partition + NetBSD FFS root; boots RPi 3/4/5).

## Why the obvious methods fail

Three RAM-flasher mechanisms were tried on `pi0`. Only the third works here:

1. **`kexec` into a RAM flasher** (what most guides assume). ❌ Impossible:
   `kexec_load(2)` returns `ENOSYS` and `kexec_file_load(2)` is not built
   (`CONFIG_KEXEC_FILE` unset). `kexec-tools` is installed but the syscalls are
   not in the kernel.
2. **systemd shutdown-pivot** (`/run/initramfs/shutdown` switch-root at reboot).
   ❌ Does **not fire** on these boxes — proven with a 3-channel evidence probe
   over several test reboots; `systemd-shutdown` never switch-roots into
   `/run/initramfs` here (the box was not booted from an initrd, and
   `dracut-shutdown` is a no-op because `.need_shutdown` is never created).
   *Also_ a red herring along the way: mounting a **separate tmpfs** at
   `/run/initramfs` gets unmounted during shutdown — always populate it as a
   plain dir on `/run` if you ever revisit this.
3. **Pi-firmware `initramfs`** (this runbook). ✅ Works. The VideoCore firmware
   loads a small dracut initramfs right after the kernel; a dracut *pre-mount*
   hook runs in RAM before the real root is mounted and does the flash.

## How it works (the method that does)

```
 EARTH (x86_64 laptop)                         piN (RPi 3B+, Rocky)
 ─────────────────────                         ─────────────────────
 Stage A: bake golden image                    Stage B: remote flash
   qemu-system-aarch64 -M virt (TCG)             1. build flasher.img (dracut,
     boots stock NetBSD arm64.img                   pre-mount hook)  -> /boot
     -> configure headless (sshd,                2. arm: write /boot/config.txt
        static mue0, user+key, pw)                    = "initramfs flasher.img
     -> sync + poweroff                              followkernel"  + trigger
   gzip -> netbsd-piN-golden.img.gz                  file /boot/netbsd-flash-mode
        │                                        3. systemctl reboot
        └── scp to piN:/home/paul/ ───────────►  ── firmware loads flasher.img ──┐
                                                                                 ▼
                                              dracut pre-mount hook (95netbsdflash):
                                               - DISARM: rm /boot/config.txt + trigger
                                                 (so any later failure self-heals to Rocky)
                                               - mount ext4 root ro, copy golden gz -> tmpfs (RAM)
                                               - dryrun: gunzip|wc -c  (validate, reboot to Rocky)
                                               - real:   gunzip|dd of=/dev/mmcblk0  (reboot to NetBSD)
                                                                                 ▼
                                              NetBSD 11.0 boots headless, static .12x, sshd
                                                                                 ▼
                                              ssh paul@<ip>  ✅ ACCEPTANCE
```

Key safety property: the hook **removes `config.txt` before touching anything
else**, so the flasher is strictly one-shot and any failure *after the hook
starts* leaves a card that boots normally (Rocky, or — once flashed — NetBSD).
The **dry-run** proves the entire boot+stage+decompress path non-destructively
before the real `dd`.

## Prerequisites

On **earth** (the flashing workstation):

- `qemu-system-aarch64`, `qemu-img`, `edk2` AAVMF firmware, `expect`, passwordless `sudo`
  (`sudo dnf install -y qemu-system-aarch64 qemu-img edk2-aarch64 expect`).
- Note: earth is x86_64, so the aarch64 guest runs under **TCG emulation** (slow
  but fine — a bake is a handful of minutes).
- Your SSH public key (`~/.ssh/id_rsa.pub`) — it gets baked into the image.

On the **Pi**:

- Reachable over SSH as `paul` with **passwordless sudo** (the f3s default).
- `dracut` present (it is on Rocky), `/boot` is the vfat firmware partition,
  ~500 MB free on `/` to stage the image.
- Its twin Pi still serving the shared role (so downtime is a non-event).

## Stage A — bake the golden image (on `earth`)

Scripts: [`bake/`](bake). Editing the NetBSD FFS root from Linux is unsafe, so
we configure the image **from inside a real NetBSD** running under qemu.

1. Choose `pi0` or `pi1`; the orchestrator renders that host's `HOSTNAME` and
   `IPADDR` into [`bake/setup.sh`](bake/setup.sh) (see
   [Doing pi1](#doing-pi1-specifically)).
2. Run the orchestrator:
   ```bash
   cd f3s/pi-netbsd/bake
   ./bake-golden.sh piN            # e.g. ./bake-golden.sh pi1
   ```
   It downloads `arm64.img.gz` (if absent), serves `setup.sh` over a localhost
   HTTP server, boots the image in qemu, blind-drives the console via
   `config.exp` to fetch+run `setup.sh`, verifies the on-disk result, then
   `sync`+powers off and produces `netbsd-piN-golden.img.gz`.
3. A random backup password is generated and printed (also saved to
   `piN-cred.txt`). The **SSH key is the primary login**; change the password
   after install.

What `setup.sh` configures inside the image: `sshd=YES`, `hostname`,
`ifconfig_mue0="inet <ip> netmask 0xffffff00"`, `defaultroute`, `dhcpcd=NO`,
`/etc/resolv.conf`, user `paul` (+`wheel`) with your `authorized_keys` and an
argon2id password (root too), `sshd_config` pubkey+password auth, and an
`/etc/rc.local` fallback that puts the static IP on the first real ethernet
interface if `mue0` is ever named differently.

> **The qemu-automation gotchas** (already handled in the scripts, documented so
> you understand them): add a **virtio-rng** device or NetBSD stalls on entropy
> and never generates ssh host keys; run `expect` under **`LC_ALL=C`** or Tcl
> chokes on the serial control bytes; **blind-drive** the login/commands with
> fixed sleeps and judge success by markers written to the log rather than
> matching the flaky console; and **`sync` then wait for a clean poweroff** —
> killing qemu before the FFS is flushed corrupts `rc.conf`/`pwd.db`.

## Stage B — remote in-place flash (on the Pi)

Scripts: [`flash/`](flash). Copy this whole `pi-netbsd/` tree (or at least
`flash/`) to the Pi first, e.g. `scp -r f3s/pi-netbsd paul@piN.lan.buetow.org:`.

1. **Stage the golden image on the Pi** (on its ext4 root):
   ```bash
   scp netbsd-piN-golden.img.gz paul@piN.lan.buetow.org:/home/paul/
   ```
   The flasher hook globs for **`/home/paul/netbsd-*-golden.img.gz`** (or under
   `/root/`), so any `netbsd-<pi>-golden.img.gz` name works — no rename needed.
   Verify integrity: `gzip -t` and compare `sha256sum` against earth.
2. **Build the flasher initramfs** (module name is `netbsdflash`, no numeric prefix):
   ```bash
   ssh paul@piN.lan.buetow.org 'sudo /home/paul/pi-netbsd/flash/build-flasher.sh'
   ```
   Produces `/boot/flasher.img` (~38 MB) and prints an `lsinitrd` sanity check.
3. **Dry-run first (non-destructive):**
   ```bash
   ssh paul@piN.lan.buetow.org '
     sudo rm -f /boot/flash-evidence.txt
     printf "initramfs flasher.img followkernel\n" | sudo tee /boot/config.txt
     printf "dryrun\n" | sudo tee /boot/netbsd-flash-mode
     sync; sudo systemctl reboot'
   ```
   The Pi goes down ~90 s (staging + decompress), then returns as **Rocky**.
   Read the proof:
   ```bash
   ssh paul@piN.lan.buetow.org 'sudo cat /boot/flash-evidence.txt'
   # EXPECT:
   #   FLASHER-RAN mode=dryrun up=...
   #   DRYRUN-RESULT bytes=<golden-image-size> rc=0
   ```
   Compare `bytes=` with `gzip -l netbsd-piN-golden.img.gz` on earth. A match,
   `rc=0`, and `config.txt` gone means the whole path works.
   **Do not proceed to the real flash unless the dry-run shows this.**
4. **Real flash (destructive, irreversible):**
   ```bash
   ssh paul@piN.lan.buetow.org '
     sudo rm -f /boot/flash-evidence.txt
     printf "initramfs flasher.img followkernel\n" | sudo tee /boot/config.txt
     printf "real\n" | sudo tee /boot/netbsd-flash-mode
     sync; sudo systemctl reboot'
   ```
   The Pi stages to RAM, `dd`s the 1.59 GB image onto `/dev/mmcblk0`, then
   reboots into **NetBSD**. Allow several minutes (SD write + first boot +
   root resize).

## Doing pi1 specifically

Only two rendered values differ from pi0:

```sh
HOSTNAME="pi1.lan.buetow.org"
IPADDR="192.168.1.126"
```

Do not edit the template: `./bake-golden.sh pi1` substitutes these values.
Everything else is identical: same board, same NIC (`mue0`), same gateway/DNS,
same flasher. WireGuard peer for pi1 is `192.168.2.204` (configured later,
out of scope here). Run Stage B against `pi1.lan.buetow.org` /
`192.168.1.126`.

> Keep pi0 (or the other twin) up while flashing pi1 so the static
> `f3s.buetow.org` backend stays served.

## Verification

```bash
ssh -i ~/.ssh/id_rsa -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null paul@192.168.1.126 '
  uname -a                       # NetBSD piN.lan.buetow.org 11.0 ... evbarm
  hostname                       # piN.lan.buetow.org
  id                             # uid=1000(paul) ... groups ... wheel
  ifconfig mue0                  # inet 192.168.1.126
  netstat -rn -f inet | grep default   # default 192.168.1.1 ... mue0
  host cdn.netbsd.org            # DNS resolves
  df -h /                        # root grown to ~29G'
```

NetBSD regenerates its own SSH **host keys**, so your `known_hosts` entry for the
Pi will change — remove the old line or use the relaxed flags above.

## Rollback / recovery

- **Before the real flash:** nothing is destroyed. Any failure once the hook has
  started leaves the card booting normally (the hook removes `config.txt`
  first). If the initramfs never runs the hook (rare — a malformed
  `flasher.img`), the Pi boot-loops on the missing/failed initramfs until you
  fix it physically: pull the SD, delete `config.txt`, reinsert. Mitigation is
  the dry-run, which proves the initramfs boots.
- **After the real flash:** the card is NetBSD. To go back to Rocky, reflash the
  Rocky image (keep a known-good one handy) — physically or by the same method
  in reverse. **Keep a Rocky SD image before starting** as the ultimate fallback
  (physical access is "inconvenient but possible" for these boxes).
- **A later in-place base upgrade:** follow
  [`NETBSD-11-PI1-UPGRADE.md`](NETBSD-11-PI1-UPGRADE.md). In particular, run
  userland sets and the conditional orderly reboot in one HUP-resistant root
  shell. Replacing userland can make the old `sshd` reject every fresh login;
  never rely on opening another SSH session between sets extraction and reboot.

## Troubleshooting & gotchas

- **Dry-run returns fast (~20 s) with no `flash-evidence.txt`:** the initramfs
  did not run the hook — check `/boot/config.txt` really says
  `initramfs flasher.img followkernel`, that `/boot/flasher.img` exists, and that
  the trigger file `/boot/netbsd-flash-mode` contains exactly `dryrun`.
- **`dracut module 'netbsdflash' cannot be found`:** use `--add netbsdflash`
  (module name = directory name **without** the `95` prefix).
- **`bytes=` differs from `gzip -l` on earth or `rc!=0`:** the staged/decompressed image is
  bad — re-check the `scp`/`sha256sum` of the golden gz on the Pi.
- **NetBSD boots but is unreachable:** the NIC came up under a name other than
  `mue0`; the `rc.local` fallback should still put `.12x` on the first real
  ethernet iface — attach a console (HDMI/serial) to inspect if needed.
- **qemu bake stalls / login never proceeds:** see the Stage A gotchas box
  (virtio-rng, `LC_ALL=C`, blind-drive, sync-before-poweroff).

## Cleanup

On the Pi (once NetBSD is confirmed, these are on the wiped card anyway for a
real flash; relevant only if you leave a Pi on Rocky after dry-runs):

```bash
sudo rm -f /boot/config.txt /boot/netbsd-flash-mode /boot/flash-evidence.txt \
           /boot/flasher.img /home/paul/netbsd-pi0-golden.img.gz
# optional: restore the untouched initramfs backup
sudo mv /boot/initramfs-*.img.orig /boot/initramfs-$(uname -r).img   # if present
```

On earth: stop any leftover `python3 -m http.server` from the bake; the
`*.img`/`*.img.gz` work files can be deleted or kept for the next Pi.

## Post-install (deferred)

Base OS + networking + SSH is the whole scope of this runbook. Re-provisioning
the Pi's role on NetBSD is separate work:

- **lighttpd static site** (`f3s.buetow.org`): `pkgin install lighttpd`, recreate
  `/var/www` + the Host-based vhost config. relayd backends keep the same IPs, so
  the OpenBSD frontends need no change.
- **DTail `dserver`**: needs a **`netbsd/arm64`** build (the Pis previously ran
  `linux/arm64`).
- **WireGuard** peer, monitoring agents, etc.

## File manifest

```
f3s/pi-netbsd/
├── README.md                      # this runbook
├── bake/
│   ├── bake-golden.sh             # earth: orchestrate the qemu bake
│   ├── setup.sh                   # image customization (EDIT HOSTNAME/IPADDR)
│   └── config.exp                 # earth: expect driver for the qemu console
└── flash/
    ├── build-flasher.sh           # Pi: build /boot/flasher.img via dracut
    └── 95netbsdflash/             # dracut module (name: "netbsdflash")
        ├── module-setup.sh
        └── flash.sh               # the pre-mount flasher hook
```
