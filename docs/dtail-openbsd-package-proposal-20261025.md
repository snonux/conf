# OpenBSD DTail compatibility package proposal

Prepared, not installed: both frontends authenticate the current laptop client
but return zero log bytes. Their identical installed executable has the historical
acknowledgement-completion handler already diagnosed under kp3. Hardening its
configuration alone cannot repair this dispatch behavior. This proposal changes
only DTail on fishfinger and blowfish and requires owner approval before installation.

## Exact candidate

- Package: `dtail-4.3.2p1-ng.tgz`, distinct OpenBSD p1 revision, existing `ng` flavor
  and `local/dtail` package path.
- SHA256: `bbe76c1e1a9fe7678ca9123c8045f71c3832dd3cf3cc4e71b110007f94ba95bf`.
- dserver SHA256: `e292371c78314462e5fb2b10390d800f9cb7a9ea18247e0e76c140c2eb5e4e79`.
- Source: exact reviewed `673c237bf495d898b8a5d92a964ef556b02a564d`.
- Native Go 1.26.8, OpenBSD/amd64 v1, CGO enabled; existing Zstandard support is
  retained. Every one of the six binaries embeds that revision and clean VCS state.

The native build ran unprivileged in an exclusive blowfish scratch directory,
with low priority, serial compilation and a 384 MiB Go memory target. Before build,
blowfish had 1056 MiB free and 99% idle CPU; observed free memory remained above
800 MiB. No system toolchain/package install, service change or signer access
occurred. The private scratch Go archive was downloaded from go.dev and verified
against its official SHA256
`338be23e51b38df2fba23b6b523b879dc90aa0408080b36cb68afc21169fd0a9`.

The package contains six binaries, the reviewed protected cache helper/startup
hook, a sample config and source/build provenance. All payload hashes and native
packing-list checksums agree. Payload owners are root:wheel, executable modes
0755 and sample/provenance modes 0644, matching Gonf's resulting ownership.
There are no dependencies, account changes or install/uninstall scripts. No live
config, private key or runtime/cache is a package member. The package is unsigned;
this proposal uses the exact checksum-verified local artifact with per-command
`-D unsigned`, without changing global signature policy or accessing signing keys.

The native dry run recognizes the upgrade from `dtail-4.3.2-ng` to
`dtail-4.3.2p1-ng`. Restrict `PKG_PATH` to the private artifact directory: the
unrestricted default preview also considered quirks, while the restricted preview
proposes only DTail. Do not run the auto-publishing `make dtail-openbsd` target.

## Proposed runtime scope

1. Fishfinger canary, then blowfish only after acceptance. Refresh trusted live
   fingerprints and native strict previews; preserve two administrative sessions.
   Back up the actual old binaries, root-controlled config/helper/rc and package
   metadata privately. Never transfer or rotate either persistent DTail key.
2. Verify the staged exact package digest and contents, stop only dserver, and
   install only the explicit local package using `PKG_PATH=PRIVATE_ARTIFACT_DIR`
   and `pkg_add -r -D unsigned PRIVATE_ARTIFACT`. The old package owns its live
   config: explicitly restore the saved config byte for byte before startup.
   No repository publication or unrelated package/OS update belongs to this scope.
3. Perform the already approved unsafe-cache quarantine and scoped
   `frontends_d_tail_hardening`. Preserve each key; bind to its WG address and
   keep log-only permissions. Publish only the config-only DTail WG monitoring
   changes, after verifying the complete Gogios configuration comparison.
4. Verify proper installed package revision and executable/provenance hashes,
   persistent public identity, WG-only listener, protected cache, same-user native
   log-prefix matches from pinned current dcat/dgrep, nonlog policy denials,
   wrong-pin rejection, safe refreshes and startup recreation. Observe starts
   independently; an observer timeout never triggers an automatic restart.
5. Require frontend WG monitoring health before wm3 closes public TCP 2222.
   No PF, system SSH authentication, reboot, ZNC or other frontend service action
   is included. SSH-forwarded client tests are not claimed as a direct device-WG
   packet-path test.

## Durability and failure handling

This is a proper distinct package installation, not a silent binary override.
The new package owns only a config sample; Gonf owns the live per-host config.
Its higher revision prevents a normal upgrade from replacing it with the old
same-flavor version. Normal repository publication remains separate and unperformed.

Keep private backups and quarantines. On failure, inspect before further action;
retain the protected helper/cache, restricted binding/log policy and unchanged
identity. Do not reinstall the vulnerable old package wholesale. If restoring an
actual old binary is needed to recover prior behavior, explicitly record the
resulting package-checksum drift and retain the hardened config/helper/rc; that is
not successful current-client acceptance. If no safe useful state is established,
leave only dserver stopped and report the failure. Do not conceal it by reverting
cache ownership, wildcard binding or broad file access.

The final artifact and native build/verification receipts are under
`~/.local/state/conf-worktrees/3n3-fleet-20261025/ops/openbsd-build/`.
The earlier root:bin metadata draft was superseded before any installation or
approval request; only the final SHA256 above is proposed.
