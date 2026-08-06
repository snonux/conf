# pi0 NetBSD 11 recovery handoff

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

## Status at handoff time

Task `ww0` is upgrading pi0 from NetBSD 10.1 to 11.0. The kernel, modules,
direct-boot files, and userland sets are already NetBSD 11.0. The hardened
same-session `sysupgrade sets` to orderly-reboot sequence worked. The upgrade
is incomplete because `etcupdate` replaced local account configuration before
`postinstall`, configuration validation, cron restoration, and the final
reboot were completed.

pi0 is currently powered on and reachable at `192.168.1.125` and
`192.168.2.203`. OpenSSH responds on port 22 and presents the expected host
key, but rejects every currently offered user key. The repository SSH config
selects port 2 for the pi aliases, so all direct tests must explicitly use
`-p 22`:

```sh
ssh -p 22 paul@pi0.lan.buetow.org
```

`dserver` on port 2222 and bozohttpd remain intentionally stopped from the
maintenance window. pi1 is healthy on NetBSD 11.0 and continues to serve the
public static sites. Do not modify or reboot pi1 during pi0 recovery.

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

The likely mistake was replacing `authorized_keys` with only `id_rsa.pub`
instead of preserving/appending the pre-existing `paul@earth` key. Install
both agent keys on the next offline pass. Do not replace one with the other.

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

## Required next offline repair (completed)

The following procedure was completed successfully and is retained only as a
recovery record. Do not repeat it on the recovered card.

The user must power pi0 off and reinsert its microSD card into earth. Then:

1. Re-identify the card by the exact geometry above and confirm it is not
   mounted.
2. Reverify the existing full-image checksum; a second 32 GB image is optional
   because the verified pre-repair image already exists.
3. Build `authorized_keys` from all public keys currently returned by:

   ```sh
   ssh-add -L
   ```

   Include `~/.ssh/id_rsa.pub`, deduplicate complete key lines, and verify that
   the resulting file contains both fingerprints listed above.
4. Boot a disposable formal NetBSD 11 recovery image under QEMU with:
   - the recovery image as the first virtio disk;
   - the physical card as the second virtio disk (`ld5`, `dk3` root);
   - a separate read-only FAT payload as the third disk (`ld6e`), containing
     the two-key `authorized_keys` and a short repair script.
5. In the guest:

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

6. Require both expected key fingerprints in output, `AUTH_REPAIR_OK`, a clean
   post-write filesystem check, and a clean QEMU halt.
7. On earth, flush the card and safely power off the reader before removal.
8. Reinstall and boot pi0, then test both identities explicitly on port 22.

## Online completion after SSH recovery (completed)

Once a fresh login succeeds, do not immediately reboot. First require:

```sh
id
doas -n id
uname -a
```

`id` must include `wheel`, and `doas -n id` must report uid 0. Then resume task
`ww0` from its annotations:

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

Update the tracked upgrade runbook so future `etcupdate` work verifies
`id paul`, explicit wheel membership, `authorized_keys`, and a fresh
noninteractive doas login before releasing the last privileged session.

## Repository and task state at handoff time

- Active task: `ww0` — pi0 NetBSD 11 base upgrade with gated reboots.
- Current repository worktree was clean before adding this handoff.
- Latest relevant commits:
  - `ad96a0a` — NetBSD 11 golden-image recovery workflow.
  - `744fefe` — hardened same-session sets-to-reboot procedure.
  - `82d9a3a` — pi1 NetBSD 11 upgrade completion record.
- Nothing from this recovery work has been pushed.
