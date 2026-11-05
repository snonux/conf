# f3 shared-wire pilot: execution proposal

Prepared 2026-11-05 for conf y14. This initial proposal is historical;
[the guest execution record](Y14-GUESTS.md) records accepted native Stage,
and [the next slices](Y14-NEXT.md) cover subsequent guarded work. Root coordination reviews each concrete live slice before
execution. Existing user authorization covers reviewed staged migration;
physical cable gates remain later. All addresses below use the current wire.

At 12:13 UTC root put guest trials on hold for the user's Shelly/Matter full
shutdown window. No guest address trial had started. f3 and both guests had
no pending journals; neither guest had activation gates. Dormant q24 helpers
were installed and watchers initialized/verified. FreeBSD was temporarily
running for preparation; root coordinates its graceful stop during shutdown
and preserves its normal stopped policy. Resume only after root reports the
hosts warm and healthy, then repeat all time-sensitive preflights below.

## Observed starting state

Read-only native checks at about 11:53–11:55 UTC found:

- f3/re0 retains primary 192.168.1.133 and alias 10.20.0.133, default
  192.168.1.1, and laptop 10.99.0.2/32 via 192.168.1.2. Its address journal
  reports no pending transaction. v14 separately accepted its local ULA.
- r2/enp0s5 reports no pending address transaction and routes 10.20.0.133
  directly, selecting source 10.20.0.122. Its unattended service is inactive;
  next timer was 12:45 UTC. Recheck before each pending change/reboot.
- f0 and r0 route 10.20.0.133 through their old default 192.168.1.1. No
  successful new-address transport from old-only referrers is established.
- f3's standalone rocky is running; freebsd and lab guests are stopped.
  rocky/enp0s5 is old-only 192.168.1.123, default 192.168.1.1. Its next
  unattended timer was 11:55 UTC; that observation is already time-sensitive.
  f3 unattended runs at minute 15 hourly. Do not change those schedules.
- f3 has working /usr/local/bin/python3.12 (3.12.15) but no
  /usr/local/bin/python3. The endpoint helper's documented interpreter
  prerequisite therefore needs an explicit reviewed provisioning choice.

Unchanged protections: no address retirement, AP/SSID/controller/DNS changes,
igc1 assertion, production cable move, whole-mesh installer or simultaneous
NFS/CARP writers. Independent task 914 remains untouched.

## First live slice proposed: standalone guest Stage acceptance

Use existing q24 recipes, serially: rocky first, then freebsd. No host primary
promotion, endpoint change or source YAML change belongs to this slice.

1. Recheck f3 and guest unattended activity/next execution, root storage and
   cluster baseline, fresh old literal authenticated SSH, active guest
   interface/profile and exact source-owned recipe preview. Capture guest
   persistent/running addresses, routes, native IPv6 lifetimes, DNS, firewall,
   wg0 semantic state and boot identity privately. Verify guest-specific
   noninteractive administration: rocky uses root SSH, freebsd paul/doas via
   its trusted inventory alias. Do not assume rocky's paul has general sudo.
2. Install only `rocky_guest_network_prepare` on rocky or
   `freebsd_guest_network_prepare` on freebsd. Review the concrete scoped dry
   run first; do not apply either cluster in bulk. Verify deployed bytes,
   syntax, root-only journals and native watchdog/boot ordering. Rocky uses
   the standalone runtime unit without any k3s dependency or operation.
3. Coordinate a bounded canary authorization under the existing recovery
   procedure. Run shared Stage: add .20.123 or .20.139 while retaining old
   primary/default; use disabled physical IPv6 with no unallocated guest ULA;
   install laptop /32 return through .1.2. Never use the isolated profile.
   Prove fresh old/new SSH, public IPv4 connectivity and unchanged wg0.
4. Let an independent trial expire with originating SSH gone. Reconnect via
   old literal IP; compare saved/running state to original, check no pending,
   and execute explicit rollback twice. Then perform a separate unconfirmed
   native guest reboot well before deadline; verify actual new boot identity,
   boot restoration and fresh old access. Recheck unattended before arming
   and immediately before reboot. Never reboot f3 for a guest-only test.
5. After expiry/boot/reapply acceptance, run a distinct Stage, hold fresh
   independent old/new sessions, verify services and confirm. Observe beyond
   its former deadline and run only its scoped Prepare preview/reapply after
   no pending remains. Remove bounded authorization markers.
6. For freebsd, root coordination must first temporarily start the normally
   stopped guest. Restore its normal stopped state with graceful guest/vm
   shutdown after testing; leave vm_list/autostart unchanged. rocky remains
   running and autostarted. These lifecycle actions are not executed by this
   proposal.

Guest firewall/listener rules and the actual return path must pass before any
activation; successful q24 offline checks do not establish those native facts.
Stop and resolve a failing prerequisite through its existing owner.

## Endpoint canary: protocol issues to resolve before deployment

The p24 controller requires the complete persisted referring-host set for a
destination, snapshots all before first apply, and promotes source only after
all accept. For f3 these are f0–f2 and r0–r2; revalidate the complete inventory
privately against current saved configurations. r2 is currently the only
observed referring host with a connected new /24. A r2-only manifest must not
pretend to be complete or commit the f3 source candidate.

The host helper's `snapshot` additionally requires `recovery-approved`, whose
documented meaning is completed native timer/boot proof. That proof is still
absent. The tool presently has no separately scoped bounded-canary entry point
analogous to v14. Resolve this bootstrap contract before writing that assertion;
never label a test authorization as already completed proof.

After that resolution, a bounded r2-to-f3 trial can test independent endpoint
expiry and boot restoration with old/new f3 retained, full saved/runtime semantic
comparison, actual UDP56709 traffic, fresh handshake/counter progress, overlay
IPv4/IPv6 and existing cluster/storage/ingress checks. Native FreeBSD endpoint
recovery needs its own proof too. Such canaries must roll back and leave the
authoritative YAML old until the complete serial stage is eligible.

Keep destination address transactions accepted and archived before endpoint
trials. Do not set address-helper `endpoints-required`: p24 deliberately rejects
that local hook directory because it cannot recover remote referring journals
or the controller. Source/saved/runtime rollback uses p24's independent journals
and durable controller reconciliation, with old/new addresses retained.

## Primary/gateway promotion remains a separate proposal

Before promoting f3 or a guest, establish new-source public NAT through
10.20.0.1 and check every affected service/source guard. f3's trusted_lan PF
table still contains the old LAN/native ULA, while its explicit UDP56709 rule
accepts transport from any source. Do not mistake SSH over wg2 for full native
ULA/service acceptance.

Prove outgoing wg0 transport source selection and old-only peers' return
routing before selecting a new physical primary. A peer can learn a new outer
source endpoint even when its saved endpoint remains old; retaining old aliases
alone does not establish bidirectional transport. If that proof fails, retain
the accepted dual Stage until those referrers have their own reviewed routing
or dual Stage. Never silently expand y14 into an all-host rollout.

Promotion, when separately ready, snapshots accepted dual Stage and restores
that same current-cabling state on expiry/boot. It must retain old IPv4 and
laptop /32 via .1.2. No dedicated recovery .98 gateway or isolated marker is
appropriate on the unchanged wire. Required final evidence includes actual
primary/gateway, fresh old/new SSH, new-source public egress, DNS, wg0 IPv4/IPv6,
service health, independent rollback and later persistence/reapply.

Candidate f3 transport protection for root review: add this source-owned rule
before `pass out quick all` in the existing PF owner, then preview/validate and
apply only to f3 with states preserved:

```pf
block out log quick on re0 inet proto udp from 10.20.0.133 to { 192.168.1.120, 192.168.1.121, 192.168.1.122, 192.168.1.130, 192.168.1.131, 192.168.1.132 } port 56709
```

This is a fail-closed observation guard, not a source-routing fix. It prevents
new-source packets from changing old-only peers' learned endpoints during a
bounded f3 primary trial. The narrow rule is dormant with old-primary transport;
address rollback returns that accepted transport without depending on removal
of this PF guard. Keep the guard in its owner source across reapply. Before
activation, verify existing live states cannot authorize a new .20.133 tuple,
and observe source packets/counters under the old-primary baseline. During the
trial check each old referring peer's handshake/transfer progress and actual
outer source under passive capture. If the guard logs/drops traffic or any
required path fails, explicitly restore the address transaction immediately;
do not edit peer endpoints to make the trial pass. Remove the guard only at a
separately reviewed stage when the relevant new return paths are accepted.

New-source NAT can be tested inside that bounded primary transaction, before
confirmation, using IPv4 HTTPS explicitly bound to 10.20.0.133 and simultaneous
router ingress/egress observation proving translation to 192.168.1.2. Router
configuration remains unchanged. DNS is tested both through the configured
resolver and explicit Pi-hole queries; a controller-resolver limitation is
reported separately. This proposal still needs the concrete PF source diff,
native validation, fresh old/new access and root coordination review before
any production rule or primary change. No such guard is deployed here.

## Local checks actually run

Both existing FreeBSD and Rocky disposable address fixtures passed. The p24
endpoint unittest suite passed 25 tests. These checks did not create interfaces,
modify endpoints, test disconnected native execution or establish guest boot
acceptance. The later native-discovered shell DNS correction is recorded below;
no Go implementation changed.

Native preparation before the hold installed only the existing source-owned
guest Prepare plans. Rocky's scoped dry run showed eight changes; FreeBSD's
showed six. An existing FreeBSD noninteractive shell banner prevented `gonf
push`'s version probe and scp. Preparation instead transferred the exact local
plan, blobs and same-source FreeBSD receiver through tar-over-SSH, then ran
native `gonf apply -n` followed by apply. Shell startup configuration was not
changed. Both native helper syntax checks and watchdog/boot-order checks
passed. Private evidence is under
`~/.local/state/conf/y14-preparation/evidence/`.

The freshly started FreeBSD guest's wg0 already has zero handshakes/received
bytes and failed IPv4/IPv6 overlay probes. Read-only comparison found its
actual public identity differs from the gateway's configured identity for
its overlay .202. Public IPv4 HTTPS and literal SSH work. Root accepted
preserving this exact pre-existing unhealthy baseline; key/peer repair is a
separate task. Guest migration must compare identity/configuration and the
actual baseline without claiming healthy overlay traffic. Rocky's baseline
overlay IPv4/IPv6 and public IPv4 HTTPS passed.

The candidate f3 PF observation guard is now prepared in the owner source
but remains undeployed. Native `pfctl -nf` accepted it; a scoped owner preview
would change only `/etc/pf.conf`. The 12:09:48 UTC complete verbose state
capture contained no 10.20.0.133 entries. That is dated preparation evidence;
repeat the exact state check immediately before any later guard activation
and refuse/wait for relevant states to expire rather than flushing states.

After root's post-power-window health clearance, the first rocky Stage attempt
at 13:06:23 UTC armed its independent journal but failed before network mutation:
native `nmcli -g IP4.DNS` returned `a | b | c` on one line, which the helper's
newline-only joining passed to `ipv4.dns` unchanged. PID1 explicitly restored
the original profile at 13:07:02 and reported no pending journal. New/old
interface state, default and profile hash remained original. The proposed
normalization fix handles both bar-separated and per-line runtime DNS lists;
the command fixture now emits both and validates each actual DNS argument as
IPv4. The old implementation fails that regression; the fixed fixture, native
`nmcli --offline` candidate, Bash syntax and shellcheck pass. Root reviews the
source fix before scoped installation and a new native expiry trial. The failed
pre-mutation attempt does not count as dual-address expiry acceptance.

Root reviewed and approved that correction. Its scoped Prepare reapply changed
only the helper; native and source SHA256 matched. Retry Stage at 13:09:43 UTC
completed successfully with deadline 13:24:43. Fresh old/new literal SSH, laptop
return via .1.2, DNS, public IPv4 HTTPS and wg0 overlay IPv4/IPv6 passed while
the old physical primary/default remained. Independent expiry, boot and final
confirmation acceptance are still in progress; do not infer them from Stage
success or the tool's activation marker.
