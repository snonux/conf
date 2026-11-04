# FreeBSD address migration preparation (m14)

Prepared only. Nothing here establishes that native FreeBSD rollback, new
management connectivity or cable movement has passed acceptance. Keep all
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
SLAAC addresses are restored with their saved prefix lengths on timed
rollback; RA resumes where previously enabled. Boot rollback uses the saved
persistent settings. DHCP and an active DHCPv6 client require separate review.

Alias200/201/202 are reserved to this helper; collisions fail before mutation.
Promotion replaces the physical primary with the new address and keeps the
old as alias200. It never restarts netif or destroys a bridge. No retirement
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
root-only `igc1-approved` gate after shared acceptance, human test-device
cabling, fresh authenticated recovery proof, and the coordinated production
cable transition. Do not create that marker during shared staging. Accepted
isolated snapshots keep the recovery address/route on rollback. Never roll a
moved host back to an old-cabling snapshot: take a new snapshot of the tested
current-cabling state before another disruptive operation.

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
