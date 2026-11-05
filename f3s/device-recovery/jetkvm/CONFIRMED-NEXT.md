# Next JetKVM confirmed runtime and owner/boot gates

Source-only proposal under s14. Root reviews exact code/drivers and serializes
all native windows; F3 currently owns the live window. No device access, file
replacement, lease signal, helper invocation, hook or boot is authorized here.
Only first device e10a0883debf7d4a/.1.241/.11.140 is in scope. v26 accepted
runtime fixed HTTP/SSH/media and independent expiry, not these later gates.

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

The cleanup source is NOT yet in the installed v26 binary. Tests cover both
terminal states, repeated cleanup, pending refusal, pending-primary/hash/boot/
foreign failures, retry and selected/pre-existing/foreign module preservation.

## File-only binary rollout before a confirmed trial

Review/commit/build first, then pin the new artifact size/hash. Profile stays
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

For the subsequent fresh runtime generation, privately archive the known v26
terminal metadata, then atomically replace ONLY authorization input as in v19.
Keep old active/owned/lock records; StartCanary accepts a DIFFERENT generation
because old active is expired. No journal deletion, manual reset or receipt.

## Proposed confirmed runtime and owner-renew slices

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
