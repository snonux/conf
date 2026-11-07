# FreeBSD address migration preparation (m14)

The bounded f3 native canary passed expiry, actual boot recovery, fresh
old/new management, confirmation and scoped reapply on 2026-11-05; see the
[v14 execution record](V14-CANARY.md#native-results--2026-11-05). The f3 primary
and both standalone guests completed their serial acceptance and cleanup on
2026-11-06; see the [primary acceptance record](Y14-CONFIRMED-NEW-BOOT-STABLE.md).
The [f1 execution record](Z14-F1.md) tracks the next host. Keep all
production cabling unchanged through shared-port0 acceptance. Never create
an activation marker merely because these local fixture checks pass.

`freebsd_network_migration_prepare` installs dormant tooling. All migration
methods are Operational and excluded from the routine `freebsd` aggregate.
Prepare refuses to overwrite tooling while a transaction is pending. It does
not change rc.conf addresses, routes, PF, service listeners or WireGuard.

Before production activation, task v14 must validate the helper on a bounded
FreeBSD canary: fresh literal old/new SSH connections, actual cron expiry
with the originating SSH session gone, repeated rollback, confirmation,
unconfirmed reboot, and routine gonf reapply after recovery. Check cron runs,
`rcorder /etc/rc.d/*` puts gonf_network_rollback before netif/routing, and the
helper/state filesystem is mounted at FILESYSTEMS. Check upcoming unattended
reboots independently; do not change task 914 or rely on its schedule.

Task v14 may create short-lived, scoped canary authorization through the
root-only `/var/db/gonf-network-migration/activation-approved` marker under
its existing approved task scope, after checking the independent watcher,
boot hook and coordinated service/firewall prerequisites. That bounded test
authorization permits collecting the missing native evidence; it does not
establish completed proof or authorize production rollout. Remove it after
the bounded test. Production authorization follows actual expiry, boot,
fresh reconnect and reapply acceptance plus the coordinated service/firewall
review. No recipe creates the marker. Stage and Promote arm
a 900-second transaction before changing persistent or running state. Root's
independent minute cron rolls it back within a minute of its deadline. A
persistent pending journal causes rollback before netif at boot regardless
of the deadline; the rc hook signals rc's documented ALRM handler to reload
rc.conf. Deleting a key alone leaves rc's cached variable alive: rollback
therefore appends a tagged `unset` statement for only its snapshot-absent
keys. The parent executes that statement on reload before netif enumerates
aliases; CARP alias0 is excluded. The next armed transaction removes the old
cleanup statement before adding new sysrc assignments. No timer is a child
of the migration SSH process. Clock jumps can
change the wall-clock deadline; choose the window accordingly and prove the
native cron path before relying on it.

Stage adds the new IPv4 while retaining the old primary, configures static
local ULA on re0, suppresses RA there, removes prior non-link-local physical
IPv6, and removes only a physical re0 IPv6 default. wg0 IPv6, CARP alias0,
vm-bhyve bridge/taps, PF, service binds and hosts names are untouched. Native
SLAAC addresses are restored with their saved prefix lengths, autoconf flag
and finite preferred/valid lifetimes on timed rollback. Time spent in the
trial is deducted; expired addresses are not resurrected. Metadata is captured
with native `ifconfig -L` before mutation. Temporary addresses and unsupported
autoconf lifetime metadata fail closed before activation, rather than becoming
immortal static aliases. RA resumes where previously enabled. Boot rollback uses the saved
persistent settings. DHCP and an active DHCPv6 client require separate review.

Alias200/201/202 are reserved to this helper; collisions fail before mutation.
Promotion selects the first non-CARP physical address and keeps the old as
alias200. It rotates only owned aliases, keeping the desired address present;
it never uses an implicit address replacement that could overwrite a CARP VIP.
A foreign ordinary IPv4 alias refuses promotion before arming. CARP rows may
precede the physical primary in ifconfig output after rotation. The helper never
restarts netif or destroys a bridge. No retirement
command exists. Runtime rollback replaces only that primary and this helper's
addresses; it does not clear the interface. Scoped rc.conf keys are restored
with sysrc, preserving CARP and unrelated managed settings. The entire
rc.conf/interface/route snapshots are private evidence, not files to copy
wholesale over other recipes. Do not run concurrent writers of the network
keys/static_routes while a transaction is unconfirmed.

The shared profile routes laptop wg2 `10.99.0.2/32` through reachable
`192.168.1.2`; it does not point at uncabled `10.98.0.1`. The isolated profile
adds the reserved `10.98.0.N` alias on the same re0 and routes that laptop
through `10.98.0.1`. Its later explicit CLI invocation requires a separate
root-only `igc1-approved` gate after shared acceptance, actual human-confirmed
igc1 connectivity and fresh dedicated recovery proof. Do not create it while
the recovery gateway is reachable only on a hypothetical future cable path. Accepted
isolated snapshots keep the recovery address/route on rollback. Never roll a
moved host back to an old-cabling snapshot: take a new snapshot of the tested
current-cabling state before another disruptive operation.

After shared promotion, `promote isolated` can add the reserved recovery alias
directly under its newly armed snapshot and explicit igc1 gate. It persists
alias202 without writing another lab alias201, keeps the accepted NEW primary,
default, OLD/ULA and CARP, and returns laptop traffic through98.1. A separate
isolated Stage is unnecessary. The user-confirmed dual-uplink preparation
topology permits testing this path before the old switch uplink is removed;
its actual igc1 recovery must already be proved, and the cable-removal human
gate still applies. Unconfirmed recovery restores the shared NEW snapshot,
including removal of its new alias202 from cached rc.conf at boot.

Each transaction is separate. Confirm Stage after fresh old/new sessions and
service checks, then arm Promote separately. Confirm is an operator assertion
that these external checks passed; a command exit status is not that proof.
The helper rejects confirmation after expiry. Failure leaves pending state
armed and independently retried. Root can explicitly request rollback at any
time. Journal archives are root-only and retained for inspection.

## Endpoint rollback integration (p24)

The helper never reads or changes mesh keys, peer sets, AllowedIPs or endpoints.
Before a transaction that includes endpoint changes, create the root-only
`endpoints-required` marker in its state directory. p24 must install the
root-owned executable `/usr/local/sbin/gonf-endpoint-rollback` implementing:

- `snapshot DIR`: capture the scoped authoritative source and saved/running
  endpoint state privately in DIR, failing before address mutation if incomplete.
- `restore-runtime DIR`: idempotently restore those scoped endpoint values;
  old host addresses have already been restored. Preserve unrelated wg0 data.
- `restore-boot DIR`: restore persistent/source state before interfaces start;
  defer any required runtime reconciliation safely until wg0 exists.
- `confirm DIR`: verify its source/saved/runtime acceptance contract and fail
  if incomplete. No full stale tunnel reload or whole-mesh install is permitted.

Hook failure or disappearance prevents confirmation and retains the pending
journal for retry. Coordinate that source rollback across referring hosts in
p24; this host-local helper cannot restore an unavailable controller's source
by itself. Keep old addresses reserved through endpoint/reboot/cutover acceptance.

PF currently trusts the old LAN prefix. Before any activation, prepare and
validate narrow old/new/recovery/ULA source rules through its existing owner
`freebsd_firewall_config`, preserve bridge bypass and old management rules,
and include any changed firewall/listener state in that step's independent
rollback contract. This helper intentionally makes no PF or service change;
its activation gate must remain absent until those prerequisites exist.

FreeBSD source references used for boot ordering and alias semantics:
[rc](https://github.com/freebsd/freebsd-src/blob/releng/15.0/libexec/rc/rc),
[rc.subr](https://github.com/freebsd/freebsd-src/blob/releng/15.0/libexec/rc/rc.subr),
[network.subr](https://github.com/freebsd/freebsd-src/blob/releng/15.0/libexec/rc/network.subr).
Run `python3 check-offline.py` for the disposable command-fixture check. Its
boot regression sources unconfirmed configuration in a real shell, runs the
rollback child, reloads configuration in that same parent, then verifies
absent keys and the actual enumerated alias variables. It covers clearing
Stage alias201 and clearing Promote alias200 while retaining accepted Stage
alias201. It executes no host network commands and is not a native FreeBSD
acceptance test.

For physical `re0` promotion, `check-wg-refresh.py` exercises the real helper's
private live-configuration snapshot and in-place WireGuard refresh, including
snapshot failure and partial-apply/rollback retries. `check-wg-static.py` checks
the semantic comparison used by live acceptance: peer/AllowedIPs order and
counters may change, but keys, endpoints, prefixes, ports and keepalives must
match. Never print the intermediate output of `wg-static.awk`: it includes
private configuration and must be hashed on the host. Use `check-offline.py
--guest-only` to run the existing standalone guest assertions separately.

Standalone guest preparation (q24, 2026-11-05): the separate
`freebsd-guest-migration` cluster exposes only `freebsd_guest_network_*`
actions. It does not join hypervisor aggregates or change vm-bhyve autostart.
Root coordinated a temporary guest start for read-only inspection; its default
policy remains stopped. Inspection established hostname freebsd.lan.buetow.org,
vtnet0 / MAC 58:9c:fc:00:56:44, static .139/24 and `inet6 accept_rtadv` in
root:wheel 0644 `/etc/rc.conf`; no rc.conf.local, empty rc.conf.d. Access is
paul with noninteractive doas; the inventory uses the existing trusted
`freebsd.lan` SSH alias. Root SSH is not assumed.

Its explicit `ipv6_policy=disabled` and empty ULA remove upstream physical
native IPv6 and suppress RA, leaving link-local and wg0 untouched. No guest
ULA is allocated. The existing rc.conf key journal and boot hook restore the
previous address/RA/default state, and timer, endpoint and current-cabling
return-route contracts remain identical. No netif restart or VM operation is
performed by guest migration actions. The fixture now exercises vtnet0 stage,
promote, runtime/boot restoration, missing isolated gate and unknown IPv6
policy. Native FreeBSD `/bin/sh -n` accepted both helper and boot hook; no
native expiry/reboot, connectivity or migration acceptance is claimed.
