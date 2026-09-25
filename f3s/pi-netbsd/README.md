# NetBSD on pi0/pi1

pi0 and pi1 (Raspberry Pi 3B+) run NetBSD 11.0 GENERIC64 evbarm/aarch64 and
serve the static sites behind relayd. This directory holds the procedure that
replaced Rocky with NetBSD over SSH, without pulling the SD card. Use it to
rebuild a node or provision a replacement, never as an in-place update of a
live NetBSD node.

| | pi0 | pi1 |
|---|---|---|
| LAN (`mue0`) | 192.168.1.125 | 192.168.1.126 |
| WireGuard (`tun0`) | 192.168.2.203 | 192.168.2.204 |
| Hostname | pi0.lan.buetow.org | pi1.lan.buetow.org |
| Role | static content source of truth | pulls pi0's docroot hourly (paul crontab) |

Gateway/DNS 192.168.1.1. 909 MiB RAM. SD `/dev/mmcblk0` (30 GB).

Live services: bozohttpd (vhosts `f3s.buetow.org`, `www.f3s.buetow.org`,
`standby.f3s.buetow.org`, `snonux.foo`, `www.snonux.foo`), WireGuard, NPF
(TCP 22/80/2222 on `mue0`, all on `tun0`, default block), uptimed,
dserver (DTail, TCP 2222), hourly goprecords upload and dserver key-cache
cron, unattended upgrades and vuln audit via gonf (`pis_netbsd`).

Package repos:

```
https://cdn.NetBSD.org/pub/pkgsrc/packages/NetBSD/aarch64/11.0/All
https://pkgrepo.f3s.buetow.org/netbsd/11.0/packages/aarch64/     # dtail, f3sctl
```

Build DTail for NetBSD natively on pi0 (`make dtail-netbsd` in
`packages/`, default 11.0). `pkg_create` must match the target release: never
build a 10.1 package on an 11.0 host.

## Why the firmware initramfs

- `kexec` is compiled out of the Rocky RPi kernel (`6.1.31-v8.1.el9.altarch`,
  `kexec_load` returns `ENOSYS`, no `CONFIG_KEXEC_FILE`).
- The systemd shutdown pivot (`/run/initramfs/shutdown`) never fires on these
  boxes.
- What works: the Pi firmware loads a dracut initramfs from a one-shot
  `config.txt`; its pre-mount hook flashes the card from RAM. The Rocky
  kernel has ext4, vfat, mmc, sdhci and tmpfs built in, so the initramfs needs
  no modules.

The hook deletes `config.txt` and the trigger file before doing anything
else, so any failure after it starts leaves a card that boots normally.

## Prerequisites

Workstation (`earth`, x86_64, runs the aarch64 guest under TCG):

```sh
sudo dnf install -y qemu-system-aarch64 qemu-img edk2-aarch64 expect
```

plus passwordless sudo and `~/.ssh/id_rsa.pub` (baked into the image).

Pi: SSH as `paul` with passwordless sudo, `dracut`, vfat `/boot`, ~500 MB
free on `/`. Keep the twin up so the static backend stays served.

## Stage A: bake the golden image

```sh
cd f3s/pi-netbsd/bake
./bake-golden.sh pi1        # or pi0
```

Downloads `arm64.img.gz` (NetBSD-11.0 evbarm-aarch64 gzimg, SHA-512 pinned,
then `gzip -t`; a bad cache fails closed), renders `HOSTNAME`/`IPADDR` into
`setup.sh`, boots it in qemu, drives the console with `config.exp`, verifies,
syncs, powers off and writes `netbsd-piN-golden.img.gz`. The random backup
password is printed and saved to `piN-cred.txt`; the SSH key is the real
login.

`setup.sh` sets `sshd=YES`, hostname, `ifconfig_mue0`, `defaultroute`,
`dhcpcd=NO`, `resolv.conf`, user `paul` in `wheel` with key and argon2id
password (root too), pubkey+password sshd, and an `rc.local` fallback that
puts the static IP on the first ethernet interface if it isn't `mue0`.

Only update the pinned digest after verifying a formal replacement release.

Handled in the scripts, but know them: add virtio-rng or NetBSD stalls on
entropy; run `expect` with `LC_ALL=C`; blind-drive the console and judge by
log markers; `sync` and wait for a clean poweroff or FFS corrupts
`rc.conf`/`pwd.db`.

## Stage B: flash on the Pi

```sh
scp -r f3s/pi-netbsd paul@piN.lan.buetow.org:
scp netbsd-piN-golden.img.gz paul@piN.lan.buetow.org:/home/paul/   # gzip -t + sha256sum both sides
ssh paul@piN.lan.buetow.org 'sudo /home/paul/pi-netbsd/flash/build-flasher.sh'   # -> /boot/flasher.img (~38 MB)
```

The hook finds `/home/paul/netbsd-*-golden.img.gz` (or under `/root/`).

Dry run (non-destructive, ~90 s, comes back as Rocky):

```sh
ssh paul@piN.lan.buetow.org '
  sudo rm -f /boot/flash-evidence.txt
  printf "initramfs flasher.img followkernel\n" | sudo tee /boot/config.txt
  printf "dryrun\n" | sudo tee /boot/netbsd-flash-mode
  sync; sudo systemctl reboot'
ssh paul@piN.lan.buetow.org 'sudo cat /boot/flash-evidence.txt'
#   FLASHER-RAN mode=dryrun up=...
#   DRYRUN-RESULT bytes=<size> rc=0
```

`bytes=` must equal `gzip -l` of the image on earth, `rc=0`, and
`config.txt` gone. Don't continue otherwise.

Real flash (destructive; `dd` of ~1.6 GB, then first boot and root resize):
same command with `real` instead of `dryrun`.

## Verify

```sh
ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null paul@192.168.1.12N '
  uname -a; hostname; id; ifconfig mue0
  netstat -rn -f inet | grep default
  host cdn.netbsd.org; df -h /'
```

Host keys change after the flash.

When rebuilding an existing node, restore its own SSH host keys and
WireGuard identity; never copy the twin's. Then:

1. Restore peer auth explicitly: pi1's sync key in pi0's
   `authorized_keys`; the pull is pinned to
   `IdentitiesOnly=yes -i /home/paul/.ssh/id_ed25519`.
2. Check helper-script `PATH`s include `/usr/pkg/bin` (the goprecords
   uploader needs curl from there).
3. Run one full pull on pi1 and one manual goprecords upload.
4. Failover gate: stop one node's bozohttpd (or reboot it) and check all five
   vhosts return 200 through both relayd addresses (23.88.35.144,
   46.23.94.99). `/fotos/` and `/scifi/` return 404 on snonux vhosts by
   design.

## Recovery

- Before the real flash nothing is lost. A malformed `flasher.img` that never
  runs the hook boot-loops the Pi: pull the SD and delete `config.txt`. The
  dry run exists to rule this out.
- After the flash, going back means reflashing Rocky. Keep a Rocky image.
- File-level backups: `fishfinger:/home/rex/backups/pi1-20260803`,
  `fishfinger:/home/rex/backups/pi0-20260806T0712`.

Base release upgrades (`sysupgrade`) are manual. Lessons from the 11.0
upgrade:

- The firmware loads `/boot/netbsd.img`; `sysupgrade kernel` updates
  `/netbsd`. Keep both at the same release. Don't touch `config.txt`,
  `cmdline.txt`, don't install `bootaa64.efi`.
- `sysupgrade sets` replaces sshd under the running daemon; new SSH logins
  then fail until reboot. Run sets and the reboot from one HUP-proof root
  shell in the existing session:

  ```sh
  doas sh -c 'trap "" HUP; /usr/pkg/sbin/sysupgrade sets </dev/null >/var/log/sysupgrade-11.0-sets.log 2>&1 && exec /sbin/shutdown -r now'
  ```

- After `etcupdate`, keep the root session open and check `id paul` (uid
  1000, group `users`, in `wheel`), `/etc/group` has `wheel:*:0:root,paul`,
  `~paul/.ssh` is 700 and `authorized_keys` 600 owned `paul:users`, then a
  fresh `ssh -o BatchMode=yes -o IdentitiesOnly=yes ... 'id; doas -n id'`
  from a second shell.

Full records: [`docs/archive/f3s/pi-netbsd/`](../../docs/archive/f3s/pi-netbsd/).

## Troubleshooting

| Symptom | Cause |
|---|---|
| Dry run back in ~20 s, no `flash-evidence.txt` | hook didn't run: check `config.txt` line, `/boot/flasher.img`, trigger file says exactly `dryrun` |
| `dracut module 'netbsdflash' cannot be found` | use `--add netbsdflash` (dir name without `95`) |
| `bytes=` mismatch or `rc!=0` | bad staged image; recheck scp and checksum |
| NetBSD up but unreachable | NIC not `mue0`; `rc.local` fallback should still set the IP; attach console |
| qemu bake hangs | see Stage A notes |

Cleanup on a Pi left on Rocky after dry runs:

```sh
sudo rm -f /boot/config.txt /boot/netbsd-flash-mode /boot/flash-evidence.txt \
           /boot/flasher.img /home/paul/netbsd-pi0-golden.img.gz \
           /home/paul/netbsd-pi1-golden.img.gz
```

Inspect any `/boot/initramfs-*.img.orig` by hand before restoring it; no
wildcard `mv`.

## Files

```
bake/bake-golden.sh        earth: qemu bake orchestrator
bake/setup.sh              image customization template (rendered per node)
bake/config.exp            expect console driver
flash/build-flasher.sh     Pi: builds /boot/flasher.img
flash/95netbsdflash/       dracut module "netbsdflash" (module-setup.sh, flash.sh hook)
```
