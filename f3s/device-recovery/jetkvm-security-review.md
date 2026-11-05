# JetKVM developer authentication review

**Evidence date: 2026-11-05.** Pinned system source is
[dc2b6d967fc16aa59b412ae306eb34f4c1ec3d0b](https://github.com/jetkvm/rv1106-system/tree/dc2b6d967fc16aa59b412ae306eb34f4c1ec3d0b),
release/v0.2.8. The installed canary banner identifies Dropbear2025.88.

The [build Makefile](https://github.com/jetkvm/rv1106-system/blob/dc2b6d967fc16aa59b412ae306eb34f4c1ec3d0b/sysdrv/tools/board/dropbear/Makefile)
uses the bundled Dropbear2025.88 archive. Its exact source establishes:

- `src/default_options.h` enables server password/publickey authentication
  and disables PAM. The reviewed build adds no explicit password override.
- `src/svr-auth.c:svr_authinitialise` advertises the compiled password method
  unless `noauthpass` is set, independently of the account's password field.
  Therefore password advertisement is compatible with a locked root account.
- `src/common-session.c:fill_passwd` calls getpwnam, then getspnam when the
  password field is a shadow reference; it does not directly select var/etc.
- `src/svr-authpasswd.c` rejects invalid/locked crypt salts, empty stored
  passwords and unequal hashes. No password was submitted to test this.
- `src/svr-runopts.c` supports `-s` to disable all password logins and `-g`
  for root password logins. JetKVM's launcher uses neither.

Pinned rootfs/etc/passwd refers to shadow, and rootfs/etc/shadow locks root.
The separate rootfs/var/etc/shadow contains a hash-class field. Neither hash
nor credential value was printed or committed. `sysdrv/Makefile` copies
rootfs/* after unpacking its skeleton, with separate etc/var directories;
the skeleton has no etc→var symlink. Reviewed bootstrap/init/app launch and
board post-processing scripts do not redirect that account database. This
bounded source review cannot rule out other build/installed modifications.

Root reviewed v3's bounded client-publickey-only inspection with an initial
effective-account classification gate. That first command emitted only root
shadow classification, passwd-source classification and file metadata.
Fixtures proved no field/hash output for locked, empty and hash cases.

**Installed v3 outcome:** root shadow is locked, passwd uses shadow_reference,
and both are regular files. Thus the installed account condition agrees with
the reviewed password-rejection path despite the advertised method. No
password or empty-password attempt occurred. Only then did read-only SSH
inspection run. Root and /userdata are mounted ext4 rw; eth0 remains
192.168.1.241/24, default gateway192.168.1.1. Init uses /etc/init.d/rcS,
with S20linkmount and S21appinit present. No hook or config was installed.
The command returned1 because optional guessed paths /etc/rc.d,
/usr/bin/dropbear.sh and /etc/rc.local were absent; actual launcher contents
were not captured. Do not call its installed hook behavior verified.

Fresh ID/USB/video checks passed; cleanup disabled developer mode, drained
the original enable request to terminal, verified TCP22 closed and restored
the exact original empty authorized_keys. An independent TCP22 probe also
returned connection refused. Private evidence lives under
`~/.local/state/conf/s14-jetkvm-inspection-v3-20261105/` (0700, files0600).

The official app/UI exposes developer mode and key enrollment, not daemon
authentication flags. No supported official setting was found to enforce
server-side key-only without changing the launcher, process, build or account
database. Those changes exceed current inspection scope. Client password
disable flags apply only to that client; the effective locked root account
is the relevant installed protection, not disappearance of advertisement.

Pinned [RkLunch.sh](https://github.com/jetkvm/rv1106-system/blob/dc2b6d967fc16aa59b412ae306eb34f4c1ec3d0b/project/app/jetkvm/RkLunch.sh)
explicitly executes /userdata/init.d/S??* before post_chk starts app/network.
Subsequent v4 installed inspection verified that exact hook and its ordering,
with /userdata mounted before the launcher. v5 captured the installed stop and
udhcpc scripts and actual process roles. Both inspections restored developer
access exactly. The [recovery design](jetkvm/README.md) records those later
facts and the remaining network-owner/watchdog questions. No hook was installed
and no reboot recovery proof is claimed.
