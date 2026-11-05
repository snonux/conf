# JetKVM additive runtime evidence — 2026-11-05

Task s14 remains open. **First-device IPvlan fixed HTTP/SSH/media and independent
expiry/cleanup passed in v26. Boot, DHCP-owner and fleet acceptance remain open;
no primary renumbering is accepted.**

Only `e10a0883debf7d4a`, originally `192.168.1.241`, was tested. Its verified
ARMv7/5.10.160 device received the root-private helper binary/profile; `-version`
returned 0.1.0. IQ owns the reserved management rows. The initial macvlan shared-igc0
profile used `192.168.11.140/24`, MAC `02:52:53:08:97:97`, gateway `.11.2`,
table 1000 and priority 1140 for controller `10.99.0.2/32`. Vendor DHCP, factory
MAC and original addresses/default remained unchanged. No hook, boot allowance,
primary transaction, management receipt or confirmation was installed/issued.

## Historical macvlan runtime outcome

Two separate unconfirmed generations reached local bootstrap readiness, each
with an immutable device BOOTTIME deadline of floor(uptime)+110 seconds.
The originating controller process exited before expiry. Both actual detached
expiries and subsequent separate read-only audits passed:

- Same native boot, generation, deadline and profile; expired/not-confirmed.
- Owned child/module removed; table 1000 and priority 1140 empty; all owned
  intents cleared. Journals remained for audit.
- Settled parent link/address/routes/rules matched the original pre-trial
  baseline after ignoring only numeric address lifetimes.
- Fresh old-IP authentication, USB/version/video checks passed. Every temporary
  access scope genuinely drained its original enable request and restored false
  developer mode, closed TCP22 and the exact prior empty authorized-key scalar.

The second generation archived the prior terminal metadata privately before
atomically replacing only the authorization input. Existing active/owned/lock
records were retained for the helper's normal expired-generation transition.

## Historical macvlan usability failure and packet evidence

The first fixed SSH attempt timed out. Durable router state showed TCP80 had
established transport, but the client had not recorded its individual HTTP
result. This does not establish authenticated fixed-path access.

The second trial recorded HTTP login timeout before any status and SSH TCP
connect timeout. Device-side reads showed wildcard TCP22 listeners for IPv4
and IPv6 and the intended return route through `.11.2` on `jkrecover`.
Two 40-second nonpromiscuous router captures bracketed the probes and retained
only relevant ARP/zero-payload TCP22 headers, excluding HTTP payload:

- Controller SYNs arrived on wg2 and left igc0 toward the correct child MAC;
  no SYNACK returned.
- Router ARP for `.11.140` elicited replies from both the factory parent MAC
  and the child MAC.
- The child repeatedly asked for `.11.2`; router unicast ARP replies to the
  child MAC did not stop those retries.

These observations suggest failed child-unicast reception, with parent ARP
flux also present. They do not yet identify a NIC/driver defect. The first
trial's TCP80 state could have depended on parent-MAC ARP selection; it is not
proof that the child received unicast. No ARP policy, promiscuity, macvlan mode,
driver, sysctl or network repair was attempted.

The read-only old-path video baseline separately decoded an actual H264 frame
over the exact old UDP pair. The agent client's original 128-packet jitter
buffer was too small for the observed fragmented IDR; a client-only 512-packet
buffer decoded successfully. Server video configuration and timing budgets
were unchanged. No fixed-path media session ran because HTTP/SSH proof failed.

## IPvlan prerequisite and file-only installation

Read-only v20 identified the actual `rk_gmac-dwmac` driver and regular shipped
IPvlan module (24,960 bytes, SHA256
`8e076dc464de7291124530fafca4d83d079471cf34164752927a5938dda4bc39`).
Both modules were unloaded. This is presence evidence, not a loadability result.

Root-reviewed v25 then completed a **file-only** transition on the same first
device. A complete exclusive controller archive was synced; the original seven
files, including terminal journals and stable lock, were renamed together to
`/userdata/jkrecover-macvlan-expired-s14-v25-20261105`. Their exact inode/hash/mode
identities remained unchanged. The new default directory contains only:

- `jkrecover`: root-owned 0700, 4,399,140 bytes, SHA256
  `1e69b15b9d452a0e1653cb4a7db8dee06d29e391213e09c729d2fe97cb976c95`.
- `profile.json`: root-owned 0600, 242 bytes, SHA256
  `2815fbf4031cda9346619e76b9037a29c0ce2f544fdb12a20f8bcb05913a3d91`.

The profile explicitly selects IPvlan and expected factory parent MAC
`30:52:53:08:97:97`; other address/table/rule fields remain the same. The old
locally administered MAC reservation remains unused. At the file-only stage, only `-version` ran,
returning 0.1.0. Preflight took 1.330s and staging 2.624s; copy transferred the exact
binary in 0.762s. Original-parent comparison passed. Developer access cleanup and
all nine fresh HTTP/WebRTC/identity/version/USB/video/mode/key/closed22 checks
passed. No module, link, netlink, IP/rule, sysctl, ARP, hook, boot allowance,
receipt or primary operation occurred. That file-only phase did not establish IPvlan network usability.

Earlier file-only attempts stopped safely: v21 failed a namespace guard before
archive/staging; v23 completed the controller archive but its9,855-byte SSH exec
exceeded the pinned Dropbear2025.88 limit9000. Read-only v24 proved OLD_INTACT,
all seven original identities/hashes, absent archive sibling and unchanged
parent/kernel resources. A local postcheck insertion error was caught; saved
bytes classified offline, then a separately reviewed exact original fresh
postcheck passed all nine checks. v25 used a reviewed lossless gzip/base64 short
exec, validating exact decoded bytes while preserving profile/binary stdin.
It changed no server limit or payload protocol. Private failed-attempt evidence
was retained.

The namespace API was unavailable to v22. It is not claimed verified. File-only
staging did not require namespace admission because it could not change a port.
A first kernel trial still requires global IPvlan and macvlan module absence,
no parent upper links, verified factory MAC and serialized writers; a globally
absent IPvlan module excludes existing IPvlan ports in any namespace. No namespace
probing or mutation occurred in v26.

## v26 IPvlan runtime acceptance and independent expiry

Root reviewed and ran one unconfirmed 110-second additive generation using the
installed hashes above, explicit IPvlan L2 and factory MAC `30:52:53:08:97:97`.
Both global modules and parent upper links were absent before authorization;
parent MAC/OEM module hash and original parent baseline were checked. The pinned
helper validated typed actual kind/L2/parent/MAC/ifindex before UP. No namespace
API verification was claimed.

Fresh fixed HTTP authentication/device identity and root public-key SSH passed
from `10.99.0.2` to literal `192.168.11.140`. The originating RTC disabled
developer access, drained its original enable future to terminal and restored
the exact empty key before fixed RTC takeover. That new session proved an
explicit allowed candidate and actual nominated UDP pair exactly
`10.99.0.2 → 192.168.11.140`, data-channel identity/version/USB/video and one
actual decoded H264 frame. Client jitter capacity remained512, media budget14s
and frame budget5s. It received237 RTP packets/260,645 bytes; nine initial decoder
warnings were retained rather than hidden, and the decoded frame passed within
the existing budget. No server video/control/HID/power setting changed.

These controller BOOTTIME witnesses are exact private observations, rounded to
milliseconds; offsets are relative to origin start110504.974s:

| Witness | Offset seconds |
| --- | ---: |
| Both router captures listening | before origin start |
| Bootstrap Ready | 10.600 |
| Fixed HTTP and SSH proof | 11.715 |
| Exact original-auth cleanup | 11.878 |
| Fixed pair/data/decoded frame proof | 13.174 |
| Origin process terminal, exit0 | 18.999 |
| First fresh fixedTCP80 refusal after expiry | 119.187 |
| Independent observer terminal, exit0 | 155.882 |
| Separate native audit terminal, exit0 | 171.580 |

The immutable device BOOTTIME deadline was24,668,000,000,000ns, identical at the
postexpiry audit. No keep/confirmation, boot allowance, hook or receipt was used.
The observer verified origin PID/birth gone before earliest expiry and three
fresh fixedTCP80 refusals after the original deadline. It did not clean resources.

Both bounded40-second nonpromiscuous router captures bracketed the actual fixed
probes. igc0 recorded `.11.140` ARP replies using the factory MAC, then controller
SYN/fixed SYNACK/ACK. The old locally administered child MAC was absent. Only
selected ARP and zero-payload TCP22 headers were retained, not HTTP/video payload.

A separate postexpiry temporary-auth scope passed all20 native checks: same
boot/generation/profile/deadline, expired/unconfirmed/no allowance, no child or
IPvlan/macvlan modules, empty table1000/priority1140, cleared owned intents, two
parent snapshots matching both preflight and ORIGINAL v16 baseline, unchanged
USB/version and no tentative/DAD failure. Both access scopes genuinely drained
their original enable future and restored false mode/closed22/exactemptykey;
all nine fresh old-HTTP/newRTC/identity/version/USB/video/access checks passed.
All PCs, SSH/capture processes and workers were terminal. Expired journals remain.

This accepts the first device's runtime IPvlan fixed path and detached expiry,
not persistent native recovery. Vendor DHCP/default/original addresses remained
unchanged. No primary or full-acceptance receipt exists.

## Remaining gates

[Confirmed runtime and owner/boot plan](CONFIRMED-NEXT.md) describes the next
separately reviewed slices. Root source `0da9417` added the narrow `canary-cleanup` entrypoint. Reviewed v27
then atomically replaced ONLY the binary with the 4,399,605-byte static artifact
SHA256 `a0857114635d6404e3ad797b96bb2877726ed420c6e38bb257df96911389c14b`.
Profile and all six nonbinary files (authorization, active, owned, worker, lock
and profile) retained exact inode/content identities; the complete old seven-file
v26 store was independently archived on the controller first. Preflight took
1.450s, staging 3.820s and copy 1.154s. Only `-version` ran, returning 0.1.0.
Original-parent checks, exact access cleanup and all nine fresh checks passed.
No cleanup/bootstrap/renew/keep or kernel operation occurred. A confirmed trial
still needs its separately reviewed driver and root-released window. DHCP renew/rebind/owner reapply, bounded boot with
actual early MAC ordering, all other devices and the boot-safe full-hook/receipt
transition remain unproved. Do not broaden ARP/promiscuity/global policy.

Private evidence remains under `~/.local/state/conf/` in v16/v19 historical
runtime, v20/v22/v24 read-only, v25 file staging and
`s14-jetkvm-ipvlan-runtime-v26-20261105/` (including orchestration and separate
postexpiry). Credentials, key/shadow values, raw application configuration,
video frames and private outputs are not reproduced here.
