# Router rollback controller

The recovery controller and wg2 bootstrap were deployed on 2026-11-05.
Two unconfirmed 180-second trials expired through native root cron; fresh old-LAN
SSH verified restored/no-pending state and absence of wg2's file/interface.
The first acceptance attempt hit a startup handshake delay and was left
unconfirmed. After recreating only the controller's temporary wg2, a new
post-activation handshake and held fresh old/VPN SSH receipts passed; token
`t14-vpn-accept2-20261105` was confirmed. Final fresh old/VPN reconnects passed.
The router-only `shared-igc0` profile subsequently passed a 300-second native
cron-expiry trial (`t14-shared-expiry3-20261105`), including original static
addresses/routes, managed SLAAC ownership, PF rules/table presence, service and
resolver restoration. Distinct token `t14-shared-accept3-20261105` passed fresh
old/new/VPN socket proofs, DNS/ntpd resolution, public IPv4 egress and host-health
checks, then confirmation. No rollback is pending.

The laptop's root-private `/etc/wireguard/wg2.conf` is enabled through
`wg-quick@wg2`, with routes to the router VPN, lab, recovery and two Shelly
addresses. Its DNS, default routes, old management IPv4 and wg1 are unchanged.
The automatic upstream DHCPv6 host lease rotated during this session; it was
recorded separately from migration changes. The phone remains dormant.
The u14 timed and actual reboot canaries also passed, followed by a confirmed
default reapply (`u14-final-20261105`). Host renumbering, forwarded new-host NAT
and physical igc1 recovery remain later gates. No AP or cable changes occurred. Task l14 supplies
the recovery controller; t14/u14 own its bounded native deployment and proof.

`VPN.Candidate`, registered separately as an operational task, reads the private
logical secret `paths.TriggerfishSecret("hostname.wg2")` and stages only
`router-profiles/vpn-bootstrap/hostname.wg2`. This candidate contains the router
10.99.0.1/24 address, UDP51822, one private key and distinct authenticated laptop
10.99.0.2/32 and reserved phone 10.99.0.3/32 peers. The phone remains dormant
until separately configured and tested. Keys and client configurations stay
outside Git; staging does not create an interface.

The `vpn-bootstrap` trial resolves the initial dependency on an already tested
wg2 before physical-profile activation. It uses this same journal, timer and
early boot hook, but snapshots only the absence of `/etc/hostname.wg2` and wg2.
Arming and activation refuse either pre-existing object. Activation installs
only that file and creates a trial-tagged wg2; physical interfaces, PF, services
and existing tunnels stay unchanged. Restoration removes only the introduced
file/interface; unexpected file drift or a foreign interface description refuses
destruction and leaves recovery pending. Boot restoration removes the pending
hostname file before netstart; the previous kernel's interface cannot survive
the reboot. Fresh `old` and `vpn` socket receipts plus controller transport
acceptance are required for confirmation. The VPN receipt requires server
10.99.0.1 and client 10.99.0.2. The laptop must also independently verify the
actual route and recent WireGuard handshake before setting service acceptance.

First exercise expiry, fresh old-LAN reconnect and wg2 removal, then repeat and
confirm with measured receipts. Only an accepted bootstrap can supply the real
wg2 readiness dependency for `shared-igc0`. A disposable fixture exercises the
boot-file restore; it does not establish actual reboot recovery. Keep such
unperformed acceptance explicitly recorded.

## Bounded bootstrap execution plan

This was the review boundary before live Setup. Use distinct tokens and
`window_seconds:180` for both trials. Do not write physical-profile readiness
or activate a physical profile during this bootstrap. Reuse the protected
prepared keys; never render the hostname candidate or client private key to a
terminal/log. Keep the phone peer dormant.

1. Establish a fresh authenticated SSH connection to literal `192.168.1.2`
   with port 22 and multiplexing disabled. Recheck old `.1.2` and `.11.2`, wg2
   file/interface absence, no pending journal, active cron, the native rc hook
   anchor and no controller overlap with `10.99.0.0/24`. Capture controller
   addresses/routes, existing wg1, DNS and default route as the comparison
   baseline. Verify the trusted router host key against the existing old-LAN
   identity; use that same identity via `HostKeyAlias=192.168.1.2` for VPN SSH.
2. Through gonf push with `-privilege=doas`, explicit `-p 22` and disabled
   multiplexing, run `triggerfish_rollback_setup` then
   `triggerfish_vpn_candidate`. Setup changes only recovery machinery (helper,
   early rc hook and cron); candidate staging creates no interface. Verify
   the installed helper syntax, hook position and root cron before arming.
3. Supply nonsecret `GONF_ROUTER_TRIAL` JSON for the expiry token/profile/window;
   run `triggerfish_rollback_stage`, inspect its inactive digest/status, then
   `triggerfish_rollback_arm`. Verify the pending marker and persisted deadline
   before `triggerfish_rollback_activate`. The stage prerequisites restage
   inactive physical candidates only; they do not activate those profiles.
4. Bring up the prepared controller wg2 with only the temporary
   `10.99.0.1/32` route needed for this bootstrap. Preserve controller DNS,
   default route and wg1. Within the deadline, measure a recent authenticated
   handshake, route selection/source `10.99.0.2`, and fresh SSH connections
   to both `192.168.1.2` and `10.99.0.1`. Keep those sessions held while their
   measured tuples are recorded by the gonf proof task if time permits.
5. Deliberately omit confirmation. Allow cron to expire the router trial
   (180 seconds plus at most 60 seconds tick granularity). Establish fresh
   old-LAN SSH; require restored/no-pending status and absence of both
   `/etc/hostname.wg2` and runtime wg2. Compare old aliases and physical/PF/
   service state to the baseline. Stop if any required check fails.
6. Restage with the distinct acceptance token, arm, verify deadline and
   activate. Recheck handshake/route and hold two fresh old/VPN SSH sessions.
   Feed each session's measured `SSH_CONNECTION` tuple and `proof_role` to
   `triggerfish_rollback_proof` through gonf while that socket remains open.
   Only after transport and baseline comparisons pass, set
   `services_verified:true` and run `triggerfish_rollback_confirm` before
   expiry. Require confirmed/no-pending status and fresh old/VPN reconnects.

On an interruption or failed transport check, leave confirmation unset and
observe automatic expiry through a new old-LAN session. Revert only the
introduced controller wg2 when abandoning the bootstrap. Actual reboot
recovery and igc1 physical recovery remain unproved by this experiment.
The full shared profile still requires the separately reviewed armed
resolver/resolvd transition recorded in IQ HANDOFF before activation.

## Router reboot canary (u14; verified 2026-11-05)

The sequence below completed natively. `u14-expiry-20261105` removed its alias
and comment through root cron. `u14-reboot-20261105` rebooted with an 1800-second
deadline; a changed boot time and `restored-at-boot` were observed before that
deadline, with no pending rollback. Protected file hashes/modes, static routes,
IPv6 policy, PF policy/named tables/contents and service state matched the
accepted baseline. Fresh old/new/VPN SSH, DNS and public IPv4 passed. The final
default reapply was confirmed and contains no canary artifacts. Kubernetes,
NFS and Gogios remained healthy. This does not establish any physical igc1 path.

The fixed temporary address `10.20.0.254/24` is reserved for this router-only
test, outside all planned host/JetKVM addresses and DHCP bindings. Explicit
`boot_canary:true` in a `shared-igc0` trial adds that alias and an ntpd comment
to the inactive candidate. Other profiles and default input never add them.
Every rendered lab hostname file also explicitly disables IPv6 autoconfiguration
and temporary addressing, preserving the accepted runtime policy across boot.

`RebootCanary` is Operational and refuses default/other-profile input. The
single recovery helper additionally requires an active unconfirmed canary,
unchanged live/candidate files, intact snapshot, more than five minutes left,
active cron, empty captured dynamic PF tables, and currently established
old/new/VPN proof sockets. It refuses queued/running upgrades. Only then does
it request a normal reboot, leaving the pending marker intact.

The reviewed live plan must complete these steps before claiming boot recovery:

1. Verify old/new/VPN access, confirmed shared state, unused reserved alias,
   current host/service health, empty dynamic PF tables, no queued/running
   upgrade, and the native mount/rc anchors. `/etc/gonf` is on root and local
   `/usr` mounts before the hook on this host; the hook tools are available.
2. Stage/arm the ordinary updated shared profile through gonf under a fresh
   300-second token. Verify its explicit persistent IPv6 policy, fresh held
   old/new/VPN sockets, DNS/egress and existing health, then confirm. No host
   addresses or production cabling change.
3. Capture that accepted baseline. Stage a distinct 180-second shared trial
   with `boot_canary:true`, bind readiness to its frozen digest, arm and
   activate. Prove the extra address and comment really exist in live and
   persistent state, along with fresh old/new/VPN access and service health.
   Deliberately omit confirmation. Native cron must remove the canary and
   restore the accepted baseline; require fresh reconnects and no pending state.
4. Repeat the canary with a distinct token and 1800-second window. Hold three
   fresh authenticated sessions while recording their socket proofs. Recheck
   live canary files, baseline boot time and recovery deadline, then invoke
   `triggerfish_rollback_reboot_canary` through gonf. A disconnected command
   alone is not reboot or recovery evidence.
5. Reconnect through old LAN, then wg2 with a new handshake. Require a changed
   native boot time, `restored-at-boot` with no pending rollback **before** the
   original deadline, absent canary alias/comment, exact accepted persistent
   file hashes/static addresses/routes/PF tables, intended IPv6 policy and
   service state. Check router IPv4 egress and existing cluster/storage/monitoring.
   wg2 keys/peers remain unchanged. The router-local recovery gateway does not
   prove a physical igc1 path.
6. Reapply the default profile under another guarded trial and confirm only
   after fresh old/new/VPN and service checks. End confirmed/no-pending with
   no canary artifacts; the normal aggregate still excludes Operational tasks.

The disposable native fixture mocks shutdown and mutation commands and tests
reboot refusal/guard logic plus file restoration. It cannot prove a real reboot.
Native evidence is retained privately under the t14/u14 preparation state;
the fixture remains only a guard/logic test and never requests a real reboot.

## Shared-profile resolver transaction and acceptance plan

Physical profiles snapshot `/etc/resolv.conf` (including absence and mode),
resolvd runtime flags/running state, and persistent rc.conf.local before arming.
Symlinks, including dangling symlinks, refuse before any pending marker.
Activation stops resolvd with the other owned services, disables it persistently,
starts the configured Unbound, atomically switches the router resolver to only
`nameserver 127.0.0.1`, and then starts ntpd. Bootstrap never enters this branch.
Restore uses saved runtime flags when restarting resolvd, restores persistent
flag drift afterward, then restores exact resolver-file bytes/mode after any
service-generated rewrite. The early boot hook clears cached `resolvd_flags`
before reparsing restored rc configuration. Setup upgrades the exact previous
bootstrap hook only with no pending trial; altered hook blocks refuse.

The reviewed router-only `shared-igc0` activation sequence is:

1. Recheck fresh old/VPN SSH, confirmed/no-pending bootstrap, handshake,
   controller route and unchanged physical cabling. Capture router running and
   persistent address/route, PF, forwarding, DNS and service state. Recheck
   controller DNS/default/wg1 and host/service health (including documented
   pre-existing exceptions). Inspect ARP/host/controller inventory for `.11.2`
   clients and new-prefix collisions; retain `.11.2`. Review r14 dependencies
   specifically for router-only staging: all production hosts/services still
   use their old addresses/routes, no endpoint or service bind is promoted,
   and TemporaryFlows remains empty. Any discovered dependency blocks readiness
   until resolved; preparation tasks alone are not runtime acceptance.
2. Through gonf, install the reviewed updated helper/hook with Setup; verify
   native syntax, exact early hook and root cron. Stage `shared-igc0` with a
   new token and a 300-second window. Obtain its frozen digest. Only after the
   preceding measured inventory/dependency review, use gonf Readiness with
   that exact digest and explicit reviewed assertion. Arm; inspect persisted
   deadline/pending state before activation. No DHCP offers on shared igc0.
3. Keep the existing controller VPN `/32`; temporarily add only
   `10.20.0.1/32` to this peer's AllowedIPs and controller route for new-gateway
   SSH via source `10.99.0.2`. Preserve controller old IP, DNS/default/wg1.
   Activate through gonf. Check expected old/provisioning/zone aliases on igc0,
   local ULA only, no native IPv6 default/autoconfiguration, recovery prefix
   on igc1 without overlap, IPv4 forwarding on/IPv6 forwarding off, loaded PF
   policy, running Unbound/ntpd and stopped/disabled resolvd/dhcpd/rad.
4. Before confirmation, prove fresh held old `.1.2` and new `10.20.0.1` SSH
   (new via the authenticated VPN), plus fresh `10.99.0.1` SSH. Verify selected
   routes/sources and post-activation handshake. Use native system resolution
   for public A records including `pool.ntp.org`, direct local-Unbound queries,
   ntpd peer resolution and the router's public IPv4 egress. Assert resolver
   file is localhost-only and controller DNS/default/wg1 remain unchanged.
   Compare existing host/k3s/storage/monitoring reachability to the baseline.
5. First leave confirmation unset and observe native cron expiry. Fresh old
   and VPN reconnects must show restored/no-pending state, original static
   addresses/routes/PF/forwarding/services, native SLAAC ownership and managed
   prefixes with live lifetimes (privacy suffixes may rotate), and exact saved resolver file/
   persistent flags with original resolvd runtime state. wg2 must survive this
   physical-profile restore. Remove only the temporary new-gateway `/32`
   controller route/AllowedIP after the experiment.
6. If expiry passes, repeat with a distinct token/frozen digest/readiness.
   Repeat the preceding checks; record held old/new socket receipts through
   gonf Proof, then set service acceptance and Confirm before expiry. Recheck
   fresh old/new/VPN access and no-pending state afterward. Leave host aliases,
   canonical DNS, service binds and mesh endpoints for their own guarded steps.

This router-only acceptance does not establish same-interface forwarding/NAT
from a migrated lab client, application ACLs for new sources, actual boot
recovery, physical isolation, phone transport or igc1 cable recovery. Those
require their own bounded canary/native or physical gates before relying on
them. No cable action belongs in this transaction.

Register `Rollback{}` with prefix `triggerfish_rollback_` on the triggerfish
cluster, and attach `RouterTrial{}` to its inventory. All methods are operational;
no normal aggregate selects them. Empty trial data refuses stage/arm/activation.
`Setup` installs `/etc/gonf/router-rollback`, an early `/etc/rc` hook and the
root cron job. It refuses replacement while a trial is pending. The hook is
inserted after local filesystems/root become writable and before sysctl, PF or
network startup. It restores files and explicitly clears/reparses cached rc
service variables. `rc.local` and `rc.firsttime` are too late for this contract.
The native anchor is checked each time; an upstream rc change must be reviewed.

The controller entrypoint parses `GONF_ROUTER_TRIAL` with `TrialFromEnvironment` before
registering inventory. Input is nonsecret JSON; missing input keeps the zero
default, while unknown fields, trailing data and invalid profile/token/window or
proof tuples return a payload-free error. The composition root supplies that
value through `RegisterWithRouterTrial`; ordinary `Register` stays unchanged.
`triggerfish_vpn_candidate` stages the private bootstrap input separately;
neither ordinary registration nor a routine aggregate activates a trial.
For example, a trial starts with `{"profile":"vpn-bootstrap","token":"t14-vpn-expiry-20261104","window_seconds":180}`.
Proof calls add `proof_role` and the measured `ssh_connection` from a fresh held
session. Confirmation sets `services_verified` only after controller checks.

`Stage` accepts `shared-igc0`, `post-rack` or `post-ap`, freezes its files and
validates PF, Unbound and ntpd natively. Enabled DHCP uses OpenBSD's actual
`dhcpd -n -c FILE` syntax. Hostname input accepts only the renderer's address/up
vocabulary and rejects commands/autoconfiguration. Final address retirement is
not supported here. No stage operation changes live addresses or services.

`Arm` requires the stage's token/profile/window (180–1800 seconds), active root
cron, the checked boot hook, a real provisioned/running wg2 and reviewed dependency
input. It saves running addresses, physical gateway routes, forwarding sysctls,
PF rules/tables/enabled state, service runtime flags/running state, and persistent
files separately. `/etc/gonf/router-owner` transfers ownership away from legacy
`Base.Network` and remains after restore/confirmation. That legacy method must
be operational and refuse any invocation while this marker exists. Routine
reapply therefore cannot overwrite a restored current-cabling profile.

Readiness is machine input from the later dependency/VPN workflow, not another
human gate. That workflow installs a root-owned mode-600 file at
`/etc/gonf/router-readiness/PROFILE` with exactly these lines:

```
candidate-sha256=THE_DIGEST_RETURNED_BY_STAGE
provisioning-clients=captured-inventory-no-known-clients
dependencies=r14-reviewed-current-stage-only
wg2=provisioned-and-tested
old-management=fresh-authenticated
current-cabling=PROFILE
```

`Rollback.Readiness` writes this receipt only with explicit `readiness_reviewed`
and `candidate_sha256` input. That assertion means the controller has measured
the accepted wg2 transport, fresh old management, captured provisioning-client
inventory and dependencies for this precise stage. "No known clients" is bounded
by the captured host/controller/ARP inventory; it is not proof about unseen
devices. Keep .11.2 and revisit readiness if any client/dependency is discovered.

Do not create readiness with unresolved clients/dependencies in the inventory. The receipt
binds the exact frozen candidate; it does not discover dependencies or claim
they have passed. The controller never copies a wg2 template, creates a key, or
alters wg0/wg1/wg2. Missing real-form wg2 private/two peer keys or a running
10.99.0.1 gateway refuse arm/activation. The VPN workflow must verify identities,
AllowedIPs, handshake and recovery routing before supplying readiness.

`Activate` requires an unexpired armed trial and rechecks the timer/readiness.
It marks the journal before its first write. Prefixes are removed before they
move between physical ports; 192.168.1.2 remains on igc0 and existing 10.98.0.1
stays on igc1. It stops/changes only dhcpd, unbound, ntpd, rad and resolvd, preserves
unrelated sysctl keys, and binds Unbound at native
`/var/unbound/etc/unbound.conf`. It flushes PF states when switching policy so
old states cannot bypass new rules. Fresh connections are required afterwards.
An interrupted or partially failed activation remains pending for the timer.

The root cron `tick` restores expired trials independently of SSH, within one
minute of the persisted deadline. It can terminate an overrun activation before
restoring. A reboot with any pending marker immediately restores persisted
files before networking starts, regardless of the deadline. Explicit `Restore`
launches a detached attempt, writing a root-private
`/etc/gonf/router-restore.log`, so PF reload cannot terminate its delivering
SSH session and the recovery attempt together. Require restored/no-pending
`Status`; launch success alone does not prove completion. Failed attempts keep
the journal armed for cron, and can be repeated.
If a later inactive stage refused before arming, `RestorePrevious` can name
the previous restored trial token. It verifies the snapshot, archives the
inactive stage, and queues that original restore; it refuses a pending or
confirmed previous transaction. It never constructs a replacement baseline
from the partially restored live state.
It recreates only the physical interfaces/routes it owns; no global route flush
or tunnel reconfiguration occurs. Gateway restoration accepts an already exact
route, including one recreated by SLAAC between checking and adding it; a failed
add without that exact route still refuses. Runtime/persistent address drift is preserved
separately instead of assuming saved hostname files reproduce runtime state.
Post-rack restore retains the saved igc1 recovery address and its PF permissions.
The ownership marker remains so normal reapply cannot undo recovery.

After automated fresh authenticated connections, supply `ProofRole` (`old`,
`new`, or `recovery`) and that remote session's `SSHConnection` four-tuple in
`RouterTrial`, then invoke `Proof`. The helper checks the actual established
socket and rejects tuples present before activation. Old/new proof addresses
are 192.168.1.2 and 10.20.0.1; post-rack/post-AP also need 10.98.0.1. After the
existing controller's service checks pass, set `ServicesVerified=true` and invoke
`Confirm` with the same token. No extra human approval is introduced. Confirmation
cancels the pending restore; it does not retire old/recovery addresses.

PF anchors and unrecognized runtime-only tables are refused before arming:
their independent loaders are outside this recovery controller. The narrow
exception is an optimizer-shaped `__automatic_HEX_NUMBER` table that is const
and referenced by a running rule. Its captured contents become an explicit
const declaration in the saved runtime rules, which are parsed natively before
arming. Mutable or unreferenced lookalikes refuse. Across a cold boot, acceptance
compares these internal tables by contents and rule references; their opaque
names can change. Named table identity and contents remain exact.
Profiles use floating state policy
and only loopback skip; unexpected persistent if-bound policy or skipped physical
interfaces also refuse. Table declarations come from the saved parsed file,
while dynamic contents and live rules are captured separately. Restore also
removes persistent tables introduced by the frozen candidate but absent from
the snapshot; reloading old rules alone leaves those tables in the kernel.
Unfamiliar table names refuse cleanup. IPv6 recovery
restores captured static addresses/prefix lengths and both autoconfiguration/
temporary-address policy flags. Originally disabled physical IPv6 families are
disabled again, removing trial-introduced link-local addresses. Captured SLAAC
addresses are never recreated as static aliases: after restoring PF, native
solicitation must recover their managed /64 prefixes with positive lifetimes
within ten seconds. Privacy suffixes may rotate. Activation disables both
autoconfiguration policies, drains in-flight proposals within a bounded loop
and refuses unexpected remaining IPv6 addresses before acceptance.
The timed and actual reboot canaries passed as recorded above. Physical
recovery remains a later gate; offline fixtures cannot establish it.

`checks/rollback.ksh CONTROLLER PROFILE_DIRECTORY NAT66_EGRESS_HELPER` runs on OpenBSD under root
using a disposable rewritten filesystem and mocked mutation commands. It uses
real configuration parsers and covers missing readiness/keys, changed candidate,
template rejection, partial failure, timed restore, idempotence, drift, fresh
socket confirmation, cached boot flags and retained igc1 recovery. Native syntax
and fixture success are preparation evidence, not a deployed timer/reboot test.

During v14 native host testing on 2026-11-05, the first forwarded wg2 SYN
was rejected by the router's egress default block despite matching its ingress
permit. Profiles now tag only explicitly permitted transit with `gonf_transit`
and allow IPv4 egress carrying that internal tag, following OpenBSD's
[packet-tagging pattern](https://man.openbsd.org/pf.conf.5#tag).
Ingress source/interface/port restrictions and the default block remain.
No NAT is added between wg2 and the lab. All four candidate PF files passed
native parsing. The corrected shared profile was separately accepted through
the guarded gonf journal (`v14-transit-20261105`), with fresh old/new/VPN SSH,
DNS, NTP, public IPv4 and protected file/service/controller checks. Against
an absent lab canary, TCP22 proceeded to neighbor resolution while TCP23
received immediate block-return. A subsequent f3 retry established fresh SSH through wg2 from 10.99.0.2
to 10.20.0.133:22. Full host timed/boot recovery remains v14 acceptance;
the initial absent-destination probe alone did not establish it.

## o34 IPv6 stages (NAT66 for Wi-Fi)

The CPE delegates no prefix (DHCPv6 IA_PD answers NoPrefixAvail), so Wi-Fi
IPv6 uses NAT66. Two stageable profiles share the wifi-first wiring:

- `wifi-ipv6-uplink`: igc0 `inet6 autoconf -temporary` (one stable RFC 7217
  address and the IPv6 default via the CPE), router-originated IPv6 only,
  `ip6.forwarding=0`, Wi-Fi unchanged.
- `wifi-ipv6-nat66`: adds `fd02:ca0b:f1cb:30::1/64` on igc2, `/etc/rad.conf`
  for igc2 only, `rad` enabled, `ip6.forwarding=1` and
  `match out on igc0 ... nat-to <nat66_egress>`, placed before every quick
  uplink rule (a quick rule ends evaluation, so a later match never
  translates; live trial `o34-nat66-20261109-b` showed exactly that, and
  Stage now refuses such an order).

`nat-to (igc0)` is deliberately not used: a dynamic interface pool
round-robins over every non-link-local igc0 address, including the lab ULA.
`triggerfish_nat66_egress` installs `/etc/gonf/nat66-egress` and a per-minute
root cron job that keeps `<nat66_egress>` at exactly ONE address, because a
table pool also round-robins: of igc0's stable slaacd GUAs (not temporary,
deprecated, tentative, duplicated or detached) the one with the longest valid
lifetime, then preferred lifetime, then the lowest address. After a CPE
renumbering only the new prefix is refreshed, so it wins at the first RA that
announces it, even without a deprecating announcement for the old one. The
helper acts only while the loaded rules contain `nat-to <nat66_egress>`, and
only while holding the journal lock `/etc/gonf/router-lock` (it skips the run
if a rollback operation holds it). Every journal operation, including the
cron `tick`, waits up to 5 s for a live holder; `tick` terminates only an
overrun `router-rollback activate` and otherwise waits, so an expired trial
is restored even when the helper runs in the same minute. A lock whose pid
is not written yet counts as busy and is removed only when over a minute old
(owner died between mkdir and the write). A `persist` table outlives a reload that no longer declares it, so
activation of a profile without NAT66 kills an orphan `<nat66_egress>`;
restore removes the candidate's table as before. Without that, the next
snapshot would refuse the runtime-only table. Arm refuses the NAT66 profile
without the helper. Failures go to syslog (`nat66-egress`, daemon.err).

The helper validates placement (SLAAC only on igc0, Wi-Fi ULA only on igc2,
rad only for igc2, forwarding only for NAT66, the egress table present).
Uplink RAs are accepted only from the CPE's `fe80::1`; other RAs on igc0 are a
logged drop. A host on the shared igc0 wire can still spoof that address.
Activation enables forwarding only after the candidate PF, solicits on igc0
after that PF admits RAs, and starts rad last. Acceptance admits stable
slaacd addresses only where the candidate enables SLAAC; temporary addresses
or the temporary flag refuse. `/etc/rad.conf` joined the owned file set: timed
and boot restoration remove it, and runtime restoration stops rad, restores
forwarding, drops the Wi-Fi IPv6 family and relearns a saved SLAAC prefix
(restoring to the uplink stage). A snapshot taken before rad.conf ownership
still restores candidates without rad.conf and refuses a NAT66 candidate.
The fixed reboot canary is also accepted for `wifi-ipv6-nat66`, so its
persistence can be proven across a real reboot. Fixture coverage lives in
`checks/rollback.ksh` (stage refusals, temporary-address refusal, ordering,
timed/boot/legacy restoration, canary).

## p34 Shelly HTTPS (temporary TCP443 next to TCP80)

The Shelly plugs serve native HTTPS with private-CA certificates (IQ
`hosts/shelly-plug.md`). While the controllers move to HTTPS, only
`wifi-ipv6-nat66` and the g24 `camera-igc3` built on it permit
`port { 80, 443 }` for exactly the existing
controller pairs: pi0/pi1 to both plugs, f0-f3 to the fans plug, and laptop
wg2 `10.99.0.2` to both (`shellyPorts` in `profiles.go`). Every other profile
renders byte-identical, with port 80 only. pfctl merges the fans-plug sources
into `__automatic_*` tables, so verify the loaded rules by expanding those
tables, not by counting lines. Trial `p34-shelly443-20261109-a` was confirmed
on 2026-11-09. Port 80 is removed in a later p34 trial, once every caller is
accepted on HTTPS.

## g24 camera port (camera-igc3)

`camera-igc3` (`camera.go`) is the live `wifi-ipv6-nat66` profile, p34
Shelly TCP80+443 permits included, with the camera zone moved to its own
port: `10.40.0.1/24` and its connected route leave igc0 for igc3, which stays
uncabled until the human camera gate (m24). Differences from NAT66, and
nothing else: `hostname.igc0` loses the camera alias, `hostname.igc3` gains
it, PF names `cam_if = "igc3"` (anti-spoofing follows) and adds

```
block drop in log quick on $cam_if
block drop out log quick on $cam_if
```

The catch-all `block return log all` already denies the zone; the quick
rules also beat non-quick permits without an interface (`pass out inet
tagged gonf_transit`) and give pflog a distinct rule. A later recorder or
update permit (l24/n24) must be a quick rule before them. `<outdoor>`
(10.40.0.11-14) is the static allocation, with no DHCP; PF loads an
unreferenced const table only once a rule uses it. `<recorders>` and
`<camera_updates>` stay empty and unreferenced: no NAT, internet, router
service, lab or Wi-Fi path, and no IPv6 (rad serves only igc2).

router-rollback checks the camera zone for every profile (`validate_camera`):
`10.40.0.1` on the port the PF macro names (`camera_port`: igc3 for
camera-igc3 and post-ap, igc0 otherwise), no `pass` rule and no translation
mentioning `$cam_if`, `10.40.`, `<outdoor>`, `<recorders>`,
`<camera_updates>` or the dedicated port, no `inet6` line on a dedicated
camera port, and the two deny lines for camera-igc3. The existing zone check
still refuses the prefix on two ports. Activation runs `ifconfig igc3 -inet6`
for a dedicated camera port (draining keeps link-local addresses), and
acceptance refuses any IPv6 on it, link-local included. Timed, boot and
forward restoration move the prefix back the same way it came: drained from
one port before it is added to the other. All other profiles render
byte-identical. Fixtures in `checks/rollback.ksh` cover the refusals (wrong
port, macro mismatch, both ports, NAT66 moving the cameras, permits by macro,
literal port or update table, NAT by source or table, missing deny, IPv6 line,
rad and DHCP on igc3), activation order, stray link-local removal, the
runtime IPv6 refusal, timed, boot and canary restoration and a forward NAT66
restage.

While igc3 has no carrier, the connected route `10.40.0/24` sits on igc3
with flags `Cn` (no `U`), and `route get 10.40.0.11` still resolves to igc3,
not the default route. A router-originated ping to 10.40.0.11 hit the
explicit deny (`block out on igc3`) in pflog; lab (pi0/pi1) and Wi-Fi SYNs
towards the zone were refused in on igc0/igc2 by the catch-all. Camera
traffic itself is untested until cameras exist.

Live 2026-11-09, under `live-rollout.lock`: trial `g24-camera-20261109-a`
(every staged profile equal to the local render, frozen PF diff exactly the
11 camera lines) was checked and confirmed with fresh old/new proofs via pi0
and VPN. A real reboot canary (`g24-camera-boot-20261109-b`) restored at boot
and brought the zone up on igc3 through netstart, then the plain profile was
reapplied and confirmed (`g24-camera-final-20261109-c`). IQ
`openbsdrouter-plan/NEXT-SESSION.md` holds the receipts.
