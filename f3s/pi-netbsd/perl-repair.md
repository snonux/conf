# Pi1 Perl payload repair (jp3)

Prepared on 2026-10-24; superseded on 2026-10-25. **Never applied. Do not run
this 5.42.3 restoration against the current installation.** The owner approved
the reviewed restoration, but its mandatory fresh version guard stopped it
before any host writes.

## Current outcome: already repaired by package maintenance

Read-only checks on 2026-10-25 at approximately 15:55 UTC found
`perl-5.44.0nb2` installed. The unattended-upgrade log records downloading and
upgrading that package; its installed `+CONTENTS` timestamp is October 24 at
22:24:16 in the host's displayed time. This work did not initiate that upgrade.
The previous 5.42.3 artifact and absent-path guards no longer match the host.

Current native evidence:

- Normal operator and normal `doas` both print `v5.44.0`. `ldd` resolves
  `/usr/pkg/lib/perl5/5.44.0/aarch64-netbsd-thread-multi/CORE/libperl.so`, with
  no missing library. Config, Fcntl, File::Temp, Unicode::Collate and Digest::SHA
  load; Unicode comparison and a known SHA-256 digest exercise pass.
- `pkg_admin -q check perl-5.44.0nb2` exits zero with no warnings.
  `pkg_info -Fe /usr/pkg/bin/perl` now returns `perl-5.44.0nb2`, so the earlier
  ownership-lookup anomaly is no longer reproduced.
- Fresh existing-key SSH and `doas` work. Pi0 and Pi1's static HTTP endpoints
  return 200. `/usr/local/bin/f3sctl --local power status` exits zero and reports
  all seven f/r hosts reachable. No power action is executed.
- `/etc/rc.d/uptimed status` reports PID 4739 running. Its records and backup
  are nonempty. Ordinary Pi1 records uploads returned HTTP 204 at 14:17 and
  15:17 UTC on October 25; server records are 345 bytes with the latter upload's
  modification time. No upload was forced.
- The updater lock is absent. This work acquired no lock, staged no host
  files, performed no extraction or rollback, and changed no services,
  packages, authentication or power state.

The executable SHA-256 is
`14084026652ceb1aef603e194cf1f347c97d55d3b1bc764d369eb9493e0f6a0c`;
installed `+CONTENTS` SHA-256 is
`b48db2d7e3a1cd5dde942b6042ef30f9bb29ff432cc8f93722ac9d70780ca59a`.
Private evidence is retained at
`/home/paul/.local/state/conf-worktrees/jp3-approved-20261025/evidence/`.

Limitations: the original damage's cause remains unknown. The planned restore,
rollback and disposable flock exercise were not run; these results establish
the current runtime and ordinary upload health, not execution of the historical
repair. Initial `pgrep -x uptimed` did not match the process; the rc.d status and
process evidence did. An unauthenticated CGI request returned 401, and an initial
guessed f3sctl executable path did not exist; the corrected installed local
read-only command passed. Those failed probes are retained in the evidence.

The rest of this document preserves the reviewed historical plan and its
5.42.3 evidence. It is not a current execution instruction. Pi0 received no
changes.

## Observed state

Both Pis run NetBSD 11.0/aarch64 with `perl-5.42.3`. Their executable and installed
package manifest hashes match. Pi0 prints `v5.42.3` and its complete Perl package
passes `pkg_admin -q check perl-5.42.3`. Pi1 cannot load `libperl.so`.

Pi1's full package check reports **2,178 missing files and one checksum failure**:
`/usr/pkg/lib/perl5/5.42.0/Unicode/Collate/Locale/ln.pl` is zero bytes, versus
487 bytes in the package. The architecture directory, modules, utilities and
manual pages are among the missing files. The executable's RPATH is correct;
adding a global loader path or copying only the library would leave the package
incomplete. The cause of the damage is unconfirmed.

The root filesystem has 13 GiB available. Early-boot `sdmmc0` errors also occur
on healthy Pi0; they do not establish the cause of this damage or prove current
storage health. Stop the repair on new read/write errors.

Pi1's `pkg_info -Fe /usr/pkg/bin/perl` lookup also fails although its package
metadata exists. This procedure restores payloads against that existing
metadata; it does not claim to repair the separate file-ownership index.

## Verified source and prepared repair

The exact official package is still available from the
[2026Q2 repository](https://cdn.NetBSD.org/pub/pkgsrc/packages/NetBSD/aarch64/11.0_2026Q2/All/perl-5.42.3.tgz).
The moving 11.0 repository now contains a newer Perl version; do not substitute it.

| Artifact | SHA-256 |
| --- | --- |
| Official `perl-5.42.3.tgz` | `1d72bb21e8a16f48d09df952ca69ffa263d5702e8e4d0fe03537c001b136e7c3` |
| Official and both installed `+CONTENTS` | `10f244ed22df91383925bb3c4495c8b437bb63810742e945ba553ba4b162463a` |
| Existing executable and official `bin/perl` | `c8ea67cb47180e9fb6beac0267683949657511c02b0ad6d493d9a1057ea32a62` |
| Official library and healthy Pi0 library | `89103b32cd20bcb8d0d1a8f17ebcb7f55f10316e1eb85dd7835e0b891483fc31` |
| Prepared `perl-5.42.3-pi1-repair.tar.gz` | `05c8df20948d70697cf6a5a09dad034c87f6b1702daf35d1677ead61cb34f11d` |

Controller artifacts and private evidence are in
`/home/paul/.local/state/conf-worktrees/jp3-perl-20261024/evidence/`.
The repair archive has exactly 2,179 regular files, all below `lib/perl5/`
relative to `/usr/pkg`. It contains the missing files plus the one corrupt file;
healthy payloads and package metadata are excluded. Selected hard links are
materialized as regular files with identical bytes and modes. There are no
symlinks, absolute member paths, parent traversal or install scripts. `libperl.so`
is the last member, so it becomes available after the modules are restored.

All 2,859 checksum-recorded official file payloads pass their recorded checksums,
including three resolved hard links. The selected repair payloads were checked
again against the original archive. Pi1's native `tar -tzf -` accepted the repair
stream and returned the exact member list/order without extracting or saving it.

Read-only native guards confirm all 2,178 missing destinations are absent and
are not dangling symlinks; the replacement is a regular zero-byte file.
The ten existing ancestor directories are root-owned without group/world write;
202 additional directories are required. Full guard output and exact manifests
are retained beside the archives.

Native `pkg_add -n -U` was also examined: it prints a planned package deletion,
then exits 1. **It is not a passing replacement preview.** Upstream
[`start_replacing`](https://github.com/NetBSD/pkgsrc/blob/trunk/pkgtools/pkg_install/files/add/perform.c)
initializes its return value to -1 and leaves it unchanged in fake mode, which
is consistent with this result; that is not a proof about the exact installed
tool's source. No package installation or deletion was executed. The proposed
repair uses only verified payload restoration, without package scripts.

## Proposed bounded application

1. Obtain approval for Pi1 only, the exact repair archive above, its private
   backup and the checks below. Retain a working existing-key admin connection.
   Revalidate host identity, package/executable/manifest hashes, the complete
   damage list and archive hash. Stop on any changed destination, unexpected
   symlink, wrong owner/mode, different package version or new I/O error.
2. Coordinate against package maintenance using the existing
   `/var/run/unattended-upgrade.lock` directory protocol. If the lock exists,
   wait for its owner; never steal or remove another job's lock. Confirm no
   other package writer is active before acquiring the lock. Hold it for the
   repair and immediate verification window and release only the lock this
   operation owns. The updater treats locks older than two hours as stale;
   keep this window bounded below that lifetime. Observe the next ordinary
   upload after releasing the lock rather than extending the lock for that wait.
3. Create a unique root-only directory under `/var/backups/jp3-perl.*` and stage
   the repair there with mode 0600. Verify its hash and exact member list again
   using native tools. Save `ln.pl` with its owner, mode and contents. Save the
   pre-change integrity report, the 2,178 absent-file paths, the exact 202
   absent-directory paths, and package inventory/metadata hashes for comparison.
   Stop if a complete verified rollback record cannot be made.
4. Recheck every existing ancestor is a real root-owned directory without
   group/world write. Create only the recorded absent directories, root:wheel
   0755. With umask 022, extract the verified regular-file-only repair archive
   into `/usr/pkg` using native `tar -xzpf`. Restore archive modes/ownership;
   do not recursively chmod/chown an existing tree or run package hooks.
   Verify every restored payload against `repair-members.json`, including
   `libperl.so`, before treating extraction as successful.
5. Run the immediate acceptance checks below while the maintenance lock is held.
   Retain the private backup after success. Remove only this operation's staging
   files and release its lock, then observe the next ordinary upload. Record
   native results and publish the scoped runbook status only within the approved
   publication scope.

## Historical required live acceptance (restoration not run)

- As both the normal operator and through normal `doas`, Perl prints `v5.42.3`.
  Native `ldd` resolves the expected library and contains no `not found` entry;
  `ldd`'s exit status alone is insufficient (it returned zero while broken).
- The complete `pkg_admin -q check perl-5.42.3` exits zero **without warnings**.
  Run actual module exercises using `Config`, `Fcntl`, `File::Temp`,
  `Unicode::Collate` and `Digest::SHA`. A disposable flock test must exclude a
  second process until the first releases its lock. Keep it separate from the
  f3s power-job locks; execute no power or mute commands.
- The Perl version, executable, package manifest, package inventory and unrelated
  healthy payloads remain unchanged. Restored file modes/owners match the
  verified package. Report the pre-existing ownership-index anomaly separately.
- A fresh normal SSH/doas connection works. Pi1's static HTTP service, f3sctl
  read-only status path, uptimed process and next ordinary records upload work;
  Pi0 keeps serving. No service restart, cron edit, forced upload or power test
  is part of acceptance. A failed check is not completion.

## Rollback

While retaining the lock, remove only the exact newly restored files whose
hashes still match this repair's payloads; stop for inspection if one changed.
Restore the backed-up zero-byte `ln.pl` with its saved ownership/mode. Remove
only the recorded newly created directories, deepest first with `rmdir`, so a
nonempty or changed directory is preserved. Recheck the original package
inventory, metadata and damage report; this deliberately returns Perl to its
previous broken state. Leave unrelated files and all service/SSH configuration
untouched. Release only the owned maintenance lock and keep the backup/evidence.
Do not delete the package, restore a global package database or reboot as a
rollback shortcut.

References: [NetBSD package integrity checks](https://man.netbsd.org/pkg_admin.1),
[package information](https://man.netbsd.org/pkg_info.1),
[package replacement options](https://man.netbsd.org/pkg_add.1), and
[ELF library lookup](https://www.netbsd.org/docs/elf.html).
