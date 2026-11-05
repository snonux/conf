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

## Remaining gate

Identify the actual Ethernet driver and its receive-filter behavior before
another network trial. Pinned kernel/rootfs capability, including a possible
IPVLAN alternative sharing the physical MAC, is source evidence only until
installed support and a reviewed bounded native test establish it. Do not
change macvlan mode or enable parent/global promiscuity or ARP changes blindly.
Boot recreation, DHCP-owner reapply, all other devices and primary-network
transactions remain unproved.

Private evidence is under `~/.local/state/conf/s14-jetkvm-runtime-canary-v16-20261105/`
and `s14-jetkvm-runtime-canary-v19-20261105/`, with separate postexpiry audits,
terminal metadata archive and bounded capture timings. Credentials, key/shadow
values, raw application configuration, video frames and private outputs are
not reproduced here.
