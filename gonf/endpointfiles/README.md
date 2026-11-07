# Endpoint-file recovery during task924

This helper guards only the closed profile/path lists in `assets/guard`.
It does not change network addresses. Prepare installs an independent minute
watchdog and OS boot hook; a pending transaction prevents replacing the helper.
Linux/NetBSD require their native `flock`; FreeBSD/OpenBSD require the already
installed Perl Fcntl module. A persistent lock inode and inherited descriptor
serialize operations; service and timeout children close that descriptor.

Arm snapshots bytes, permissions and ownership before applying an existing
owner's candidate. One profile may be pending per host. Expiry restores its
files and previously active service; boot restores files before consumers,
then the watchdog completes runtime recovery. Failed recovery retains the
journal for retry. DNS stops FTL before restoring its TOML to avoid shutdown
writes overwriting the snapshot. Accepted/restored journals stay private under
the platform state directory. Neither an archived journal nor parser success
proves service health; fresh traffic checks precede confirmation.

Operational tasks use strict `GONF_ENDPOINT_TRIAL` JSON, for example
`{"profile":"hosts","seconds":900}`. The sequence is prepare, arm, apply via
the existing owner, verify, confirm. Exercise real expiry and boot restoration
before fleet rollout. For service profiles leave at least135 seconds of the
original lease for bounded restart and confirmation. Do not renew an existing
lease or edit its deadline to make a failed trial pass.

Hosts normal reapply is permitted after an accepted hosts transaction. Other
service profiles require a fresh lease for helper promotion. Canonical recipe
sources must be adopted alongside acceptance so ordinary reapply retains the
accepted values. The helper refuses unknown roles, custom controller settings,
symlinks and special owned files; it never prints configuration contents.

Offline checks run the real dispatcher/merger with temporary paths and mocked
OS service boundaries. They cover exact expiry/boot restoration, failed copy
and runtime retry, FTL shutdown writes, modes, invalid inputs and real kernel
lock contention. Native service, SSH, DNS and reboot checks remain separate.
