# Preserve f3's dserver identity before cache hardening

Deployment update: the owner approved the f3-only repair; see
[dserver-f3-acceptance-20261025.md](dserver-f3-acceptance-20261025.md) for actual
native results and limitations. The preparation evidence below remains historical.

Task 3n3 preparation, refreshed for kp3 on 2026-10-25. Nothing in this document authorizes a
production write, key copy, service action, source publication or reboot. The
separate seven-BSD proposal still excludes f3. Keep that distinction until the
f3-specific procedure and runtime acceptance have been approved.

## Current evidence

Strict administrator SSH reached f3. Its existing dserver is enabled and serving
TCP 2222 with wildcard binding. The configured key is
`/var/run/dserver/cache/ssh_host_key`; `/var/db/dserver` is absent. The key is a
regular, single-link 1675-byte file, mode 0600, uid 1002 (`dserver`), gid 0. Both
runtime directories are daemon-owned 0755. `/var`, `/var/run` and `/var/db` are
root-owned 0755 directories. No key-cache updater was running during inspection.
These are observations to refresh before an approved operation, not permanent
assumptions about inode numbers or process IDs.

The external public SSH handshake and a loopback handshake obtained through the
already trusted administrator SSH connection agree on this served RSA identity:

`2048 SHA256:D45UZ7edwGAk4FCtxdxAuIFpwrYnb+AG0QQU2c/9E+Y`

This is the refreshed 25 October identity after the owner-authorized normal
shutdown/startup. The earlier volatile identity did not survive. Refresh it
again immediately before any approved preservation; no private key was copied.

The controller had no pre-existing global dserver pin for f3. A private test pin
was therefore derived from the public handshake only after comparing it with the
trusted administrator-channel loopback fingerprint. Global `known_hosts` was not
changed. No private-key file was read, copied or transferred during preparation.

The destination is the existing BSD layout: directory `/var/db/dserver`, mode
0700 owned by `dserver:dserver`, containing `ssh_host_key`, mode 0600 owned by
`dserver:dserver`. This persistent identity is separate from the hardened,
root-owned authorization cache under `/var/run/dserver/cache`.

## Acceptance blocker found before migration

The current dcat client authenticated to f3 using the verified private pin and
existing SSH-agent keys, but returned exit 0 with **zero log payload** for the
nonempty `/var/log/messages` (about 1 MiB). An explicit dgrep query did the same.
Holding stdin open and changing plain/quiet modes did not establish a real log
read. The dserver account could read and hash the first 4096 bytes of that OS
file through the normal administrator channel, so the test did not rely on an
unreadable or empty source. Client and native server report DTail 4.3.2-ng /
protocol 4.1; those labels alone do not prove identical builds or compatibility.
Task **kp3** now reproduces the old-server/current-client acknowledgement
compatibility failure in a controlled matrix, corroborated by installed f3
handler disassembly. Its exact source revision remains unproven. See
[dserver-f3-read-investigation.md](dserver-f3-read-investigation.md). No log payload
was retained or printed.

Do not accept exit status alone or claim successful DTail access. After the
normal startup the user authorization cache is also absent; the old rc hook
never refreshes it. No successful pre-repair read is currently possible. The
separate kp3 proposal explicitly requests a coordinated f3-only binary repair
and this safe cache/identity migration, with pinned actual-content acceptance
afterward. Neither document authorizes that operation or a key rotation alone.

## Proposed approved procedure

1. Refresh the service/configuration/listener states, account IDs, parent-path
   metadata, updater schedule and public identity above. Establish two trusted
   administrator sessions. Record the actual pre-repair read/auth failure; only
   the separately approved coordinated kp3 repair supersedes the earlier
   successful-read prerequisite. Record the initial enabled/running state and
   keep the ordinary system SSH path.
2. With explicit approval, create fresh root-private staging and backup
   directories on f3. Back up only root-controlled config, helper and rc files.
   Retain old logs in quarantine. Never send the private host key to the laptop,
   Git, command output or a general-purpose temporary directory.
3. Recheck the volatile source with native lstat semantics: each source path
   component must be a real directory, and the key a non-symlink regular file
   owned by dserver, mode 0600, with one link. Record its metadata before and
   after the read. **Read as dserver, not root**: for example, a bounded
   `su -m dserver -c 'exec /bin/cat /var/run/dserver/cache/ssh_host_key'` writes
   through the administrator shell's redirection into a newly created 0600
   file inside the root-private staging directory. FreeBSD's native `timeout`
   must bound a stalled read. No root cat/cp, recursive repair or chown through
   the daemon-controlled source tree is permitted.
4. Parse only the staged copy using native `ssh-keygen -y -P '' -f STAGED_KEY`
   with its public result redirected privately. Compare its public SHA256
   fingerprint with the trusted served fingerprint above. Reject an invalid,
   encrypted, changed or mismatching key; never generate a replacement. A
   failed preparation leaves the existing daemon and key path untouched.
5. Require the persistent destination to remain absent, including absence of a
   dangling symlink. Create `/var/db/dserver` exclusively as a root-owned 0700
   directory beneath the verified root-controlled parent; do not adopt or
   repair an unexpected existing tree. Install the already validated staged
   key there as `dserver:dserver` 0600. While the directory is still private
   to root, verify the installed key's fingerprint, type, ownership, mode and
   single link. Hand off the new directory to `dserver:dserver` 0700 last.
   After handoff, inspect private-key contents only as dserver. Recheck that
   dserver can read it and derives the identical public fingerprint. The live
   configuration still points to the original key throughout this preparation.
6. Only after preservation succeeds, use the separately approved dserver-only
   stop. Confirm the updater is idle, then quarantine the entire old runtime by
   a same-filesystem rename under a fresh root-only parent. Do not recursively
   repair the old tree or delete it. Apply the current-base named
   `freebsd_dtail_config` task for f3 through normal Gonf/doas. Its configuration
   must point to `/var/db/dserver/ssh_host_key`, bind `192.168.1.133:2222`, and
   restrict reads to logs. The runtime/cache must be root:dserver 0750, with
   authorization files root:dserver 0640. Do not deploy a cluster aggregate or
   change packages, sshd, PF, guests, mounts, CARP or routing.
7. Verify the served fingerprint is unchanged using the private verified pin.
   Prove a real OS log payload matches a same-user OS read, and prove that DTail
   denies `/etc/fstab` and an unrelated temporary file. Verify authentication
   as paul, only the expected listening address, native 0640 cache reads, denied
   daemon cache writes, two cache refreshes, health checks and normal SSH. The
   dserver startup helper must preserve the existing persistent fingerprint;
   no reboot is needed or authorized.

The authoritative hardening procedure is [dserver-cache-hardening.md](dserver-cache-hardening.md).
Use an integration based on published `1edefe8` or its approved successor so the
accepted sm3/co3 guards and private Gogios writers are preserved. The old
seven-host source base must not overwrite later approved changes.

## Failure handling and remaining verification

Before configuration/service changes, a failed staged-copy or fingerprint check
must leave the existing service/key untouched. Retain the failed root-private
staging for inspection; do not remove an unexpected tree. Once a key directory
has been handed to the daemon, do not recursively chmod/chown or delete through
it as root.

If post-hardening acceptance fails, stop only dserver within the approved
rollback scope, keep the preserved identity and quarantine, and follow the main
runbook's hardened rollback. Do not restore vulnerable cache permissions,
wildcard binding or unrestricted file access. The original staged key is already
a root-private rollback copy, so rollback need not read the daemon-owned key tree
as root. Agree retention before deleting any old key/log material.

Private preflight receipts are in
`~/.local/state/conf-worktrees/3n3-f3-identity-20261024/evidence/`.
Key staging/publication, service migration and production acceptance remain
**unperformed**. The kp3 diagnosis and coordinated repair proposal now explain
the successful-empty baseline and missing startup cache; native f3 repair
acceptance remains pending explicit approval. Laptop/phone WireGuard device packets
are not established by these LAN/admin checks.
