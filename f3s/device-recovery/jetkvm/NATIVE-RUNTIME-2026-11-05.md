# JetKVM additive runtime trials — 2026-11-05

Task s14 remains open. **Detached expiry and cleanup passed; a usable fixed
management channel did not. No primary renumbering is accepted.**

Only `e10a0883debf7d4a`, originally `192.168.1.241`, was tested. Its verified
ARMv7/5.10.160 device received the root-private helper binary/profile; `-version`
returned 0.1.0. IQ owns the reserved management rows. The tested shared-igc0
profile used `192.168.11.140/24`, MAC `02:52:53:08:97:97`, gateway `.11.2`,
table 1000 and priority 1140 for controller `10.99.0.2/32`. Vendor DHCP, factory
MAC and original addresses/default remained unchanged. No hook, boot allowance,
primary transaction, management receipt or confirmation was installed/issued.

## Native outcome

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

## Usability failure and bounded packet evidence

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

- `jkrecover`: root-owned0700, 4,399,140 bytes, SHA256
  `1e69b15b9d452a0e1653cb4a7db8dee06d29e391213e09c729d2fe97cb976c95`.
- `profile.json`: root-owned0600, 242 bytes, SHA256
  `2815fbf4031cda9346619e76b9037a29c0ce2f544fdb12a20f8bcb05913a3d91`.

The profile explicitly selects IPvlan and expected factory parent MAC
`30:52:53:08:97:97`; other address/table/rule fields remain the same. The old
locally administered MAC reservation remains unused. Only `-version` ran,
returning0.1.0. Preflight took1.330s and staging2.624s; copy transferred the exact
binary in0.762s. Original-parent comparison passed. Developer access cleanup and
all nine fresh HTTP/WebRTC/identity/version/USB/video/mode/key/closed22 checks
passed. No module, link, netlink, IP/rule, sysctl, ARP, hook, boot allowance,
receipt or primary operation occurred. IPvlan network usability is **untried**.

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
probing or mutation is included in the proposed trial.

## Remaining gate

The [IPvlan L2 candidate](IPVLAN-L2-CANDIDATE.md) is installed as files only.
Root must review a concrete short unconfirmed native trial before loading the
module or creating its link. Typed helper creation explicitly requests L2 and
must observe actual mode/kind/parent/MAC/index before UP; source tests do not
establish native reception. Reuse fresh forced fixed-path authentication/media,
independent expiry and a separate postexpiry audit. Do not change parent/global
promiscuity or ARP policy to make a trial pass. Boot recreation, DHCP-owner
reapply, all other devices and primary-network transactions remain unproved.

Private evidence remains under `~/.local/state/conf/` in the v16/v19 runtime,
v20/v22/v24 read-only and v25 file-staging directories. Credentials, key/shadow
values, raw application configuration, video frames and private outputs are
not reproduced here.
