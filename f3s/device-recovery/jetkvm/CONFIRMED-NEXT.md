# Next JetKVM confirmed runtime and owner/boot gates

Source-only proposal under s14. Root reviews exact code/drivers and serializes
all native windows. v32 ran and is terminal; further device access, file
replacement, lease signal, helper invocation, hook or boot needs its own reviewed
slice and exclusive-window clearance.
Only first device e10a0883debf7d4a/.1.241/.11.140 is in scope. v26 accepted
runtime fixed HTTP/SSH/media and independent expiry. v28 proved keep/repeat keep,
owner renew before/after keep and typed cleanup twice, but FAILED the former-
deadline decoded-video proof. A separate v32 generation later PASSED all three
fixed media phases with explicit startup/decode timing, renew/keep/repeat and
typed cleanup twice/native21; current v32 state is confirmed and cleaned. See
[NATIVE-RUNTIME-2026-11-05.md](NATIVE-RUNTIME-2026-11-05.md).

## Current checkpoint — 2026-11-06, 06:42 UTC

v43 FAILED live management readiness but completed verified cleanup. The new
helper retained a known negative result: original wait timeout, last collector
timeout, first-write persistence. Short observer lock times were measured; the
previous long-lock pattern was not repeated across readiness. The last code
is not enough to identify the earlier blocking predicate. No new boot is planned
before a bounded read-only readiness timing probe.

Original expiry of the negative state and all owned cleanup/auth/fresh9/archive/
process checks passed; management/primary/fleet acceptance remain false. Original7
is restored, historical6 unchanged, no managed resources/hook exist. Helper
remainsb759/b9 and current kernel is c910d0bc…. See the
[v43 record](NATIVE-RUNTIME-2026-11-05.md#v43-known-readiness-failure-and-completed-cleanup--2026-11-06).

## Earlier checkpoint — 2026-11-06, 06:09 UTC

Binary-only v42 PASSED with helperb75947bc…/4730284B/root0700/version0.1.0,
sourceconfb9e754e/clean embedded revision. It retains fixed-code negative
continuation evidence, including an independent bound diagnostic on result
persistence failure, and prevents admission with a stale readiness clock.
All original6 historical metadata bytes/inodes remain unchanged; complete
original7 archive, actual final native markers, auth/fresh9 and local
process/stream closure were independently verified. Kernel remains09aac103…;
no runtime generation/hook/network/reboot was introduced.

The corrected short-lock/pending observer and known11-file journal are source
work for a separate complete boot proposal. v41 remains FAILED, with no new
boot/primary/fleet acceptance. See the
[v42 file-only record](NATIVE-RUNTIME-2026-11-05.md#v42-terminal-diagnostic-helper-file-only-stage--2026-11-06).

## Earlier checkpoint — 2026-11-06, 05:12 UTC

v41 FAILED management boot proof after successful setup, one normal reboot and
old-address HTTP/H264 recovery. Early boot+20s observation prematurely required
a result before the original readiness deadline, but the late boot+331s snapshot
also had no result; the native continuation outcome remains unknown. Long
observer-held lock and discarded failure diagnostics are concrete source gaps,
with actual timing causality unproved. No blind reboot retry is granted.

Cleanup is terminal: exact hook/empty parent retired, whole failed nine-file
journal archived verbatim, original seven restored, all authentication/fresh9,
archive/native/path/process/stream checks independently verified by root. Helper
remains c29/d81, no managed namespace or hook exists, and historical metadata is
unchanged. A separate source-only observer/diagnostic correction is in progress;
no replacement helper, primary migration or fleet action ran. See the
[v41 evidence](NATIVE-RUNTIME-2026-11-05.md#v41-failed-deferred-boot-with-completed-cleanup--2026-11-06).

## Earlier checkpoint — 2026-11-06, 03:33 UTC

The v39 failed six-file store is archived verbatim and the original historical
seven-file store was restored under its exact locks. File-only v40 then installed
the corrected c29b9a51…/4657137B helper from confd81ef3d, preserving the other six
files/inodes and all historical metadata. Original access/fresh9/local drains
passed. No hook/generation/runtime network/reboot action was introduced.
Boot/primary/fleet remain open. The v39 checkpoint below is retained history.

## v39 checkpoint — 2026-11-06, 03:04 UTC

One normal boot recovered original-address HTTP/H264/USB, but deferred claims
were absent and initial/FIRST observations failed. Boot/fixed-path/expiry
acceptance remains FAILED. Separate minimal retirement subsequently removed
only the owned hook and empty created parent, after a619-file durable archive
and exact locked state checks. Final managed network absence, original parent,
authentication/fresh9 and local drains passed. Inert six-file unused seed state
still needs explicit whole-store reconciliation; no replay or reset is granted.
Configuration-hash drift remains unexplained and was not reclassified as success.

The concrete sysfs-symlink preclaim defect is fixed in confd81ef3d and covered by
native-composition regression/race/vet/errcheck. Its ARM helper c29b9a51… was later
installed by the separate file-only v40 stage above. See the
[full native record](NATIVE-RUNTIME-2026-11-05.md#v39-failed-boot-trial-and-owned-hook-retirement--2026-11-06).

## v37 preflight result — 2026-11-06

The first deferred-boot attempt stopped during its read-only beforeimage export
at02:27 UTC, before setup or reboot. The partial binary export and6.1986-second
elapsed time are consistent with the six-second native bound; actual exit code
was not saved. Original authentication, all nine fresh access checks and local
process/stream drains passed. No hook/new seed/reboot was introduced. This is a
FAILED attempt, with v36 still the accepted state; see the
[native evidence](NATIVE-RUNTIME-2026-11-05.md#v37-attempt-stopped-during-read-only-preflight--2026-11-06).
The separate successor remains source work until its whole protocol is reviewed.

## v36 file-only deferred-helper stage — 2026-11-06

The first device now has helper SHA256
`f9459c84744340e5f8a79d64f25bde7261069908f4dd81bfab7261cd36787cae`
(4588105 bytes, root0700, version0.1.0), built from conf2e31a63. Native file-only
replacement completed with preflight1.692s/stage4.331s. A complete exclusive
seven-file controller beforeimage was fsynced; the same stable native lock and
all identities/bytes were rechecked before atomic replacement of only the binary.
The other six files, profile2815, inode IDs and original v32 boot/deadline are
unchanged. Historical metadata was not relabelled to match the newer kernel.

The original temporary-access future completed, authentication restoration was
exact, all nine fresh access checks passed, PCs closed and owned processes
terminated. No hook/init parent, generation, authorization reset, runtime helper
action beyond `-version`, module, network setting or reboot was introduced.
Private full terminal evidence is
`~/.local/state/conf/s14-jetkvm-deferred-v36-20261106/runtime/terminal-sanitized.json`.

The installed [deferred role](helper/DEFERRED-MANAGEMENT.md) is still unaccepted
natively. Its first boot requires separately reviewed zero-intent input/store
and bounded-hook setup, followed by actual boot/fixed-path/independent-expiry
and cleanup proof. It does not replace the legacy/full-primary guard or grant
main-IP/fleet promotion. Those next protocols are source work only.

## Vendor readiness evidence after the early boot failure

The v33 early receipt proved that vendor startup had not yet established the
factory MAC or brought eth0 UP. Its observer hook was removed, and v34 removed
the exact empty init parent. A blocking management hook remains unsuitable.
Read-only v35 then completed with original authorization restored and all nine
fresh access checks passing. Two steady-state snapshots showed current-boot
`/run` tmpfs, stable DHCP daemon PID/birth/argv/session and complete lease,
SID equal to PID with PPID1, no child callback or unknown process reads, and
matching actual IPv4/default route/DNS plus factory MAC and administrative UP.
The optional proc children file is unavailable; direct PPID enumeration supplies
the child check. The lease is written before daemon backgrounding, so requiring
its timestamp after daemon birth would reject normal startup.

These facts support a bounded read-only readiness collector and validator.
They do not accept deferred boot timing or authorize a blocking pre-vendor
hook. Existing ManagementBoot/NoVendorApp and full-primary guards stay intact.
The v32 seven-file confirmed/cleaned store retains its old boot/deadline bytes.
Private v35 evidence is under
`~/.local/state/conf/s14-jetkvm-parent-readiness-v35-20261105/`.

## Source contract and the narrow cleanup addition

- `helper/cmd/jkrecover/main.go`: `canary-keep` passes the explicit
  `-fresh-recovery` boolean to KeepCanary. Go flags must precede the action.
  That boolean is root's external fresh-proof attestation, not a built-in
  media/evidence-file verifier. Root must bind positive HTTP/SSH/exact-pair/frame
  records to the actual same generation/profile/boot and current deadline.
- `helper/internal/recovery/canary_linux.go`: KeepCanary checks BOOTTIME before
  and after typed Probe, refuses expiry/mismatched profile/boot, then sets
  confirmed=true and boot_allowance=false. WorkCanary exits on confirmation
  WITHOUT cleanup. The original deadline stays recorded but no longer cleans.
  StartCanary refuses same-generation bootstrap after confirmation; never call
  bootstrap as a confirmed-path reapply/renew operation.
- ManagementBoot checks primary state before terminal cleanup. Confirmed OR
  expired records call existing Adapter.Cleanup without recreation. Its CLI
  first requires NoVendorApp, so it cannot clean a live confirmed runtime.
  The management-boot branch and wrapper remain unchanged.
- New source `CleanupCanary` / CLI `canary-cleanup` fills only that runtime gap.
  Under the existing lock, require no pending/invalid primary, exact profile
  and SAME boot, and confirmed OR expired additive state; then use existing
  typed Cleanup. Pending unconfirmed/unexpired state is refused even if its
  numeric deadline has elapsed: preserve the watchdog's own terminal action.
  No app stop, vendor JSON, hook, receipt, primary API or deadline change.
  Foreign ownership/metadata refuses; a busy module retains intent for retry.
  Confirmed state remains confirmed after cleanup; no receipt is created.

Root reviewed/committed the cleanup source as `0da9417`; v27 installed binary
SHA `a0857114…389c14b` without changing profile or terminal journals. The cleanup
action returned twice during actual v28 confirmed cleanup. Tests cover both
terminal states, repeated cleanup, pending refusal, pending-primary/hash/boot/
foreign failures, retry and selected/pre-existing/foreign module preservation.

## Completed file-only binary rollout; preserved protocol

Root completed review/commit/build and v27 file-only rollout, pinning the new
4,399,605-byte binary SHA `a0857114…389c14b`. This section records its protocol. Profile stays
EXACT2815fbf4031cda9346619e76b9037a29c0ce2f544fdb12a20f8bcb05913a3d91.
Do not reset the complete v26 terminal store or replace its stable lock.

A future bounded installer must verify original parent, same native boot,
expired/unconfirmed/no-allowance v26 active record, exact old authorization,
empty owned intents, old worker PID/birth gone, no child/modules/table/rule,
no hooks/receipt/primary/foreign files and all seven private inode/hash/mode
identities. Exclusively archive the complete beforeimage on the controller,
fsync it, then recheck all beforeimages while holding the same old lock.
Copy only a new exclusive root-private binary temporary file on the same
userdata filesystem, verify exact SHA/mode/root-owner/inode and ONLY-version,
then atomically replace only the old binary and fsync directory. Preserve
profile, authorization/active/owned/worker/lock bytes and identities. Cleanup
only the installer-owned temporary inode on failure; unknown drift stops.
No kernel/helper action beyond -version. New rollout needs its own reviewed
short exec/stdin and exact access cleanup; no implicit reuse of v25 permission.

v28 privately archived the expired v26 terminal beforeimage, then replaced ONLY
authorization input. Current v28 active state was CONFIRMED and cleaned, not
expired. Reviewed actual v31 archived that WHOLE seven-file store under its
stable lock to `/userdata/jkrecover-confirmed-cleaned-s14-v31-20261105`, retaining
all identities/bytes plus an independent controller archive. At v31 terminal the default
had only the SAME binary/profile bytes, no journals or authorization; ONLY-version
ran. Exact auth cleanup and fresh9 passed. This was an explicit file-only
transition, not an expired-generation shortcut or runtime grant. Subsequent v32
required fresh two-file admission and separate full-protocol review/GO, and has
now passed. Current default again holds a confirmed-and-cleaned SEVEN-file v32
terminal store; preserve it and its original deadline/boot/evidence. Do not
reuse fresh-two-file or expired-state admission without a reviewed transition.

Source clarification: `StartCanary` permits a different generation after a
confirmed or expired record. The v31 whole-store archive was an operator choice
to preserve evidence, not a CLI requirement. A later authorization still needs
current-boot admission and must not borrow stale ownership or rewrite history.

## Reviewed v28 runtime protocol and remaining acceptance

Use exact actual CPU/ABI/profile/module pins and original parent baseline.
Require global IPvlan/macvlan absence, absent parent upper links and serialized
writers before the fresh pending bootstrap; no namespace probing/claim. Keep
original vendor DHCP, factory MAC, old IP/default and all router/controller
settings. No boot allowance/hook during this runtime protocol.

1. Fresh pending generation, original110-second deadline, independent worker.
   Repeat v26 serial original-PC auth lifecycle and forced fixed path. Before
   keep, exercise ONLY the separately reviewed installed DHCP owner's renew
   operation, then prove actual owner/lease completion and unchanged old path,
   helper child identity/policy and fresh fixed HTTP/SSH/media. Abort to normal
   independent expiry if proof or remaining budget is insufficient; no extension.
2. After the first media PC closes, a fresh bounded old-path auth scope may
   attest those same-generation proofs, refresh fixed HTTP/SSH under its own
   temporary key, and invoke `-fresh-recovery canary-keep` BEFORE the deadline.
   Pin active generation/profile/boot immediately before/after; no competing
   writer can substitute a generation. Verify confirmed=true, unchanged original
   deadline, allowance=false and native worker PID/birth exited. Root's proof
   includes actual frame/pair; do not set the flag from mere reachability.
3. Exercise the SAME reviewed owner-renew operation after keep, without calling
   bootstrap, recreating a worker or issuing a new authorization. Restore exact
   temporary access before a new fixed RTC takes over. Prove fresh fixed media,
   USB/version and original DHCP/default after that owner action. Separate
   read-only observer waits beyond the FORMER deadline and repeats fresh proofs;
   the confirmed child/policy must survive while the worker remains gone.
   The source35s developer/14s media/5s frame budgets stay bounded; do not leave
   developer access enabled while waiting across the former deadline.
4. Close media first. Over a new verified OLD-path auth scope invoke the new
   `canary-cleanup` TWICE, only on the reviewed same confirmed generation/boot.
   First removes exact owned child/routes/rule/module; second proves idempotence.
   Failure retains journals and is reported; do not stop the app, manually delete
   interfaces or substitute management-boot. This is deliberate terminal cleanup
   after confirmation, not independent expiry, and must be labeled accordingly.
5. Separate native audit: confirmed/not-expired status and unchanged old deadline,
   same generation/profile/boot, worker gone, cleared ownership, both modules/
   child/table/rule absent, two settled parent snapshots matching ORIGINAL,
   unchanged DHCP/network settings/default, USB/version/video and all nine fresh
   old-access checks. Do NOT reuse v26's expired=true validator for confirmed
   cleanup. Archive the complete terminal evidence; no full receipt follows.

The actual v28 renew command bound the selected owner PID/birth/exact argv and
pinned selected handler immediately before direct SIGUSR1. This avoided the
app object's cached PID-only RPC target; readonly network settings and changed
lease output verified completion before and after keep. The original protocol
above is retained as the reviewed sequence, not a whole acceptance claim. The v28
former-deadline decoded frame remains failed (zero RTP), although fixed HTTP/SSH,
exact UDP/data and deliberate typed cleanup passed.

The concrete renew source is pinned app0.5.10
`f152581df82c519ecc4349f471dfc8612fbf0c48`:
`network.go:rpcRenewDHCPLease` → `pkg/nmlite/interface.go:RenewDHCPLease` →
`pkg/nmlite/dhcp.go:Renew` → `pkg/nmlite/udhcpc/proc.go:Renew`, SIGUSR1 to the
selected eth0 udhcpc process. The source finds PID via configured pid file or
matching proc entry; root must verify ONE expected live PID/birth/interface
before approving a signal. `/run/udhcpc.eth0.info` is the source lease path;
installed hook behavior/presence takes precedence. RPC return only proves a
signal attempt, not DHCPACK/bound/renew completion. Observe actual renewed lease/
owner event and settled old path without dumping credentials or vendor JSON.
Source `interface.go:applyDHCPLease` skips udhcpc address application because
its installed shell hook owns ifconfig/default/DNS; jetdhcpc instead reconciles
eth0 addresses. A first-device udhcpc renew is not jetdhcpc, rebind/expiry or
carrier-down/up acceptance. Do not call setNetworkSettings/toggleDHCPClient,
release leases, killall, full RkLunch or its stop script as a narrow owner test.

## Video timing follow-up — separate v32 fixed runtime passed

v29 reproduced zero-RTP/frame failure on a single cold fresh OLD-path view,
without a preceding capture PC. HTTP/exact UDP/data proofs passed; streaming was
inactive and the concurrent3s sleep getter timed out, so actual sleep remains
unknown. Pinned source automatically starts video on first session and can wait5s
for wake/relock under videoLock; no explicit view activation RPC is required.
The separately reviewed v30 protocol admitted actual decrypted H264 RTP by
session start+9s, then measured the unchanged5s decode limit inside14s media
(total35s outer/RPC3/jitter512). One OLD-path view passed: first RTP5.723s,
frame5.805s,186 RTP packets/208,250 bytes,12 assemblies and zero decoder warnings.
The concurrent sleep getter still timed out independently; actual sleep state
remains unknown. PC/wrappers/process closed. This is a new measured startup
protocol, not a relabeling of v28/v29 failures. It did not accept the failed v28 fixed former-deadline frame. Separate reviewed
v32 then passed all three fresh fixed media phases using explicit9+5 inside14,
including first postformer RTP5.721s/frame5.908s. Original/auth cleanup and native
cleanup twice/21 passed. V28/v29 failures remain preserved; no boot proof follows. No setter/IDR/control RPC or decoder/network repair
is authorized by these diagnostics.

## Actual v33 early-boot result and v34 retirement

The normal force=false device reboot/old-path recovery passed, but at native
uptime0.82s the vendor userdata boundary had eth0 MAC `8e:5b:23:67:39:ac`,
flags0x1002/UP0, both management modules absent. Current-boot receipt/CPU binding
and all fields were complete. FactoryMAC/UP prerequisites were therefore actually
NEGATIVE. The proposed synchronous blocking management hook must remain HOLD;
no manual setter/UP/network_init call is permitted. Vendor startup later restored
factoryMAC/original DHCP/default/parent and fresh old decoded media.

Owned observation hook and native readers/watchdog retired; original future/auth/
fresh9 passed. Its newly created empty init parent initially remained; DISTINCT
v34 then verified exactowned inode/root0755/empty/nonsymlink and retired it with
rmdir/fsync, preserving all helper/observer bytes and fresh9. Current helper store
is unchanged v32 CONFIRMED+CLEANED with OLD bootID under the NEW actual kernel.
Do not call helper cleanup/management boot or rewrite that metadata to repair it.
Private observational data stays inert, with no installed hook. See native report
for exact facts and timing. Next step is SOURCE-ONLY ordering design, no launch.

## Separate bounded boot gate — currently HOLD

A confirmed additive record is NOT the input to boot recreation: ManagementBoot
cleans terminal confirmed state and lets vendor startup continue. A later boot
trial needs a NEW unconfirmed generation with one boot_allowance=true. Only
after successful bounded bootstrap has persisted active state may its exact
management-only synchronous hook be installed; no missing-active hook window.
On the next native boot, ManagementBoot consumes/syncs the allowance, records
new boot ID and fresh120-second WindowNS deadline, starts a checked detached
worker, then ensures the additive path. Same-boot reapply does not renew it;
second pending boot or terminal confirmed/expired state cleans, no recreation.
This is a separate120-second BOOT window, not a changed110-second runtime limit.

EARLY factory MAC and physical-link prerequisites now have actual NEGATIVE v33
evidence. The earlier pinned source explains the observed ordering:
Pinned system0.2.8 `dc2b6d967fc16aa59b412ae306eb34f4c1ec3d0b`:

- `project/app/jetkvm/RkLunch.sh:rcS` runs userdata hooks synchronously BEFORE
  `post_chk` starts `network_init`.
- `network_init` calls ifconfig eth0 down, then
  `project/app/jetkvm/jetkvm-lib.sh:set_up_mac_address`, which chooses I2C/user/
  stored MAC and runs `ifconfig eth0 hw ether ...`, then eth0 up and udhcpc.
  The setter can also call fix_i2c_mac_address, migrate/write MAC files or choose
  a random fallback; it is not a harmless read-only prerequisite. Do not call it.
- `sysdrv/source/kernel/drivers/net/ethernet/stmicro/stmmac/dwmac-rk.c:
  rk_get_eth_addr` can retain a valid early address, read vendor storage or
  generate a random address. Post-start observed30:52:53:08:97:97 does not
  establish the MAC or UP/config state at the EARLIER hook boundary.

Even if early identity is correct, the child must subsequently survive the
stock vendor eth0 down/MAC reapply/up sequence and normal owner initialization;
runtime DHCP renew alone does not accept that sequence.

Thus the current blocking hook could loop on factory-MAC/parent prerequisites
before vendor setup assigns them (v33 actually observed differentMAC/DOWN),
denying unchanged DHCP/app startup. Do not
install/reboot that hook until a reviewed source-grounded early observation or
boot-safe ordering solution proves these prerequisites. No arbitrary parent MAC
setter, parent-UP manipulation, full network_init/RkLunch invocation or global
policy workaround is proposed. Preserve vendor ownership; any future ordering
solution must handle interruption without blocking the old path. A native boot
must record known or unknown host attachment and scope its actual USB/video
interruption in the separately reviewed device-boot window; never guess a mapping.

After an actually safe bounded boot proof, demonstrate normal vendor init,
old DHCP, forced fixed media, independent new120-second expiry and exact cleanup;
remove only the owned management hook while preserving terminal evidence. A
lingering terminal hook calls typed cleanup+success so old vendor startup can
continue; foreign/missing/corrupt state and cleanup errors remain fail-closed.
The separate FULL pre-app hook/external receipt transition remains a future
crash-safety checkpoint before any primary trial; no receipt is manufactured
and no full hook is installed by these additive tests. Repeat required owner,
boot and fixed media proofs on all four actual devices before fleet acceptance.
