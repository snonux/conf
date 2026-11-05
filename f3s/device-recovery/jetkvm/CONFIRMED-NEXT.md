# Next JetKVM confirmed runtime and owner/boot gates

Source-only proposal under s14. Root reviews exact code/drivers and serializes
all native windows. v28 ran and is terminal; further device access, file
replacement, lease signal, helper invocation, hook or boot needs its own reviewed
slice and exclusive-window clearance.
Only first device e10a0883debf7d4a/.1.241/.11.140 is in scope. v26 accepted
runtime fixed HTTP/SSH/media and independent expiry. v28 proved keep/repeat keep,
owner renew before/after keep and typed cleanup twice, but FAILED the former-
deadline decoded-video proof; its terminal state is confirmed and cleaned. See
[NATIVE-RUNTIME-2026-11-05.md](NATIVE-RUNTIME-2026-11-05.md).

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
all identities/bytes plus an independent controller archive. Current default
has only the SAME binary/profile bytes, no journals or authorization; ONLY-version
ran. Exact auth cleanup and fresh9 passed. This was an explicit file-only
transition, not an expired-generation shortcut or runtime grant. Proposed v32
requires fresh two-file admission and separate full-protocol review/GO.

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
above is retained as the reviewed sequence, not a whole acceptance claim. The
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

## Current video follow-up — old-path timing passed, fixed proof open

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
protocol, not a relabeling of v28/v29 failures. It does not accept the failed
fixed former-deadline frame; that needs its own reviewed native slice and safe
terminal-store transition. No setter/IDR/control RPC or decoder/network repair
is authorized by these diagnostics.

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

However, EARLY factory MAC and physical-link prerequisites are NOT yet proved.
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
before vendor setup assigns them, denying unchanged DHCP/app startup. Do not
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
