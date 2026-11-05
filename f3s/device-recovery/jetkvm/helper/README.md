# JetKVM recovery helper

This standalone Linux Go module implements the revised protocol in
[the parent README](../README.md#revised-design-fixed-management-plus-one-target-bearing-boot).
It is installed on the first device; two unconfirmed additive runtime trials
proved detached expiry and exact cleanup, but routed fixed-channel access failed.
See [native evidence](../NATIVE-RUNTIME-2026-11-05.md). No address/profile is
allocated by this module. Full native acceptance remains task s14; offline tests
do not establish working remote access, boot recovery or DHCP-owner compatibility.

Build/check from this directory:

```sh
go build ./...
go test -race -cover ./...
go vet ./...
errcheck ./...
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -o /tmp/jkrecover-armv7 ./cmd/jkrecover
file /tmp/jkrecover-armv7
```

Verified locally on 2026-11-05: build, race tests, vet and errcheck passed;
`internal/recovery` statement coverage was 68.6% (CLI glue has no direct test
coverage). The ARMv7 artifact was identified as static ARM EABI5 with no dynamic
section. Tests include real filesystem/flock/rename/fsync behavior, detached
subprocess readiness and BOOTTIME reads, plus typed fake-kernel executor tests.
They do not execute BusyBox network mutations or establish native behavior.

The CLI refuses mutations unless root, the pinned hardware device ID matches the
vendor's CPU Serial verbatim (including a 15-digit serial), Linux reports the
reviewed ARMv7/5.10.160 ABI, and `/userdata` is mounted ext4 read-write. ARM
execution, detached monotonic expiry and exact owned cleanup passed on the
first device; fixed-channel reception failed. Watchdog/boot restoration and
DHCP-owner compatibility remain unverified. `-version` skips these prerequisites.
All network execution uses fixed BusyBox argv, with a three-second per-command
execution timeout, no shell. No native operation runs as part of the tests.

## Files and identities

Default private directory: `/userdata/jkrecover`, real directory mode 0700,
owned by the effective UID (root on device). Every metadata file is a real
mode-0600 file with the same owner. `lock` is a separate stable private regular
file, opened without following a symlink; state replacement never replaces it.
All helper commands share this flock. Rename, file sync and directory sync are
part of every acknowledged metadata/config transition. The vendor JSON remains
`/userdata/kvm_config.json`. No JSON/authentication/network values are logged.
The CLI has path flags for controlled packaging; use the defaults on the device.

- `profile.json`: the explicitly allocated current-cabling management profile.
- `network-owned.json`: profile digest, boot ID, module ownership, link intent /
  ifindex, and exact route/rule intents. Intent is synced before mutation so
  partial failures can be cleaned up. A reboot invalidates old kernel ownership.
- `canary-authorization.json`: root's explicit additive trial authorization.
- `canary-active.json`: immutable per-generation/per-boot deadline and consumed
  boot allowance. Re-running bootstrap cannot extend this private active record.
- `management-accepted.json`: external root/native proof receipt. The helper never
  creates or promotes one from a successful local command or canary confirmation.
- `state.json`: version 1, device, generation, phase, boot ID, absolute nanosecond
  CLOCK_BOOTTIME deadline, one boot allowance, saved and expected network objects,
  expected-object digest, private whole last-valid JSON, and fallback-used flag.
- `generations/<nonce>`: durable nonce tombstones; old worker/confirm identities
  cannot be reused by a later transaction. Nonces are 32–64 lowercase hex digits.
- `worker.json`: generation, boot ID, deadline, PID and `/proc/PID/stat` birth ticks
  from the checked readiness handshake. It is evidence of startup, not a claim
  of permanent worker health. Replacement workers retain the original deadline.

The profile schema is `device`, `cabling`, `parent`, `link`, `mac`, `address`,
`gateway`, `controller`, `table`, `priority`. This version deliberately implements
only `cabling=shared-igc0`, `parent=eth0`, `link=jkrecover`, an allocated address in
`192.168.11.0/24`, gateway `192.168.11.2` and controller `10.99.0.2/32`.
The MAC must be locally administered unicast. Table must be **1000–1023**;
priority independently remains **1000–32000**, and both must be unoccupied.
The shipped BusyBox 1.36.1 table-name parser caps IDs at 1023, while priority
uses a separate uint32 parser. Root-reviewed read-only v11 on the first device
verified empty tables 1000/1023 and exact rejection of 1024; no routing mutation
was submitted. The proposed first profile uses table 1000 / priority 1140.
The future igc1/10.98 profile is rejected. It requires its own
reviewed profile transition and physical-path acceptance. A matching gateway IP
is never treated as proof of current cabling.

The adapter uses the shipped `/oem/usr/ko/macvlan.ko`, creating the child DOWN
without `mode bridge` (unsupported by the installed BusyBox parser). It verifies
RTM_GETLINK kind, parent, MAC and ifindex, disables IPv6 only on its own child,
adds its IPv4 address, raises the child, then installs its two source-table routes
and one controller-specific source rule. The address also has its normal
child-connected route; no parent address, parent IPv6 setting, default route,
DHCP unit, global ARP/rp_filter policy or unrelated rule is changed. All occupied
names/table/priority are refused; reapply probes exact owned state. Cleanup deletes
only recorded exact routes/rule/link, preserves a pre-existing module, and refuses
foreign addresses/identity/policy changes or a busy module. Failed cleanup remains
an error with its ownership journal retained. Do not run concurrent manual kernel
network changes against these reserved resources.

## Bounded additive bootstrap and boot proof

Prepare narrow router/client routes/PF and reserve the profile before native use.
Root installs a private authorization object with fields:

```json
{
  "profile_hash": "SHA256 of the exact profile object as encoded by Go",
  "generation": "fresh 32–64 lowercase hex nonce",
  "boot": "current /proc/sys/kernel/random/boot_id",
  "deadline_boottime_ns": 0,
  "boot_allowance": false,
  "confirmed": false,
  "expired": false
}
```

The real deadline is an absolute CLOCK_BOOTTIME value no more than 120 seconds
in the future, not zero. The profile hash is SHA256 of `json.Marshal(Profile)`;
field order is the struct order listed above. Generate metadata on the controller
using the same schema; do not hash a pretty-printed file verbatim. Do not use
wall/NTP timestamps. Invalid/missing authorization fails before any mutation.

Run `jkrecover -device ID bootstrap`. It pins the authorization into active state,
starts a detached cleanup worker with a new session, null standard descriptors
and a pipe handshake binding boot/generation/deadline/PID/birth, then creates the
owned additive path. There is no dependency on a previously accepted alias.
The worker cleans up only owned additive resources at expiry; it does not write
vendor JSON, renew DHCP, stop the app, reboot, or touch host power.
`jkrecover -device ID -fresh-recovery canary-keep` cancels that canary's cleanup
only after the controller's fresh authenticated fixed-path proof. This does not
issue a full acceptance receipt. Repeated bootstrap is bounded by the already
persisted deadline and refuses a different pending generation.

For a bounded boot recreation trial, root issues a new authorization with
`boot_allowance=true`, boot ID of the current boot and a fresh nonce, then runs
bootstrap and reboots the JetKVM within the armed window without confirmation.
Only after successful bootstrap has persisted active authorization, install the
**management-only** synchronous wrapper [management-boot.sh](management-boot.sh)
with its pinned ID before that boot. Do not install a hook without active state,
or reboot if installation consumes the remaining armed budget.
`management-boot` requires no pending primary
transaction. It consumes/syncs the single allowance, records the new boot ID and
new 120-second deadline, verifies a new detached cleanup worker, then recreates
the additive path before returning. A same-boot reapply keeps that deadline.
Another unconfirmed boot cleans up instead of recreating. Lost worker startup
synchronously cleans the additive path before returning an error. The wrapper
blocks startup on any error because RkLunch ignores hook exit status.

Confirmed or expired additive state takes a terminal cleanup branch: remove
only exactly owned additive resources and return success so unchanged vendor
DHCP/Dropbear/app startup can continue. A previous-boot ownership journal never
authorizes deletion of a foreign surviving resource. Cleanup failures still block
startup. This avoids stranding a device if the management-only hook remains at
an unexpected reboot after keep; it does not promise a persistent fixed endpoint
for terminal additive trials. The pending-primary refusal precedes this branch.
The full pre-app hook's receipt/readiness and worker-failure behavior is unchanged.
Remove only the owned management-only hook after its completed bounded boot
trial, until the separately reviewed full-hook transition. Source-only terminal
cleanup tests do not establish native startup acceptance; see
[the first-canary plan](../NATIVE-CANARY.md).

After real fresh fixed-path, boot, stock network_init, udhcpc **and** jetdhcpc
renew/rebind and owner-reapply proofs, USB/video/media/authentication checks,
replace the management-only wrapper with [pre-app.sh](pre-app.sh), pin its device
ID and verify the full wrapper installation. Only then install the full external
receipt with matching `profile_hash`, `device`, `cabling`, and all true booleans:
`fresh_authenticated_fixed_path`, `boot_recreation`, `dhcp_renew_rebind`,
`owner_reapply`, `usb_video_media_auth`, `full_transaction_hook_installed`.
The receipt attests root's native evidence; a JSON file alone proves none of it.
The additive canary must first be confirmed so no old cleanup guard can remove
the permanent path. The main arm requires this receipt plus a live exact
self-probe and rejects any still-pending additive guard.

## Primary transaction protocol

1. Quiesce UI/OTA/other configuration writers. Capture/verify old literal-address
   identity and authentication before proceeding. The helper snapshots the valid
   whole vendor JSON privately; it does not export config or credentials.
2. Prepare the exact expected `network_config` object in a root-private file.
   `jkrecover -device ID arm NONCE TARGET_FILE` persists saved/expected fields,
   original boot ID, one allowance, 120-second absolute deadline, and starts a
   detached worker. No vendor JSON is written by arm. Only command success after
   readiness permits the controller's vendor SetNetworkSettings RPC. Do not issue
   it after an arm error or after the issuing window has elapsed.
3. The vendor RPC owns applying/saving the new primary and rebooting. On the first
   boot carrying the pinned expected field, `pre-app` consumes/syncs the allowance
   before guarded startup. It recreates/probes the accepted fixed path and starts
   the new monotonic worker. Worker failure, wrong field, invalid latest JSON,
   elapsed deadline, or any later pending boot synchronously restores before app
   startup. A runtime invocation is refused while `jetkvm_app` exists.
4. Runtime expiry first marks/syncs `restoring` and revokes the allowance under
   the lock, then requests a Linux kernel reboot. It never writes live app JSON,
   calls the network RPC, stops/restarts the supervisor or accesses a host ATX API.
   Failed reboot requests stay pending and the worker retries once per second.
   Only the pre-app hook merges the saved network into the latest valid document.
   Every unrelated/unknown field is preserved. If latest JSON is truncated/invalid,
   the private whole last-valid fallback is used and `fallback_used=true` records
   it. Use `jkrecover -device ID status NONCE` to inspect the public phase/fallback
   status and recheck auth/USB/video/media; never print the private state journal.
   Concurrent configuration edits during fallback cannot be preserved; quiescence
   is a protocol prerequisite, not something the helper can infer.
5. After **fresh authenticated literal target and fixed recovery sessions** with
   the correct device identity and required services, run
   `jkrecover -device ID -fresh-target -fresh-recovery confirm NONCE`.
   These flags are controller attestations; the device does not invent proofs
   from interface presence. Expiry wins once restoration is recorded. Stale
   generations/boots and late/missing proofs cannot cancel it. Confirmation is
   idempotent and retains the fixed management path and the new primary.

The full wrapper retries on error instead of returning to RkLunch. A successful
saved-profile merge or a ready guarded target boot is required to pass. Permanent
storage/hardware failure can block startup and is not a successful software
restore. Native failure injection must prove this branch. The kernel process
scan is an additional runtime misuse check; install the wrapper at the proven
synchronous pre-app hook, not under a concurrent runtime supervisor.

A planned and an unexpected first reboot carrying the target field are
indistinguishable. Exactly one such pending boot is allowed. Each pending live
boot has a 120-second monotonic window; reapply/replacement cannot renew it.
The cross-boot bound is conservatively **120 + 120 + two measured reboot/startup
bounds**, not a single 120-second operation. Native measurement must include
worker readiness, operation/lock scheduling, cleanup/restoration, kernel reboot
and app startup. The helper cannot guarantee recovery after kernel/hardware
failure or permanent storage failure. Neither a mock nor an old open session is
native evidence.

## Native acceptance still required

Root reviews the code and concrete installation plan before any device writes.
First prove a runtime additive timeout removes owned resources and fresh old
access survives; then separate confirmed/idempotent reapply and bounded boot
recreation trials. Observe the already measured transient parent IPv6 DAD effect,
its settling and switch/VEPA behavior. Inspect actual ARP/rp_filter and prove the
controller return route without globally weakening policy. Preserve exact native
DHCP, Dropbear root/public-key authentication and temporary developer-access
metadata; never delete SSH access while a pending helper still depends on it.

For each actual DHCP implementation, prove network_init parent down/up, lease
renew/rebind and app owner reapply retain the child. Check fresh old/recovery
HTTP and authenticated SSH, correct device ID, settled IPv6 and fresh working
USB/video/media after every stage. A wildcard listener or capability-only down
link is insufficient. The external receipt is created only after those tests.

Then run the main transaction with unchanged primary network first: withhold
confirmation, prove real expiry/reboot/pre-app restore and fresh fixed/old paths.
Inject worker startup loss and an additional unconfirmed reboot. Only after the
router/client target gates pass may a bounded actual target RPC trial proceed.
Separately prove target/recovery confirmation, accepted reboot/reapply, stale
worker/confirmation refusal and exact temporary developer-access cleanup.
Do not retain an existing WebRTC session as proof. Do not operate host power.
Keep all old allocations and the current-cabling recovery path until its separate
retirement gate. The future igc1 transition is outside this profile's support.
