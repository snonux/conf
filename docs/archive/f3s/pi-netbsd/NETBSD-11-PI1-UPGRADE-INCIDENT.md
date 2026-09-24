# pi1 NetBSD 11.0 upgrade incident and procedure record

This records the staging completed on 2026-08-03 and the upgrade completed on
2026-08-05. pi1 now runs the 11.0 kernel, modules, userland, and boot files.
The commands below remain as historical procedure and recovery evidence; do
not rerun them on either upgraded host. The current dual-node state is in
[`NETBSD-11-DUAL-NODE-ACCEPTANCE.md`](NETBSD-11-DUAL-NODE-ACCEPTANCE.md).

## Verified release inputs

Only the formal `NetBSD-11.0/evbarm-aarch64` release was used:

```text
https://cdn.netbsd.org/pub/NetBSD/NetBSD-11.0/evbarm-aarch64/
```

The 11.0 `INSTALL.txt`, `LAST_MINUTE`, release announcement/notes, and the
installed `sysupgrade(8)` manual were reviewed. `LAST_MINUTE` says there is
nothing pertinent. The evbarm INSTALL recommends `sysupgrade`, explicitly
calls out updating the applicable DTB and `bootaa64.efi`, and gives the safe
major-upgrade order: fetch, kernel, modules, reboot, sets, etcupdate,
postinstall, reboot.

The signed release hash manifest is
`https://cdn.netbsd.org/pub/NetBSD/security/hashes/NetBSD-11.0_hashes.asc`.
`gpgv` reported a good signature from RSA key `89261E17F5EF49FF`, the NetBSD
Security Officer 2019 key obtained from the formal NetBSD CDN. SHA512 values
for every fetched set/kernel and for `arm64.img.gz` were checked against that
signed manifest. The image hash is:

```text
50c8597cd83b73d12a466e67c1ecc49fb1969941171644b15a7fe377b0f482ccb190b3aa38b043eba3f5e7b4d6e38cf0e7d3c6fd5251813af869b49beda2ee80
```

## Current boot path and comparison

pi1 is a Raspberry Pi 3 Model B Plus (BCM2837). It boots directly through the
Raspberry Pi firmware, **not** U-Boot or EFI. `/boot/config.txt` selects
`kernel=/netbsd.img`, `os_prefix=dtb/broadcom/`, and 64-bit mode. Consequently
the selected DTB is `dtb/broadcom/bcm2837-rpi-3-b-plus.dtb`.
`/boot/EFI/BOOT/bootaa64.efi` exists in the image but is not on this boot path.

The 10.1 and 11.0 `arm64.img` EFI partitions have identical file lists.
`config.txt`, `cmdline.txt`, the BCM2837 DTB, and all Raspberry Pi firmware
files are byte-identical. Only `netbsd.img`, `bootaa64.efi`, and several
unrelated RK3399 DTBs differ. There are therefore no local modifications to
`config.txt` or `cmdline.txt` relative to the release image. Their contents
remain:

```text
#
upstream_kernel=1
#
arm_64bit=1
os_prefix=dtb/broadcom/
cmdline=../../cmdline.txt
kernel=/netbsd.img
kernel_address=0x200000
enable_uart=1
force_turbo=0
```

```text
root=NAME=netbsd-root console=fb
```

Release-managed 11.0 boot files are staged under
`/var/cache/netbsd-11.0-boot`; preserved local `config.txt` and `cmdline.txt`
copies are in the same directory. Fetched sets and `netbsd-GENERIC64.gz` are
under `/var/cache/sysupgrade`. Do not copy staged files blindly: only the
BCM2837 B+ DTB and direct-boot `netbsd.img` apply to pi1; EFI is retained for
reference/recovery and the firmware is byte-identical.

## Historical install procedure

This is the exact command record from the completed maintenance window, not an
active runbook. A future upgrade must be derived from its target release's
formal guidance and may reuse only the failure-prevention lessons documented
here. The historical procedure first preserved the live boot partition:

```sh
set -e
stamp=$(date +%Y%m%dT%H%M%S)
doas mkdir -p "/var/backups/boot-$stamp"
doas cp -Rp /boot/. "/var/backups/boot-$stamp/"
doas cp -p /netbsd "/netbsd.pre-11-$stamp"
```

Install the applicable boot files while leaving the local configuration
untouched. This is not transactional on FAT, which is why physical SD recovery
is a prerequisite. Verify each temporary copy before renaming it:

```sh
set -e
doas cp -p /var/cache/netbsd-11.0-boot/dtb/broadcom/bcm2837-rpi-3-b-plus.dtb /boot/dtb/broadcom/bcm2837-rpi-3-b-plus.dtb.new
doas cmp /var/cache/netbsd-11.0-boot/dtb/broadcom/bcm2837-rpi-3-b-plus.dtb /boot/dtb/broadcom/bcm2837-rpi-3-b-plus.dtb.new
doas mv /boot/dtb/broadcom/bcm2837-rpi-3-b-plus.dtb.new /boot/dtb/broadcom/bcm2837-rpi-3-b-plus.dtb
doas cp -p /var/cache/netbsd-11.0-boot/netbsd.img /boot/netbsd.img.new
doas cmp /var/cache/netbsd-11.0-boot/netbsd.img /boot/netbsd.img.new
doas mv /boot/netbsd.img.new /boot/netbsd.img
doas sync
doas /usr/pkg/sbin/sysupgrade kernel
doas /usr/pkg/sbin/sysupgrade modules
```

`sysupgrade kernel` updates the conventional `/netbsd`; the separately staged
`netbsd.img` is the kernel that the Pi firmware actually loads. Both must be
kept at the same release. Do **not** replace `config.txt` or `cmdline.txt`. Do
not install `bootaa64.efi` for this direct-firmware boot path. Reboot once and
confirm `uname -r` is 11.0.

Installing the userland sets replaces libraries and `sshd` while the old
daemons are still running. On pi1, `sysupgrade sets` succeeded but every new
SSH connection then closed before sending a banner; restoring access required
a hard reboot. Avoid depending on a new login on pi0 by running the sets
installation and reboot from one hangup-resistant root shell in the existing
authenticated SSH session. Keep the client connected for observation, but do
not attempt another SSH login after this command starts:

```sh
doas sh -c 'trap "" HUP; /usr/pkg/sbin/sysupgrade sets </dev/null >/var/log/sysupgrade-11.0-sets.log 2>&1 && exec /sbin/shutdown -r now'
```

The reboot remains conditional on `sysupgrade sets` returning success.
`shutdown -r now` performs the orderly shutdown and filesystem synchronization,
then detaches from the SSH session while the reboot proceeds. If `sysupgrade
sets` fails, the command returns without requesting a reboot; inspect
`/var/log/sysupgrade-11.0-sets.log` from the existing session. If that session
was lost, do not assume either success or reboot: userland may be partial and
fresh SSH may still be unavailable, so use the documented console or SD-card
recovery path.

After the upgraded node returns, establish a fresh SSH connection and repeat the full
kernel, network, route, DNS, root-filesystem, module, and boot-diagnostic gate.
Only then continue with configuration migration:

```sh
set -e
doas /usr/pkg/sbin/sysupgrade etcupdate
```

Keep that authenticated privileged session open after `etcupdate`. Before
continuing, verify the active account databases and SSH key file, not just the
text files from which the databases are built:

```sh
id paul
grep -Fqx 'wheel:*:0:root,paul' /etc/group
test -s /home/paul/.ssh/authorized_keys
test "$(stat -f '%Su:%Sg %Lp' /home/paul/.ssh)" = 'paul:users 700'
test "$(stat -f '%Su:%Sg %Lp' /home/paul/.ssh/authorized_keys)" = 'paul:users 600'
ssh-keygen -lf /home/paul/.ssh/authorized_keys
```

`id paul` must report uid 1000, primary group `users`, and supplementary group
`wheel`. From a second client shell, require a fresh key-authenticated login
and noninteractive privilege escalation on explicit port 22 before releasing
the original session:

```sh
recovery_key=~/.ssh/id_rsa
ssh -p 22 -o BatchMode=yes -o PreferredAuthentications=publickey \
    -o IdentitiesOnly=yes -i "$recovery_key" \
    paul@piN.lan.buetow.org 'id; doas -n id'
```

The fresh `id` must still include `wheel`, and `doas -n id` must report uid 0.
Set `recovery_key` to each identity that must remain usable and repeat the
command so every required recovery key is proven independently.
Do not continue if either gate fails: `etcupdate` can replace `group`,
`master.passwd`, and the generated password databases, so a correct wheel line
or `authorized_keys` file alone does not prove that sshd recognizes the user.
Only after this fresh-session gate passes, finish postinstall:

```sh
set -e
doas /usr/pkg/sbin/sysupgrade postinstall
doas sync
```

Before the final reboot, verify `sysupgrade postinstall` succeeded. After it,
verify `uname -a`, `npfctl show`, all five rc.d services (`bozohttpd`,
`wireguard`, `uptimed`, `npf`, and `dserver`), `curl -fsI http://localhost/`,
and public service through both frontends while pi0 remains online.

## Restore and recovery

`/netbsd.old` is an SHA512-identical copy of the running 10.1 `/netbsd`.
Note that `sysupgrade kernel` itself backs up to `/onetbsd`, not
`/netbsd.old`. A kernel-only rollback is valid only before `sysupgrade
modules` has run. In that narrow case, restore the stamped root kernel and the
entire saved boot tree (the direct-boot `netbsd.img` is not interchangeable
with `/netbsd`):

```sh
doas cp -p /netbsd.pre-11-YYYYMMDDTHHMMSS /netbsd
doas cp -Rp /var/backups/boot-YYYYMMDDTHHMMSS/. /boot/
doas sync
```

`/netbsd.old` is the additional pre-staged fallback; the timestamped copy is
the maintenance-window rollback source. Once 11.0 modules or userland sets
have been installed, do not attempt a kernel-only rollback: restore the
complete fishfinger backup or reinstall/restore the SD card so kernel,
modules, and userland remain matched.

For a failure before multi-user mode, use HDMI/keyboard (the console is
`console=fb`) or remove the SD card and restore the saved EFI partition/files
from another system. `enable_uart=1` is set, but the kernel command line does
not currently select a serial console, so serial must not be the only recovery
plan. A durable full backup exists on fishfinger at
`/home/rex/backups/pi1-20260803`, and pi0 keeps the web service available.
Physical SD recovery was explicitly unavailable/accepted until approximately
2026-08-05, which was the recovery constraint during the upgrade.

## Completion

The kernel/modules/direct-boot files and all userland sets were installed on
2026-08-03. The first kernel reboot gate passed. The old sshd then stopped
serving an SSH banner after the userland extraction, so work paused without a
downgrade until pi1 could be rebooted and recovered.

On 2026-08-05, fresh SSH and the full kernel/network/root/module gate passed.
`sysupgrade etcupdate` installed the 11.0 base configuration while retaining
the local network, firewall, SSH, account, crontab, and custom rc.d settings.
`sysupgrade postinstall` passed after applying its requested 11.0 `MAKEDEV`
update. The paused content-sync, goprecords, and dserver key-cache crons were
restored from their saved originals.

After the final controlled reboot, pi1 reported NetBSD 11.0, matching modules,
a healthy writable FFS root, mue0 at `192.168.1.126`, the expected default
route and DNS, valid sshd and NPF configurations, the required rcorder, and
running bozohttpd, WireGuard, uptimed, NPF, and dserver services. Local and
public HTTP checks passed, both frontends reported pi0 and pi1 up, and pi0
remained untouched on NetBSD 10.1 during this pi1 maintenance window. Pi0 was
subsequently upgraded; see the dual-node acceptance record linked above.

## Historical staged configuration and capacity

`sysutils/sysupgrade` 1.5nb12 is installed. Its configuration is:

```text
RELEASEDIR="https://cdn.netbsd.org/pub/NetBSD/NetBSD-11.0/evbarm-aarch64/"
ARCHIVE_EXTENSION=tar.xz
KERNEL=AUTO
SETS=AUTO
POSTINSTALL_AUTOFIX="obsolete"
AUTOCLEAN=no
```

The fetch cache was 219 MiB and the boot staging tree was 23 MiB. After the
pre-maintenance staging phase, the root filesystem had 17 GiB free and `/boot`
had 47 MiB free; services remained running until the approved upgrade window.
