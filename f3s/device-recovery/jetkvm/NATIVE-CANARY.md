# First native JetKVM management canary

Prepared 2026-11-05 for s14, after offline helper commit `8379ae8`.
**Review candidate only. Nothing in this file allocates addresses, installs a
helper/hook, enables developer access, or authorizes concurrent live work.**
Root coordinates network/bootstrap/reboot work after the host/endpoint worker
reports no pending trial. Separately reviewed file-only staging is independent. IQ owns allocations and acceptance; this repository owns deployment
source. Begin with **e10a0883debf7d4a**, authenticated at **192.168.1.241**.
Do not generalize its results to the other three devices.

## Shared-wire readiness accepted by root

IQ commit `71b7503` reserves management `.11.140`–`.11.143` and their unique
MAC rows. Root reconciled fresh OpenWrt selected inventory and observed all four
duplicate-address probes on router igc0 without replies. The guarded router
transaction `s14-jetmgmt-20261105` is confirmed with no pending restore: it adds
only first-device TCP22/80 and UDP32768–60999 from laptop `10.99.0.2` to
`192.168.11.140`. Old/new/VPN router SSH and protected-state checks passed.

The laptop's private source, root-owned persistent wg2 config and runtime now
retain the four exact reserved `/32` routes/AllowedIPs; existing keys, peers,
routes, DNS/default and wg1 are unchanged. These are path-readiness results;
the separately reviewed file-only staging result below establishes only the
installed artifact, not a working child or accepted recovery path.

The single authorized v12 file-staging attempt exited 1 with no remote output,
before its first absent-path marker and therefore before mkdir or file writes.
Exact developer cleanup and fresh independent HTTP/WebRTC postchecks passed.
Pinned system source has `CONFIG_STAT` disabled, explaining the stat-applet
preflight failure by source inference; v12 did not emit an individual native
applet result. The Go helper's internal `Lstat` does not depend on a stat applet.
A private v13 installer candidate replaces shell stat with selected BusyBox
`ls -ldni` metadata, validated before writes, and emits named preflight booleans.
It remains unexecuted pending root review. No helper install is accepted.

## First file-only staging accepted: v13

Root reviewed and ran the private v13 installer on the authenticated e10 device.
Every named ABI/CPU/ext4/tool/metadata/path-absence/profile preflight passed.
Only `/userdata/jkrecover` (root:root0700), its `jkrecover` binary (0700) and
`profile.json` (0600) were created. No hook, authorization, active canary,
network interface, module, route/rule or vendor-network write was made.

The 4,394,216-byte artifact's device SHA256 matched
`6628486bfb04c4f361a2744f55264a4089774826dff15e63341d02d72f9809bf`;
the226-byte profile matched
`39836aee2b29e77ae6ec9a7c2d50f904c189c099825334023d30b00d961702ef`.
The only helper action was `-version`, which returned0.1.0 on the actual ARM
CPU. SSH staging took2.07124s; the native copy took1.345231s. These results
prove transport, metadata and basic native execution, not bootstrap recovery.

The preceding v12 attempt exited before its first write marker; no installation
was accepted. The pinned image configuration disables BusyBox stat, explaining
that guard failure, although v12 did not print the exact failed guard. v13 used
validated numeric `ls -ldni` metadata within the same userdata filesystem and
retained exclusive file creation/inode-owned partial cleanup. Exact developer
mode false/TCP22 closed/empty authorized-key restoration and original enable
request termination passed after both attempts. Separate fresh HTTP/new-WebRTC
identity, version, USB and video-ready checks passed. Private evidence is under
`~/.local/state/conf/s14-jetkvm-first-install-v13-20261105/` (and the failed
v12 sibling). No credential or full vendor configuration is published.

## Native read-only v10/v11 checkpoints

One root-reviewed inspection on 2026-11-05 verified the exact CPU Serial
`e10a0883debf7d4a`, Linux `5.10.160 armv7l`, writable ext4 userdata, and kernel
UDP ephemeral range **32768–60999**. The installed socket tables showed TCP80
and temporary developer TCP22, with no TCP443 listener. The helper directory,
child and hook were absent. Parent/all/default `rp_filter`, `arp_filter`,
`arp_announce` and `arp_ignore` were all zero; no policy was changed.

The read command then failed closed with
`ip: invalid argument '1140' to 'table'`. It did not reach priority absence or
the parent semantic snapshots. Those checks were incomplete at v10; v11 below
records their subsequent successful verification. All exact
developer-access cleanup and separate fresh HTTP/new-WebRTC ID/version/USB/video
checks passed. Private evidence is under
`~/.local/state/conf/s14-jetkvm-inspection-v10-20261105/`.

The pinned system BusyBox 1.36.1 tarball's `networking/libiproute/rt_names.c`
limits table IDs to 1023. Its `iprule.c` parses priority separately as uint32.
The follow-up root-reviewed v11 succeeded: tables 140/1000/1023 were empty,
1024 produced the exact expected status-1 parser rejection, priority 1140 was
absent, and both parent snapshots matched each other and the settled v9
baseline after ignoring numeric address lifetimes only. Neither snapshot had
tentative/DAD-failed IPv6 or the probe child/module. Exact cleanup and fresh
HTTP/WebRTC postchecks passed. Evidence is in the matching private v11 directory.

The source helper now caps tables at **1000–1023**, retaining the independent
priority range **1000–32000**. First-profile proposal is table 1000 / priority
1140. Focused tests model the actual BusyBox rejection and exercise the highest
accepted table with a larger priority. Native command acceptance/read-only
absence does not prove addressed routing or ownership cleanup. No native
helper/interface/module change occurred; bootstrap still needs root's review
of the exact installation/profile/authorization and allocated path readiness.

## Source ownership and minimum preparation

Use [the parent design](README.md) and [helper protocol](helper/README.md).
Keep original vendor DHCP, factory MAC, parent addresses/default, AP/SSID,
controller old address, wg1, and every existing WireGuard key/peer unchanged.
No host-power, USB configuration or virtual-media mounting action is part of
the additive trial. A fresh decoded video stream/data channel can be checked
without sending keyboard, mouse or power commands.

| Owner | Concrete preparation before the first addressed trial |
|---|---|
| Root / IQ ADDRESS-RECOVERY | All four management rows reserved; keep original allocations and shared-igc0 identity. |
| conf `gonf/triggerfish/profiles.go` | First tuple only admitted and guarded PF transaction confirmed; retain `192.168.11.2/24` on igc0. |
| Laptop wg2 owner | Four exact reserved `/32`s accepted in private source, persistent config and runtime; source remains `10.99.0.2`. |
| This helper | Pin ARMv7 artifact hash, actual CPU Serial, profile and private metadata; initially install no boot hook and no acceptance receipt. |

The allocated management subnet has no DHCP server. Root's fresh selected
inventory reconciliation and on-link probes are recorded above; missing ARP
replies alone would not prove a sleeping client absent. Preserve that inventory
ownership and refuse new conflicts. Do not start a DHCP server or alter vendor
reservations to reserve a
macvlan address. Reject a conflicting IP/MAC or unexplained provisioning client.
The router's `.11.2` is **shared igc0**, not igc1 recovery.

The laptop route owner is the existing root-private `/etc/wireguard/wg2.conf`
and its current wg2 peer. Root accepted the four exact reserved `/32` additions
from a private candidate with protected fields compared without printing keys.
Future changes must keep persistent source and existing-peer/runtime state
together, with bounded rollback restoring only the owned delta. Do not bring
wg1 down, replace peer keys, broaden a peer AllowedIPs prefix or use a runtime
`ip route` command as the sole source. Prove the retained router wg2 session and
`ip route get` source `10.99.0.2`, plus unchanged old IPv4/DNS/default routes.
Root coordinated that transaction after router candidate acceptance. No further
laptop source/runtime change is part of helper file staging.

Proposed first profile, only after reservation and a fresh CPU Serial check:

```json
{"device":"e10a0883debf7d4a","cabling":"shared-igc0","parent":"eth0","link":"jkrecover","mac":"02:52:53:08:97:97","address":"192.168.11.140/24","gateway":"192.168.11.2","controller":"10.99.0.2/32","table":1000,"priority":1140}
```

Recheck table 1000, priority 1140 and child name are unused; refuse foreign state. The
CLI pins `/proc/cpuinfo` Serial verbatim, not a padded conversion of the API ID.
Read the actual 15- or 16-digit serial and reconcile any API/CPU disagreement
before finalizing `device`, wrappers or CLI arguments. Verify `uname` reports
`armv7l` / `5.10.160` and `/userdata` is writable ext4. The v10/v11 reads
verified these facts for this canary. The helper artifact has not executed
on the device.

## Media ports and narrow router candidate

Pinned app source is `/tmp/conf-s14-jetkvm-source`, HEAD
`f152581df82c519ecc4349f471dfc8612fbf0c48` (app 0.5.10).
`web.go:201` carries authenticated signaling over WebSocket;
`web.go:608` starts HTTP port 80. `web_tls.go:164` uses port 443 when TLS is
enabled. `getBindAddress` also honors loopback-only and IP-version settings;
source wildcard capability does not prove the installed listener accepts a
new child address. Check actual sockets and authenticated access.

`webrtc.go:431` constructs Pion settings without an ephemeral-port limit,
UDP mux, or interface/IP filter. Its local session has no cloud ICE rewrite.
Pinned dependencies are Pion WebRTC v4.2.1 and ICE v4.1.0 (`go.mod`). ICE
[gather.go](https://github.com/pion/ice/blob/v4.1.0/gather.go#L342)
binds UDP sockets on interface IPs using port zero; its
[agent configuration](https://github.com/pion/ice/blob/v4.1.0/agent_config.go#L72)
leaves the UDP allocation policy at default when limits are zero.
There is no source-grounded fixed JetKVM UDP port or guessed 49152+ range.
Read each installed kernel's `ip_local_port_range` and occupied UDP listeners;
verify actual answer candidates against it. Keep SDP/ICE credentials private.

The source candidate should add explicit provisioning destinations rather than
expand `<lab>` or the existing general wg2 permissions. For the first reviewed
shared-igc0 profile, the intended PF semantics are:

```pf
table <jetkvm_management> const { 192.168.11.140, 192.168.11.141, 192.168.11.142, 192.168.11.143 }
pass in on wg2 inet proto tcp from 10.99.0.2 to <jetkvm_management> port { 22, 80 } tag gonf_transit keep state
# Add 443 only if its installed listener and intended test require it.
# Substitute the verified decimal kernel bounds below; this is not loadable PF.
pass in on wg2 inet proto udp from 10.99.0.2 to <jetkvm_management> port VERIFIED_LOW:VERIFIED_HIGH tag gonf_transit keep state
```

The existing tagged egress permit handles accepted transit. No NAT, phone-peer
permission, general provisioning-subnet pass, mDNS reflection, or device-initiated
UDP exception is required by this candidate. Client-initiated ICE checks should
open the exact UDP state; prove that behavior with fresh candidate nomination
and router state/counters. If it fails, inspect the exact tuple/cause before
proposing another permission. Device-side unsolicited ICE attempts may be
blocked without making the controller-initiated pair unusable.

The source-only `RouterProfiles.JetKVMManagement` field now takes exact admitted
addresses and per-device `UDPMin`/`UDPMax` bounds. It renders only `shared-igc0`
and requires `RetainProvisioningAlias=true`; validation rejects other addresses,
duplicates and zero/reversed bounds. Cluster inventory now admits only the first
device tuple; its exact two-line PF addition is confirmed. The other three
must be inspected independently before admission. Existing `MigrationFlow`
supports single ports and physical ingress;
it cannot represent this wg2 UDP range. Do not weaken that type's validation or
create thousands of per-port flows. The four-address rule is a ceiling; no
unreserved row may enter an active candidate. Different verified device ranges
need separate exact destination/range rows rather than an invented common one.

After root applies through gonf, require native `pfctl -nf`, fresh router old/VPN
SSH, no pending router journal and exact candidate rules. On the laptop verify
`ip route get 192.168.11.140 from 10.99.0.2` selects wg2. A route or PF parser
success is readiness only; first native child traffic must prove forwarding and
return source selection. Negative probes should cover an unlisted destination and unlisted TCP port.
Inspect the exact PF source restriction and dormant phone-peer configuration;
keep that peer inactive and do not change its keys or impersonate it for a test.

## Temporary SSH and read-only preflight

Reuse the exact reviewed private v9 driver procedure, from
`~/.local/state/conf/s14-jetkvm-inspection-v9-20261105/conf-s14-inspect-v9.py`,
with root reviewing the new command and budgets before execution. Do not run
that driver merely to prepare this file. Its safety sequence is:

1. Fresh literal-old HTTP login and new local WebRTC connection; check exact
   device ID and app/system 0.5.10/0.2.8, save USB/video baseline privately.
2. Read exact prior developer mode and authorized-key scalar. Require the
   expected false mode/empty key or stop on drift; save privately at mode 0600.
   Add one temporary Ed25519 key and verify exact readback.
3. Start developer enable concurrently with bounded polling, retain the enable
   future, and stop if it fails or effective enable/22 does not appear in budget.
   Inspect advertised auth without credentials. Permit publickey-only client
   authentication only; verify effective root shadow is locked and passwd uses
   shadow without printing either file or a hash. Pin the host key privately.
4. Collect only selected CPU/ABI/mount, hook existence/order/ownership, BusyBox,
   parent link/address/routes/rules, module absence/presence, table/priority,
   socket, boot ID/uptime, ARP/rp_filter and ephemeral-range facts. Do not read
   full vendor JSON, credentials, private keys or shadow contents into logs.
5. Always drain the original enable request to terminal state before final
   false-mode/closed22 verification; restore the exact original key scalar and
   verify it. Fresh old HTTP ID/version plus USB/video checks follow cleanup.
   A timeout or partial cleanup is an error, not a permission to proceed.

For later armed trials, do not close temporary SSH while it is still required
to prove or clean pending state. The existing read-only driver's 35-second
outer mutation budget is not suitable for a 120-second native expiry. Review
a distinct bounded trial driver which keeps the same terminal-request and
exact-cleanup logic, allows the independent expiry observation budget, and
never kills the detached helper when its controller connection ends.

## Serial native trials after root review

Build from helper source with `CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7`.
Record SHA256 and static ELF identity. Through the reviewed temporary SSH
session, create only the absent real root-private `/userdata/jkrecover` (0700),
copy the artifact (0700) and profile (0600), and verify exact hashes/modes.
Refuse symlinks or an existing unexplained helper tree. First `-version` proves
execution only. Do not install any hook or receipt for the runtime expiry trial.

Before each bootstrap, create `canary-authorization.json` privately with the
helper's exact Go-encoded profile hash, a fresh 32–64 lowercase-hex generation,
current boot ID, absolute CLOCK_BOOTTIME deadline within 120 seconds,
`boot_allowance=false`, `confirmed=false`, `expired=false`. Use same-schema
compact JSON serialization in Profile field order; verify its digest rather
than hashing pretty JSON. A conservative deadline of the freshly read whole
`/proc/uptime` seconds plus 110 seconds, converted using controller integer
arithmetic to nanoseconds, leaves budget below the helper's 120-second limit;
record acquisition latency and reject a late launch. Do not use wall/NTP time.
No `management-accepted.json` or primary `state.json` is fabricated.

1. **Actual expiry first.** Execute `jkrecover -device CPU_SERIAL bootstrap`.
   Require success and detached worker readiness before any success claim.
   Close the originating SSH/controller trial process deliberately. An independent
   observer checks new literal `.11.140` HTTP authentication/ID and fresh key SSH
   via laptop wg2, plus a newly negotiated WebRTC session. Restrict the test
   client's accepted remote ICE candidates to `.11.140` and bind its local socket
   to `10.99.0.2`; verify nominated candidate pair, decoded video frame and data
   channel. Otherwise the old DHCP candidate can conceal a broken recovery path.
   Withhold keep; after the original absolute deadline plus worker/command
   allowance verify child, recorded routes/rule and only the owned module are
   gone, active canary expired, and fresh old HTTP/SSH/auth/USB/video works.
2. **Distinct confirmed runtime trial.** New nonce/deadline; bootstrap, repeat
   the fixed-path proofs, then `jkrecover -device CPU_SERIAL -fresh-recovery
   canary-keep` before the deadline. Repeat keep to prove idempotence; bootstrap
   after confirmation is expected to refuse, not extend a deadline. Check the
   exact owned network state remains, including after the former deadline.
   A new bounded generation can subsequently clean this owned path on expiry.
3. **Separate unconfirmed boot recreation, after root reviews the fix below.**
   Only after both runtime results, create a new nonce/current-boot authorization
   with `boot_allowance=true` and bootstrap successfully. Then install the pinned
   [management-only wrapper](helper/management-boot.sh) as root-owned executable
   `/userdata/init.d/S00jkrecover`, verifying the actual installed synchronous
   S?? hook and absence of a conflicting file. Preserve the exact absent/present
   directory state. This ordering avoids a hook with no active authorization;
   never reboot if installation consumes the remaining armed budget. Request a
   normal JetKVM kernel reboot within the window. No primary-network RPC and
   no host reboot/power API.
   Observe actual boot-ID change, consumed allowance and fresh boot deadline,
   pre-app child recreation, stock network_init and settled original DHCP.
   Let this new deadline actually expire; prove cleanup and fresh old access.
   If startup remains blocked, do not claim boot acceptance or improvise a
   parent/network repair.
4. **Remove the owned management-only hook after that completed trial** until
   the later full-hook transition. Remove only the exact installed hook, never
   an unrelated init file; retain private evidence. This is housekeeping after
   the terminal-state fix, not the safety mechanism for an unexpected reboot.

Observe the v8 parent-IPv6 DAD transient immediately after creation and until
settled; compare semantics against baseline (ignore numerical lifetimes only).
Record maximum settling/cleanup/reboot/startup durations, parent MAC/MTU/flags,
IPv4/default/IPv6/rules, unchanged network configuration and USB/video/auth.
Unexpected loss of the old path, sustained tentative IPv6, foreign ownership,
wrong ID, missing worker, missed bound or cleanup error stops the next trial.
Do not fix these by changing parent/global ARP, rp_filter or IPv6 policy.

## Boot terminal-state defect and source correction

**Root must review the source correction before hook installation.** Runtime
expiry without a hook can proceed independently. Before this correction,
`ManagementBoot` returned an error for confirmed additive state; the wrapper loops
synchronously before network_init/Dropbear/app. An unexpected reboot after keep
would therefore deny remote entry even though the vendor DHCP JSON is unchanged.
An instruction to remove the hook before keep does not make a bad ordering or
crash window safe. This is a concrete recovery defect, not native acceptance.

Root authorized this narrow source correction under s14; it is now implemented
in `helper/internal/recovery/canary_linux.go`, with focused tests in
`helper/internal/recovery/management_boot_test.go`:

```text
retain existing refusal of any pending primary transaction
read and validate matching additive authorization/profile
if confirmed OR expired:
    exact Adapter.Cleanup()       # only owned additive resources
    return its result            # allow unchanged vendor startup on success
otherwise:
    retain existing bounded boot-allowance/worker logic
```

`Adapter.Cleanup` already invalidates previous-boot kernel ownership, refuses a
foreign surviving link/policy and preserves a pre-existing module. This branch
must not recreate a confirmed additive endpoint, write vendor JSON, mint a full
receipt, or bypass a primary transaction. A boot without recreated management
is acceptable for this additive-only terminal state because original vendor
DHCP has not changed; it is not proof of persistent fixed-path acceptance.
The full `pre-app` hook must continue to fail closed on invalid receipt,
unready management or worker failure. Missing/corrupt metadata and cleanup
failures remain errors; the corrected install order above avoids the ordinary
missing-active installation window without inventing recovery from corrupt state.

Focused tests cover confirmed and expired
authorization in same/new boot; unchanged vendor bytes; no worker start or
deadline renewal; exact owned link/routes/rule removal; preservation of a module
the trial did not load; repeated terminal cleanup success; foreign link/policy
and busy-module failures; primary pending refusal before any cleanup. Existing
boot-allowance/worker-loss tests remain and full pre-app code is unchanged.
Fake-kernel checks establish this branch's semantics;
the later real boot trial establishes installed startup behavior. This does not
require repeating task034's already completed broad review.

## What these trials still cannot accept

The first canary runs udhcpc. A fresh connection after one normal boot does
not prove explicit renew/rebind, installed lease-hook behavior, link down/up,
owner reapply or the jetdhcpc unit. Design those exact native actions from the
installed owner before submitting them; never run the whole RkLunch or stop
script as a narrow test. Preserve parent/default and unrelated configuration.
Repeat required network/media proofs on all four actual devices, including
`46793b0d4c8d45d` using jetdhcpc, before recording their acceptance.

The full pre-app hook and external receipt transition remains a separate
reviewed safety checkpoint after those proofs. Installing a full wrapper before
a valid receipt can block unchanged vendor startup on an unexpected reboot;
an instruction to avoid reboot does not close that crash window. Before any
main trial, root must review a boot-safe transition with explicit authority for
the no-primary-pending installation state (for example, a narrowly validated
transition marker allowing owned additive cleanup and unchanged vendor startup
until full acceptance is activated). It must not mint an acceptance receipt or
bypass a pending primary transaction. Prove interruption at each receipt/hook
installation boundary and preserve full-hook fail-closed behavior for actual
primary transactions. This checkpoint is not implemented by the additive
terminal-state fix and does not block the first runtime bootstrap. Primary arm
requires every native receipt field and
confirmed additive state. Only then follow the helper's unchanged-primary
expiry/reboot/restore and failure-injection trials, and later the real primary
target trial. No target address, vendor network RPC, primary promotion or igc1
physical-path acceptance follows from a successful additive canary alone.
