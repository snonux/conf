# Router rollback controller

Preparation only. The controller is not installed and no live trial has run.
Task l14 implements recovery; t14/u14 own installation and bounded live proof.
The inactive profile renderer supplies configuration input. Physical cable and
recovery-path acceptance gates remain in the IQ router plan.

Register `Rollback{}` with prefix `triggerfish_rollback_` on the triggerfish
cluster, and attach `RouterTrial{}` to its inventory. All methods are operational;
no normal aggregate selects them. Empty trial data refuses stage/arm/activation.
`Setup` installs `/etc/gonf/router-rollback`, an early `/etc/rc` hook and the
root cron job. It refuses replacement while a trial is pending. The hook is
inserted after local filesystems/root become writable and before sysctl, PF or
network startup. It restores files and explicitly clears/reparses cached rc
service variables. `rc.local` and `rc.firsttime` are too late for this contract.
The native anchor is checked each time; an upstream rc change must be reviewed.

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
provisioning-clients=reviewed-none-unlisted
dependencies=r14-reviewed-none-unlisted
wg2=provisioned-and-tested
old-management=fresh-authenticated
current-cabling=PROFILE
```

Do not create readiness while clients/dependencies remain unknown. The receipt
binds the exact frozen candidate; it does not discover dependencies or claim
they have passed. The controller never copies a wg2 template, creates a key, or
alters wg0/wg1/wg2. Missing real-form wg2 private/two peer keys or a running
10.99.0.1 gateway refuse arm/activation. The VPN workflow must verify identities,
AllowedIPs, handshake and recovery routing before supplying readiness.

`Activate` requires an unexpired armed trial and rechecks the timer/readiness.
It marks the journal before its first write. Prefixes are removed before they
move between physical ports; 192.168.1.2 remains on igc0 and existing 10.98.0.1
stays on igc1. It stops/changes only dhcpd, unbound, ntpd and rad, preserves
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
