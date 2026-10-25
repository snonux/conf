# f3 DTail reads: diagnosis and proposed repair

Task kp3, 2026-10-25. **Diagnosis and preparation only.** No production binary,
configuration, key, cache, service or package changes have been performed. This
proposal requires explicit approval for publication and the f3-only operation.
It does not authorize the other hosts in task 3n3.

## Two independent failures

Before shutdown on 24 October, the existing current dcat and dgrep authenticated
to f3, dispatched a read of the nonempty, daemon-readable `/var/log/messages`,
and returned exit 0 with zero log bytes. A working f0 control returned 269,518
bytes and its first 4096 bytes matched the same-user native OS read. Full f0/f3
configuration objects differed only in HostKeyFile. Version labels on both
servers and the client say 4.3.2-ng / protocol 4.1; those labels obscure different
builds.

After the owner-authorized normal shutdown and startup, f3's volatile host key
changed and `paul.authorized_keys` was absent from `/var/run/dserver/cache`.
The user's source authorized_keys remained a regular single-link file, owned by
paul and mode 0600, beneath real home/.ssh directories. The installed rc start
hook creates directories but never refreshes the cache; the installed helper is
scheduled by daily periodic only. Today's legacy diagnostic client therefore
fails authentication; its 211 bytes of WARN output and exit 0 are **not** log
payload. This startup failure is separate from yesterday's authenticated-empty
read. Do not invoke the vulnerable old root cache helper to work around it.

The new served identity was checked through both the external public handshake
and loopback over already trusted administrator SSH. They agreed:

`SHA256:D45UZ7edwGAk4FCtxdxAuIFpwrYnb+AG0QQU2c/9E+Y`

This is an observation to refresh immediately before an approved operation,
not permission to accept a later change automatically. Yesterday's fingerprint
is stale. No private key has been read/copied; the global known_hosts is unchanged.

## Controlled reproduction and causal evidence

An isolated root-owned local fixture ran the actual existing client and the
retained installed r0 server, plus diagnostic-only builds of unchanged DTail
source 849951b. Each case used generated keys, a private pin, a loopback port,
a nonroot server, a root:daemon-group 0640 authorization cache and a synthetic
1,200,000-byte log. Every server and temporary fixture resource was cleaned up.

| Client | Server | Exit | Returned bytes | Exact expected payload |
|---|---|---:|---:|---|
| Historical 849951b | Historical 849951b | 0 | 1,200,000 | yes |
| Existing current | Historical 849951b | 0 | 0 | **no** |
| Historical 849951b | Installed r0 | 0 | 1,200,000 | yes |
| Existing current | Installed r0 | 0 | 1,200,000 | yes |

A separate fixture with the actual current client/r0 binary also rejected an
incorrect host pin with exit 1. The historical CLI lacks `-known-hosts-path`;
its supported integration-test mode selected `./known_hosts` in an isolated
fixture directory. It used synthetic fixture keys; the diagnostic f3 attempt
used the existing explicit client key without changing its metadata. HOME,
global trust and production auth caches were not changed. The legacy client is
an investigation tool, not the proposed repair.

Current client source at 673c237 queues `.ack framing escaped-v1` and
`.ack framing control-v1` before the read in
`internal/clients/handlers/basehandler.go:requestFraming`. Its legacy transport
also wraps reads in `.ack input begin` / `.ack input complete` in
`internal/clients/connectors/sessiontransport.go:sendLegacyCommands`.
Historical source 849951b treats `.ack` as an ordinary command:
`internal/server/handlers/serverhandler.go:handleAckUserCommand` calls command
completion after processing the acknowledgement. Completion can shut down an
idle session before the queued read is admitted. AUTHKEY and SESSION are excluded
from that shutdown rule; `.ack` is not. Ignoring an unknown acknowledgement's
body therefore does not make sending it harmless.

The installed f3 executable contains the historical handleAckUserCommand and
shouldShutdownOnCommandCompletion symbols. Disassembly shows handleAckCommand
followed by the completion callback in that handler. The working f0 executable
has the newer dispatcher. This corroborates the compatibility diagnosis on f3;
its exact Git revision is **not embedded or proven**, and it is not claimed to
be byte-identical to the historical diagnostic build. The controlled matrix
establishes this failure mechanism; direct native f3 content acceptance remains
unperformed.

Application follow-up **lp3** in project **dtail** owns a generic regression fix
and real compatibility/negative tests. No application source was changed under
this conf task. There is no formal dependency preventing conf diagnosis or a
separately approved operational repair.

## Exact proposed artifact

Both FreeBSD hosts report 15.1-RELEASE-p2 / amd64. Read-only native hashes and
local copies agree:

| Artifact | SHA256 |
|---|---|
| Existing f3 `/usr/local/bin/dserver` | `010f1f18777a4cfad63f616b16a16b0357a0375ce52f309fccb23bc14a7be888` |
| Working f0 `/usr/local/bin/dserver` | `3f146b14d293df15ff019817e30f6be4f3bbf88f2bc60e5ca3bd4683cb5f8573` |
| Existing laptop dcat | `ae4f404b69f66f522200dce86bc2ffaed4630f709b7ffec2d87dd5b514a2035c` |

The f0 artifact is FreeBSD/amd64 v1, CGO disabled, nozstd, built with Go 1.26.8
and x/crypto 0.57.0. f3 uses Go 1.25.9 and x/crypto 0.39.0. Neither installed
server embeds an exact source revision. This proposal reuses the tested f0
artifact, without rebuilding from a mutable application checkout. Native f3
acceptance still gates success.

On both hosts the executable is root:wheel 0755, regular and single-link, below
root-owned real 0755 `/usr`, `/usr/local`, `/usr/local/bin`. Both are registered
to package `dtail-4.3.2-ng`. The proposed f3-only binary replacement is therefore
a **temporary package-file override**, not a package upgrade or a provenance
claim from the identical package label. Preserve pkg metadata; record the
installed hash and override in the acceptance report. Do not change package
checksums to hide the override. A future package reinstall/update may overwrite
it: require a versioned package correction and repeat the actual-content check
before declaring that separate packaging work complete. No repository/package
publication or fleet package upgrade is authorized here.

## Proposed approved operation, f3 only

Use the already reviewed task 3n3 hardening core carried by this candidate,
based on published conf 1edefe8. The detailed identity/cache procedure is
[dserver-f3-identity-preservation.md](dserver-f3-identity-preservation.md) and
[dserver-cache-hardening.md](dserver-cache-hardening.md). This proposal changes
its ordering explicitly because startup authentication currently fails; it does
not require a successful read from the broken baseline before repairing it.

1. Refresh two strict administrator sessions, f3 native version/path metadata,
   existing binary hash, current public host identity, config, service enabled
   state and listener. Revalidate the f0 artifact hash and its earlier native
   content control. Stop on drift from the reviewed baseline. Do not adopt
   unexpected filesystem objects or accept an unverified new host pin. Record
   current authentication failure as the baseline, separately from yesterday's
   protocol failure. Capture cluster/monitoring health for comparison.
2. With explicit approval, create an exclusive root-private maintenance directory
   on f3 under a verified root-controlled parent. Back up the root-controlled
   executable, config, helper and rc files with metadata. Transfer only the
   public f0 executable over trusted administrator SSH into root-controlled
   staging, verify the exact SHA above and executable format, and keep it
   unavailable to the daemon until installation. No private key leaves f3.
3. Before stopping anything or replacing the binary, preserve the **currently
   served** volatile identity using the 3n3 read-as-dserver, bounded-read,
   metadata/fingerprint checks and exclusive persistent destination procedure.
   Keep the staged root-private rollback copy. A failed preservation leaves
   the original service and configured key untouched; never rotate/generate a
   key to get past a failed check.
4. Stop only dserver, confirm it stopped and the cache updater is idle, then
   quarantine the old runtime tree by same-filesystem rename beneath a fresh
   root-only parent, without following/chowning its contents. Never run the old
   vulnerable helper. Install the verified artifact as root:wheel 0755 using a
   fresh file and atomic rename within `/usr/local/bin`; recheck the final hash.
   The normal administrative SSH service remains available.
5. Apply only `freebsd_dtail_config` for f3 from this integrated candidate through
   normal Gonf/doas. This installs the reviewed safe root-owned helper, populates
   the protected authorization cache, installs the startup hook, sets persistent
   `/var/db/dserver/ssh_host_key`, binds only `192.168.1.133:2222`, restricts reads
   to logs and starts/restarts dserver. If an unchanged Service resource does
   not start the stopped daemon, issue one explicit approved dserver start.
   Do not run a cluster aggregate or alter packages/accounts/sshd/PF/mounts,
   guests, CARP, routing or other services. No reboot is required or authorized.
6. Run the acceptance below, retaining only structural results/hashes. Publish
   only the explicitly approved source candidate and its factual acceptance to
   GitHub and Forgejo. Other prepared 3n3 hosts remain pending their own approval.

## Acceptance and stop conditions

* The served public fingerprint must equal the refreshed, admin-attested initial
  fingerprint. Use the private verified pin; a deliberately wrong pin must fail.
* Authenticate as paul with the unchanged current client. Read the existing
  `/var/log/messages` with plain/quiet output captured only in RAM. Compare a
  nonempty first 4096-byte prefix with a native read as dserver both immediately
  before and after; require the two OS prefix hashes to agree and the client
  prefix to match. Require at least 4096 actual log bytes, with no diagnostic
  warning masquerading as payload. If rotation changes the oracle, re-establish
  a stable existing log before proceeding; do not call a mismatched run a pass.
  Exercise current dgrep on the same readable source with a real-content oracle.
* DTail must deny `/etc/fstab` and an unrelated, disposable temporary file.
  No payload from denied files may be emitted. Treat status alone as insufficient.
* Verify root:dserver 0750 runtime/cache and root:dserver 0640 user cache; native
  dserver must read the cache and be unable to write there. Run two reviewed safe
  refreshes and recheck access/ownership. Verify only the inventory LAN listener,
  unchanged persistent identity and normal strict administrator SSH/doas.
* Compare cluster, applications and monitoring with the healthy preflight.
  Record failures and stop; native FreeBSD behavior and actual data are required,
  not just the local Linux fixture. Phone/laptop WireGuard device traffic remains
  a separate unperformed acceptance; these LAN checks do not claim it.

On failure, stop dserver only and retain the preserved identity, protected cache,
restricted binding/log rules, quarantine and root-private backups. The original
binary may be restored atomically from the verified root-owned backup while the
hardened config/helper/rc remain. Recheck its hash. If the old binary cannot pass
safe native acceptance, leave dserver stopped and report degraded DTail service;
do not reinstate the old daemon-writable cache, wildcard listener or broad reads.
Administrative access and all other services must remain unchanged. Do not delete
key/log backups or quarantine without an agreed retention action.

## Evidence and limits

Private receipts live under
`~/.local/state/conf-worktrees/kp3-f3-read-20261024/evidence/`: compatibility matrix,
client/server build metadata, native hashes and path/package metadata, f0 content
control, actual f3 handler disassembly and post-startup authentication result.
The normal power-on health and refreshed public identity are under
`~/.local/state/conf-worktrees/startup-20261025/`.

No production log payload or private key is retained in this report. A manual
legacy-SSH probe yielded partial bytes but failed the content oracle and is not
passing evidence. Local fixture success does not prove repaired native f3 reads.
The live repair, f3 end-to-end acceptance and source publication are all pending
explicit approval.
