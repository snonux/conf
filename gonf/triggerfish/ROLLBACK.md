# Router rollback controller

The recovery controller and wg2 bootstrap were deployed on 2026-11-05.
Two unconfirmed 180-second trials expired through native root cron; fresh old-LAN
SSH verified restored/no-pending state and absence of wg2's file/interface.
The first acceptance attempt hit a startup handshake delay and was left
unconfirmed. After recreating only the controller's temporary wg2, a new
post-activation handshake and held fresh old/VPN SSH receipts passed; token
`t14-vpn-accept2-20261105` was confirmed. Final fresh old/VPN reconnects passed.
Controller routing/AllowedIPs remain restricted to `10.99.0.1/32`; DNS, default
route and wg1 were preserved. Phone, real reboot and igc1 recovery are untested.
The resolver changes below are prepared, not installed; no physical profile
has been activated. Task l14 implements recovery; t14/u14 own bounded live proof.
The inactive profile renderer supplies configuration input. Physical cable and
recovery-path acceptance gates remain in the IQ router plan.

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

The next reviewed live step is limited to the router's `shared-igc0` profile:

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
   and VPN reconnects must show restored/no-pending state, original physical
   address/route/PF/forwarding/service state and exact saved resolver file/
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
restores files plus captured runtime state and can be repeated after failure.
It recreates only the physical interfaces/routes it owns; no global route flush
or tunnel reconfiguration occurs. Runtime/persistent address drift is preserved
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

PF anchors and runtime-only tables are refused before arming: their independent
loaders are outside this recovery controller. Profiles use floating state policy
and only loopback skip; unexpected persistent if-bound policy or skipped physical
interfaces also refuse. Table declarations come from the saved parsed file,
while dynamic contents and live rules are captured separately. IPv6 address
recovery restores captured addresses/prefix lengths and re-enables the original
physical autoconfiguration policy; SLAAC lifetimes are then maintained by the
OS. The timed/boot canaries still need to establish actual remote recovery and
service behavior; offline fixtures cannot establish that outcome.

`checks/rollback.ksh CONTROLLER PROFILE_DIRECTORY` runs on OpenBSD under root
using a disposable rewritten filesystem and mocked mutation commands. It uses
real configuration parsers and covers missing readiness/keys, changed candidate,
template rejection, partial failure, timed restore, idempotence, drift, fresh
socket confirmation, cached boot flags and retained igc1 recovery. Native syntax
and fixture success are preparation evidence, not a deployed timer/reboot test.
