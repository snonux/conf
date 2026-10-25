# FreeBSD DTail package revision and f3 canary (mp3)

Preparation only. Production package installation, source publication, repository
publication and service actions require explicit approval. The earlier f3 binary
repair approval did not include a package upgrade. Other 3n3 hosts remain pending.

## Candidate and preparation evidence

The native candidate is **dtail-4.3.2-ng_1.pkg**, FreeBSD:15:amd64, from exact
DTail commit `673c237bf495d898b8a5d92a964ef556b02a564d`:

`SHA256 5a66d78126b2b1f46ac14e4b3ec211ec0d5bd15356263185765cbf9ab11ed97f`

Controller archive verification requires Python 3.14 with Zstandard support
(verified with 3.14.3 on earth); the native f0 scripts require no Python.

The six static/nozstd binaries were built with Go 1.26.8 and an explicit amd64
v1 profile. A clean detached source clone excludes the original checkout's WIP.
Go embeds the actual VCS revision and clean status. The installed
`/usr/local/share/doc/dtail/build-info.json` records full source/toolchain details
and payload SHA256 values. Native pkg annotations also identify the source.
`.zst` log support remains disabled, as in the existing static FreeBSD builds.

The FreeBSD package revision is explicit: `DTAIL_FREEBSD_REVISION=1`. Native pkg
2.8.4 confirms `4.3.2-ng_1` sorts after the installed `4.3.2-ng`, and revision 2
after revision 1. Increment the revision for changed source, toolchain or packaged
assets while upstream keeps its version. Publication refuses an existing package
version, including a byte-identical re-upload; a label cannot silently gain new
contents. A changed source ref alone is not a release revision bump.

Preparation exercised actual Git and Go builds, original WIP exclusion, embedded
VCS data and negative input/output guards. The real f0 pkg tool assembled the
archive as an ordinary user in a private temporary directory. All packaged file
bytes, root:wheel ownership declarations and modes were checked against the local
bundle, including actual tar member types/ownership/modes and consistent full and
compact metadata. Native pkg stores numeric payload UID/GID 65534 for
[reproducibility](https://github.com/freebsd/pkg/blob/2.8.4/libpkg/packing.c#L216);
root/wheel names remain required, as do exact 0755/0644 modes. The verifier
checks those native IDs and names independently of the manifest declarations.
Executables/helper/rc are 0755; sample/provenance are 0644. No private keys,
runtime/cache directories or live host configuration are package members.

An unprivileged native integration test used a separate `pkg -r` filesystem and
package database with scripts/RC handling disabled. It exercised a legacy package
upgrade, explicit config backup/restore and `pkg check -s`. It also used a separate
scratch repository to prove that an unrelated package survives catalog generation,
duplicate versions cannot overwrite artifacts/catalogs, and tampered/symlinked
bundle inputs fail. The production f0 package version and binary hash stayed
unchanged. These are preparation tests, not a production f3 canary.

## Commands and configuration ownership

`make dtail-freebsd` now builds and assembles for review; it does **not** publish.
The stages can also be run separately, using a new bundle directory:

```sh
make -C packages dtail-freebsd-build \
  DTAIL_REF=673c237bf495d898b8a5d92a964ef556b02a564d \
  DTAIL_FREEBSD_REVISION=1 DTAIL_FREEBSD_BUNDLE=/path/to/new-bundle
make -C packages dtail-freebsd-package DTAIL_FREEBSD_BUNDLE=/path/to/new-bundle
python3 packages/scripts/verify-dtail-freebsd.py /path/to/new-bundle \
  /path/to/new-bundle/package/dtail-4.3.2-ng_1.pkg
```

Build refuses an existing output directory. Assembly uses only an exclusive
unprivileged f0 scratch directory, retrieves the artifact and verifies it. The
separate `dtail-freebsd-publish` stage verifies again, refuses same-version
replacement and regenerates the catalog over the entire existing repository.
It is a production command and must not run during preparation.

The package ships **dtail.json.sample only**. Gonf owns the actual per-host
`/usr/local/etc/dserver/dtail.json`; a generic package cannot choose every host's
LAN binding. FreeBSD's [configuration-file guidance](https://docs.freebsd.org/en/books/porters-handbook/book/#plist-config)
explains why ordinary packaged config files can be removed/replaced. The native
[packing-list interface](https://man.freebsd.org/cgi/man.cgi?query=pkg-create&sektion=8)
provides the explicit root/wheel file attributes used here.

The old package owns the live, unmarked config file. **Shipping only a sample does
not by itself preserve that old file on the first upgrade.** Back up and explicitly
restore it with RC auto-handling disabled before starting dserver. The native
isolated-root migration test verifies that procedure. Future revisions do not
own the live file. The package includes the exact already-reviewed 3n3 safe helper
and startup hook; its sample uses persistent identity and log-only permissions.

## Proposed approved canary — f3 only

1. Refresh installed package/version and the current working binary hash, config,
   protected cache metadata, service enabled/running state and LAN listener.
   Verify the currently served persistent public fingerprint through existing
   trusted administrator access. Preserve two administrator sessions and capture
   application/monitoring/guest baselines. Stop on unexpected drift.
2. Back up the actual working binary, root-controlled config/helper/rc and package
   inventory into a new root-private directory on f3. In particular, retain the
   known-working temporary binary, not just the old repository package. Do not
   read the daemon-owned persistent private key as root or transfer it off-host.
3. Stage this exact reviewed artifact privately on f3. Check its SHA256, pkg
   metadata/source revision, ABI, newer version and absence of dependencies or
   installation scripts. No repository refresh or unrelated package action.
4. Stop only dserver. Redirect service command stdin/output away from the SSH
   transport and bound observation; a timeout requires read-only state inspection,
   not blindly retrying the operation. Install the local artifact using
   `pkg -o HANDLE_RC_SCRIPTS=false -o RUN_SCRIPTS=false add ARTIFACT`.
5. Restore the saved host-specific config as root:wheel 0644 and verify its exact
   saved hash before any start. Preserve its LAN-only binding, log-only rules and
   `/var/db/dserver/ssh_host_key`. Keep protected root:dserver 0750/0640 cache
   ownership. For native startup acceptance, rename the stopped protected runtime
   into fresh root-only quarantine on the same filesystem, then let the installed
   startup hook recreate it. Do not use the vulnerable old helper or generate a key.
6. Start only dserver and require the same public fingerprint, expected LAN
   listener, protected cache recreation and real authentication. Repeat current
   dcat/dgrep reads with stable same-user native log-prefix oracles, readable
   nonlog denials, wrong-pin rejection, denied daemon cache writes and safe refreshes.
   Capture only bytes/hashes/structural results. No system SSH/PF/VM/reboot changes.
7. Require installed version `4.3.2-ng_1`, expected source provenance, root ownership
   and successful `pkg check -s dtail`; the working executable must now match the
   package, ending the temporary override. Verify normal admin access and unchanged
   workload/monitoring/guest health. Record readiness explicitly: the earlier
   binary repair had an inconclusive initial startup read before stable checks.
8. Publish only the approved conf source/acceptance to GitHub and Forgejo. Keep
   root-private backups/quarantine; agree retention before deleting key/log material.

On failure, stop the scoped operation and inspect state; do not repeat an uncertain
install/start. Retain identity, protected cache, restricted configuration and all
backups. If needed restore the actual pre-canary known-working binary and hardened
config/helper/rc, recording any resulting package checksum drift as an explicit
rollback override. Never edit pkg's checksums to hide drift or reinstall the old
vulnerable package blindly. If safe working state cannot be established, leave
DTail stopped and report it; no other service or guest recovery is implied.

## Normal repository publication gate

Normal repository publication is **not part of the proposed f3-only canary**.
Read-only checks on f0/f1/f2 show `dtail-4.3.2-ng`, ownership of the old live config
path and no package locks. Publishing revision 1 into their normal catalog can
therefore expose them to an automatic upgrade before config-preserving migration.
First prepare/approve those hosts' 3n3 cache/identity migration and explicit package
upgrades with backup/restore, or a separately reviewed package-hold strategy. Do
not silently lock packages or upgrade the other hosts to make publication possible.

Task mp3 remains open for its production steps; local artifact preparation does
not establish installed-package or fleet acceptance. Task lp3 in project dtail
owns generic protocol compatibility; its completion is not needed to prepare or
canary this known newer server source. No signing-key/VM migration is performed.

Private build/native test receipts and the reviewable artifact are under
`~/.local/state/conf-worktrees/mp3-freebsd-package-20261025/evidence/`.
