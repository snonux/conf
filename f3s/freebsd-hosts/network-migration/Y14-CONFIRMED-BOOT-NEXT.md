# Confirmed-NEW boot guard — dormant installation and next native gates

The reviewed dormant helper is installed; guard expiry and confirmed-NEW boot
remain unaccepted. f3 remains OLD-primary Stage after the accepted 300-second
expiry and 600-second OLD boot recovery. The first guard trial stopped before
confirmation and restored OLD; see [native evidence](Y14-PRIMARY-NATIVE.md).
No primary confirmation, guard activation or confirmed-NEW reboot is authorized
by this file. Root reviews the exact native slice separately.

## Minimal source contract

The separate `/var/db/gonf-network-migration/confirmed-boot-guard` holds a
root-private OLD working snapshot and a fixed deadline. It never edits ordinary
pending to waive early boot rollback. Ordinary stage/promote/confirm/rollback
refuse an active guard; Prepare requires both ordinary-pending and guard absence
before replacing helper/config/hooks. Boot and the existing minute watchdog own
guard recovery; no new daemon, cron line or task914 change is introduced.

All guard directories are root:wheel0700 and files0600, with symlinks refused.
The immutable input must be a known-hash accepted OLD dual-Stage snapshot under
this helper's state directory. Its contents and the working copy are rechecked;
only the existing restore function's generated `v6.remove` scratch is excluded.
Inventory config and helper hashes are pinned while active. New forensic rc.conf
backups are explicitly0600; live rc.conf retains its own mode. Older immutable
archives are not silently chmodded or adopted to pass this admission.

A complete private preparing directory is validated before atomic publication
and sync. Failed copying/validation cannot publish a partly armed guard. The
fixed120..900-second lifetime is not renewed by retries. One fresh kernel epoch
may be claimed, with durable intent/epoch records. Same-kernel reentry is
idempotent; expiry, a second kernel, or interrupted/malformed/nonprivate claims
restore OLD through the validated backup. A restoring marker keeps partial
runtime recovery retryable even before the deadline. The original archive is
never the mutable restore workspace.

Release requires a complete new-kernel claim, the same current kernel, NEW
persistent/runtime primary/default and retained OLD alias. It checks the clock
again after the final runtime query, immediately before archive commit. A query
that crosses the deadline cannot cancel rollback. Expired release leaves the
guard armed for native restoration. All source/endpoint/peer/crypto protections
and the six-line PF source guard remain in effect.

## Native CLI proposed by this source

`ARCHIVE` is an accepted archive basename, not an arbitrary path. The first
command is read-only apart from an ephemeral private hash manifest. The hash
must be pinned in the execution record and compared with the original accepted
Stage before any guard arm.

```text
gonf-network-migration confirmed-boot-hash ARCHIVE
gonf-network-migration arm-confirmed-boot ARCHIVE HASH [seconds=600]
gonf-network-migration restore-confirmed-boot
gonf-network-migration release-confirmed-boot
```

Arming requires the existing explicit activation permission, no ordinary or
endpoint pending transaction, and already-confirmed NEW runtime/persistent
primary/default with OLD alias retained. Arming itself writes no addresses or
rc keys. A failed arm is not an armed recovery promise: do not reboot. Explicit
guard restoration is idempotent and archives the complete guard. Release archives
it without reverting NEW. `status` reports an active guard separately.

Use a freshly rebuilt/pinned current Go controller binary containing the new
Prepare admission; an older private t14/u14 binary can load new asset bytes
while still lacking that Go check. Freeze its path/hash and the shell asset
independently before installation. Never replace either during an active guard;
prove that native scoped reapply refuses and preserves both surfaces.

## Two separate proposed native slices

First prove independent guard expiry after ordinary confirmation, without a
reboot: start from original OLD Stage; native promote shared120, all required
NEW-source NAT/six-peer/guard/full-health checks, ordinary confirm before expiry,
then pin its accepted OLD snapshot and arm a300-second guard. Require confirmed
NEW beyond the former120 deadline plus a full minute cadence, close origin SSH,
and let the native guard restore OLD independently. Observe actual OLD before
manual action, compare both configuration/runtime and every protected field,
explicit guard restore twice, archive/reapply/permission and UID cleanup. No
release is needed or accepted without a new-kernel claim.

Only after that native slice passes, propose the actual confirmed-NEW boot:
gracefully stop all VMs before arming, preserve vm_list and normal stopped policy;
promote shared300, validate and confirm, pin the original OLD snapshot, then arm
a600-second confirmed-boot guard. Record actual deadlines/epoch and original
snapshots, with enough time remaining for measured boot and acceptance. Fresh
source guards/state/aliases/routes/NAT/peers/health and all writer schedules must
fit the entire lease/cadence/margin. Root holds other network writers.

A separately reviewed causal proof must establish persistent NEW before
physical/WireGuard configuration in a genuinely new kernel, then actual NEW
primary/.20.1 default, OLD alias/ULA/.99 return and strict original WG fields
before LOGIN. After startup require all six actual OLD-source outgoing peer
transports and bidirectional overlays, zero extra guard drops, four fresh
old/new control paths, paired native NAT SYN/PF tuples and verified NEW-source
HTTPS, full cluster/NFS/DNS/API/TLS and normal Rocky-auto/FreeBSD-stopped policy.
Any failure immediately restores the guarded OLD snapshot, never a stale whole
tunnel or remote peer configuration.

After the former ordinary deadline plus cadence, release only while the outer
guard is fresh and all acceptance holds. Observe NEW beyond the former guard
deadline/cadence, preserve complete evidence, remove only own permissions/probe
UIDs and reapply unchanged. This future proof establishes confirmed-NEW cold
startup; the completed600-second OLD-recovery boot does not substitute for it.

Source YAML stays OLD throughout these slices. Per-destination f3 source
promotion still requires every actual referring host's persisted recovery and
route admission in the immutable full manifest; other destinations stay OLD.
No dependency on later all-destination address promotion or task924 is created.
