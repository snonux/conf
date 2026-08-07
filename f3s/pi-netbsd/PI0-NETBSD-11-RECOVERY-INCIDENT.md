# pi0 NetBSD 11 recovery incident

> **Completed 2026-08-06.** This document is retained as the incident and
> recovery record. The offline-repair instructions below are historical and
> must not be rerun on the recovered card.

## Final status

pi0 now runs a complete NetBSD 11.0 kernel, module, userland, boot-file, and
configuration upgrade. Both required `paul` RSA identities log in on port 22,
the active account database includes `paul` in `wheel`, and `doas -n id`
returns uid 0. `dserver` and its account/group, the local `rc.local`, and the
saved root cron jobs were restored after `etcupdate` removed them.

`sysupgrade postinstall` passed after its requested `obsolete` and `makedev`
fixes. The final controlled reboot passed the network, DNS, writable FFS,
module, boot-file, sshd, NPF, rcorder, service, DTail, cron, local HTTP,
frontend, and public HTTP gates. pi1 remained healthy and serving throughout.

A temporary root password and root-password SSH policy were used only to
recover the missing account database entries. After the final reboot gate, the
original root hash was restored, the temporary SSH block was removed, sshd was
reloaded with `PermitRootLogin prohibit-password`, and the temporary credential
was verified rejected. No temporary plaintext credential is retained in this
repository or the recovery workspace.

## Incident state when recovery began

Task `ww0` was upgrading pi0 from NetBSD 10.1 to 11.0. The kernel, modules,
direct-boot files, and userland sets were already NetBSD 11.0. The hardened
same-session `sysupgrade sets` to orderly-reboot sequence had worked. The
upgrade was incomplete because `etcupdate` replaced local account
configuration before `postinstall`, configuration validation, cron
restoration, and the final reboot were completed.

At that point pi0 was powered on and reachable at `192.168.1.125` and
`192.168.2.203`. OpenSSH responded on port 22 and presented the expected host
key, but rejected every offered user key. The repository SSH config selected
port 2 for the pi aliases, so direct tests used explicit port 22:

```sh
ssh -p 22 paul@pi0.lan.buetow.org
```

`dserver` on port 2222 and bozohttpd were intentionally stopped for the
maintenance window. Pi1 remained healthy on NetBSD 11.0 and served the public
static sites throughout recovery.

## Original failure

`sysupgrade etcupdate` removed `paul` from `wheel`:

```text
wheel:*:0:root
```

That made the existing doas policy unusable:

```text
permit nopass :wheel
```

No privileged SSH/control session survived. Root SSH keys were not configured,
the root password is unknown, `/boot` is not writable by `paul`, and no safe
remote privilege-recovery mechanism was available.

## Backups and evidence

The durable file-level backup created before the pi0 upgrade is:

```text
fishfinger:/home/rex/backups/pi0-20260806T0712
```

The complete physical microSD image taken before the first offline repair is:

```text
/home/paul/Downloads/pi0-recovery-20260806/pi0-microsd-before-wheel-fix.img
```

Properties:

```text
size:   31,914,983,424 bytes
SHA256: 62d91457b14c879ed213e94f8f4498a6c3ddd1e7b6fe07dd7d1462d692f86231
```

The checksum is stored in the adjacent `SHA256SUMS` and has been reverified.
Recovery logs are in the same directory:

```text
repair.log
auth-repair.log
```

The card's known layout is:

```text
device:  /dev/sda when inserted in earth's current USB reader
sectors: 62,333,952 x 512 bytes
sda1:    80 MiB FAT boot partition
sda2:    29.6 GiB NetBSD FFS root partition
```

Always re-identify the removable device by size and partition layout before
writing. Never assume it remains `/dev/sda`.

## First offline repair: wheel membership

The powered-off card was attached as a secondary disk to a disposable formal
NetBSD 11.0 evbarm-aarch64 QEMU guest. NetBSD identified the physical card as
`ld5`, with boot wedge `dk2` and FFS root wedge `dk3`.

The recovery process:

1. Ran `fsck_ffs -p /dev/rdk3`.
2. Mounted `/dev/dk3` at `/mnt`.
3. Saved `/etc/group.before-wheel-repair-20260806`.
4. Atomically restored:

   ```text
   wheel:*:0:root,paul
   ```

5. Synced and unmounted the filesystem.
6. Ran `fsck_ffs -n /dev/rdk3`, which reported the filesystem clean.
7. Cleanly halted QEMU, flushed the host block device, and powered off the USB
   reader.

After boot, SSH authentication still failed, so wheel/doas could not yet be
tested online.

## Second offline repair: SSH key mistake

The second repair installed only this public key as
`/home/paul/.ssh/authorized_keys`:

```text
SHA256:rzvwz15PkOvKZWtqfwMuwoug+zNTSILwFnXpejrn/Zk paul@computer
```

It came from earth's `~/.ssh/id_rsa.pub`. Ownership and permissions were
verified offline:

```text
/home/paul/.ssh                  paul:users 0700
/home/paul/.ssh/authorized_keys  paul:users 0600
```

The wheel line was still correct and FFS was clean. Nevertheless, after boot,
sshd rejected that key. The SSH agent actually contains two RSA identities:

```text
SHA256:rzvwz15PkOvKZWtqfwMuwoug+zNTSILwFnXpejrn/Zk paul@computer
SHA256:fBoselDdawO4H5F+gk0zOFqNqWZSDR2SVjbtFTSBXEU paul@earth
```

The mistake was replacing `authorized_keys` with only `id_rsa.pub` instead of
preserving/appending the pre-existing `paul@earth` key. The subsequent offline
repair installed both agent keys without replacing one with the other.

## Important failed approaches

- Linux's UFS support is read-only here and cannot safely repair NetBSD FFS.
- Attaching the physical card and its raw backup to the same NetBSD guest
  caused duplicate GPT/wedge UUIDs. NetBSD configured the backup wedges and
  refused the physical-card wedges. Do not attach both simultaneously.
- A long repair command exceeded the serial console's canonical line limit.
  Use a short command that runs a script from a separate read-only FAT payload.
- A QEMU `fat:ro:` drive appears in NetBSD as an MBR disk whose FAT partition
  is `ld6e`, not `ld6c`:

  ```sh
  mount -t msdos /dev/ld6e /key
  ```

- Writes reported against read-only `ld6e` were attempts to update the virtual
  FAT payload and did not affect the physical card. Successful physical-card
  writes were followed by clean sync, unmount, and FFS verification.

## Third offline repair: both SSH keys (completed)

The following procedure was completed successfully and is retained only as a
recovery record. Do not repeat it on the recovered card.

The completed repair used this sequence:

1. Re-identified the card by the exact geometry above and confirmed it was not
   mounted.
2. Reverified the existing full-image checksum; the verified pre-repair image
   made a second 32 GB image unnecessary.
3. Built `authorized_keys` from all public keys returned at the time by:

   ```sh
   ssh-add -L
   ```

   The payload included `~/.ssh/id_rsa.pub`, deduplicated complete key lines,
   and contained both fingerprints listed above.
4. Booted a disposable formal NetBSD 11 recovery image under QEMU with:
   - the recovery image as the first virtio disk;
   - the physical card as the second virtio disk (`ld5`, `dk3` root);
   - a separate read-only FAT payload as the third disk (`ld6e`), containing
     the two-key `authorized_keys` and a short repair script.
5. Ran the following in the guest:

   ```sh
   fsck_ffs -p /dev/rdk3
   mount /dev/dk3 /mnt
   mkdir -p /mnt/home/paul/.ssh
   cp /key/authorized_keys /mnt/home/paul/.ssh/authorized_keys
   chown 1000:100 /mnt/home/paul/.ssh /mnt/home/paul/.ssh/authorized_keys
   chmod 0700 /mnt/home/paul/.ssh
   chmod 0600 /mnt/home/paul/.ssh/authorized_keys
   grep -Fqx 'wheel:*:0:root,paul' /mnt/etc/group
   ssh-keygen -lf /mnt/home/paul/.ssh/authorized_keys
   sync
   umount /mnt
   fsck_ffs -n /dev/rdk3
   halt -p
   ```

6. Required both expected key fingerprints in output, `AUTH_REPAIR_OK`, a clean
   post-write filesystem check, and a clean QEMU halt.
7. Flushed the card on earth and safely powered off the reader before removal.
8. Reinstalled and booted pi0, then tested both identities explicitly on port
   22.

## Online completion after SSH recovery

After a fresh login succeeded, the recovery first required:

```sh
id
doas -n id
uname -a
```

`id` included `wheel`, and `doas -n id` reported uid 0. Task `ww0` then resumed
with these gates:

1. Verify `mue0=192.168.1.125`, default route, DNS, writable FFS root, matching
   11.0 modules, and clean boot diagnostics.
2. Compare and deliberately restore local configuration damaged by
   `etcupdate`, using the timestamped pi0 backups documented on `ww0`.
3. Preserve and verify `rc.conf`, hosts, resolv.conf, fstab, npf.conf,
   sshd_config, rc.local, custom rc.d scripts, accounts/groups, and crontabs.
4. Complete the pending MAKEDEV/obsolete fixes and `sysupgrade postinstall`.
5. Validate `sshd -t`, NPF, rcorder, host identity/keys, and every service.
6. Restore paused cron, publishing, content, and bozohttpd activity.
7. Perform the final controlled reboot and repeat all gates.
8. Keep pi1 healthy and serving public traffic throughout.

This incident established the future `etcupdate` gate now recorded in the
rebuild and skills documentation: verify `id paul`, explicit wheel membership,
`authorized_keys`, and a fresh noninteractive doas login before releasing the
last privileged session.

## NetBSD 11 package and service migration (completed)

The operational work for task `xw0` completed the remaining package migration
on 2026-08-06. The
durable command results and operational transcript are retained in the task's
annotations (`ask info xw0`); the summary below records the stable outcome.
The official pkgsrc repository is now the aarch64/11.0 repository. A full pkgin
upgrade refreshed all 31 installed third-party packages for 11.0 and installed
the required GCC 12 runtime dependency; a subsequent dry run had nothing left
to do. The resulting count is 33: the 31 refreshed pkgsrc packages, newly
installed GCC 12, and the separately installed custom DTail package.
`pkg_admin check` passed all 7,286 files in those 33 packages. Explicit
version checks also passed for doas, rsync, curl, wireguard-go,
wireguard-tools, autoconf, automake, libtool, and pkg-config.

Uptimed 0.4.7 was rebuilt natively with the refreshed autoconf, automake,
libtool, and pkg-config toolchain. Both uptime databases were backed up and
restored byte-for-byte with their `root:wheel` ownership, and the Linux,
NetBSD 10.1, and NetBSD 11.0 history remained visible in `uprecords`.

The already-published artifact at
`https://pkgrepo.f3s.buetow.org/netbsd/11.0/packages/aarch64/dtail-4.3.2ng.tgz`
was downloaded and installed with `pkg_add`. Its size was 44,260,503 bytes and
its SHA256 was
`26519dcbce90244bd3dd00e7e3a907e41d73e2cd7a2da53cf89ade9506594737`.
The package's `+BUILD_INFO` identified `MACHINE_ARCH=aarch64`, `OPSYS=NetBSD`,
and `OS_VERSION=11.0`; post-install `pkg_info dtail` reported
`dtail-4.3.2ng`. The persistent dserver host key retained SHA256
`eb02c2a65e3d153bfb845cd69b27efb6aec399fad84d9890245fff90d54d0b84`.
Service start and the final controlled reboot both recreated the paul key
cache; TCP 2222 listened and an external `dcat` read of `/etc/fstab` passed.

The controlled reboot ran at 21:19 UTC. Post-reboot checks included:

- fresh SSH, `id paul`, and `doas -n id`;
- NPF validation, active filtering, and rules for TCP 22, 80, and 2222;
- recent WireGuard handshakes and successful pings to both frontends;
- HTTP 200 for all five vhosts (`f3s`, `www.f3s`, `standby.f3s`, `snonux`,
  and `www.snonux`), `/fotos/`, `/scifi/`, and all 12 `index.html` files found
  under the document root;
- running bozohttpd, WireGuard, uptimed, dserver, cron, NPF, and sshd;
- successful goprecords script execution using the root cron PATH, both
  expected root cron entries, no paul cron or pull script on source pi0, and
  the expected `sync-from-pi0.sh` entry on pi1;
- the LAN address, default route, DNS lookup, persistent DTail key and
  recreated cache, retained uptime history, external `dcat`, and public HTTP
  200 for both domains plus `/fotos/` and `/scifi/`.

The negative package check was `pkgin -n full-upgrade`, which reported
`nothing to do`; `pkg_admin check` found no damaged package files. pi1 was
accessed read-only throughout this task: its NetBSD 11.0 release, local HTTP
200, pull cron, and three-hour uptime were observed after the pi0 reboot. No
command changed or rebooted pi1.
