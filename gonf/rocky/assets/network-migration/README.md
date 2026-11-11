# Rocky physical-address migration (n14)

The bounded r2 native canary passed timed NetworkManager rollback, actual boot
recovery, fresh old/new management, confirmation and scoped reapply on
2026-11-05; see the [v14 execution record](../../../../f3s/freebsd-hosts/network-migration/V14-CANARY.md#native-results--2026-11-05).
Other nodes still require their own guarded acceptance. The original k3s role
covers r0–r2/enp0s5. Reviewed standalone and resolver roles have separate
interface/IPv6 and service requirements, documented below.

All `rocky_network_migration_*` tasks are Operational. Prepare installs dormant
root-owned tools and units, with no daemon reload, enable, address change or
restart. Routine gonf reapply neither activates these tools nor replaces a
pending recovery journal. The configuration comes from per-host inventory;
NetworkManager's accepted keyfile is the persistent stage authority. No recipe
continually forces the promoted addresses back over a recovered profile.

Before production activation, task v14 must exercise a bounded native canary: timer expiry
with the originating session gone, repeated restore, explicit confirmation,
unconfirmed reboot, then fresh authenticated literal old/new access and gonf
reapply. Review the host/NodePort source guards, listeners, DNS, storage and
endpoint transport before creating gates. Those components retain their own
owners; this helper never changes firewalls or /etc/hosts. Check the independent
unattended-upgrade reboot schedule without altering task 914. No automatic
reboot is requested here. The disposable offline fixture below is not evidence
that these native recovery tests passed.

`gonf-network-migration init` explicitly loads/enables the independent PID1
watchdog and both boot hooks. The 30-second timer checks a private 900-second
transaction deadline and retries failed restores. Its ownership does not depend
on SSH. Before NetworkManager starts, the required boot hook restores any
unconfirmed keyfile/drop-in and endpoint persistent state regardless of deadline.
A second hook restores physical runtime state after NetworkManager and before
k3s. Failure leaves the journal and blocks k3s startup. The timer uses wall time;
clock changes can affect its deadline and must be included in canary review.

Within its existing user-approved scope, v14 may provision short-lived,
canary-specific authorization after verifying the watchdog, boot hooks and
prerequisites. This permits the bounded tests needed to establish native proof;
it neither asserts that those tests passed nor authorizes production rollout.
Production authorization follows successful expiry, reboot, fresh reconnect
and reapply acceptance. Authorization uses mode-0600 root-owned files in
`/var/lib/gonf-network-migration/`:

- `activation-approved`: bounded v14 canary authorization, or production
  authorization after native expiry/boot/reconnect/reapply proofs and prerequisites.
- `stage-approved`: fresh old-IP access and explicit approval for this stage.
- `promote-approved`: fresh old/new management and source/service checks.
- `serial-k3s-approved`: contains exactly this node's `hostname -s`; issue to
  **one node only**, after checking the other two etcd members and previous node.
- `confirm-approved`: fresh new and independent old/recovery connections plus
  all applicable service acceptance checks passed for the pending transaction.

Recipes never create these assertions. Keep all current cabling, the AP's old
management/SSID/DHCP service and the controller's old address during shared
staging. Stage and Promote queue activation as a PID1-owned oneshot using
`systemd-run --no-block`: the request releases its lock before the worker
acquires it. A lost SSH session cannot interrupt the worker. Inspect its journal/status. Confirm
Stage before Promote; each action gets a separate rollback snapshot. Approval
files for the action and serial restart are consumed after successful use.
Never issue Promote to a cluster or use an aggregate parallel service restart:
push to precisely one r-node, wait for completion and independently validate
Node Ready, API readyz, etcd leader/quorum, cross-node pod TCP/DNS, Services,
ingress, NFS writes and Flannel local/annotation/FDB endpoints on all nodes.

Stage retains old IPv4 as primary and adds lab IPv4, preserving the current
IPv4 gateway/DNS and unrelated routes. It replaces only physical global IPv6
with the static local ULA, suppresses physical RA/DHCPv6 and installs no native
IPv6 default. Promote orders lab IPv4 first (NetworkManager's primary selector),
keeps old IPv4 and changes the IPv4 gateway to 10.20.0.1. It writes only the
explicit `flannel-iface` drop-in and restarts this one k3s server. No node/etcd
identity, pod/Service CIDR, WireGuard configuration or tunnel IPv6 is changed.
Before confirming, verify the selected physical primary and actual Flannel
binding, not just Ready.

Both shared actions persist a laptop `10.99.0.2/32` return route through reachable
`192.168.1.2`. They never select uncabled `10.98.0.1`. A later explicit helper
`request stage isolated` or `request promote isolated` also requires
`igc1-approved`, created only after shared acceptance, human cable gates,
dedicated test-device recovery proof and coordinated host/cable transition.
That profile retains 10.98.0.N and returns laptop traffic via 10.98.0.1. Take
only current-cabling snapshots after recovery has been tested: never arm an
old shared-wire rollback after moving the rack. Retirement is its own
transaction (`retire`, below).
Each transition replaces all entries for the owned laptop /32 with exactly one
current-topology route, preserving unrelated NM routes and their attributes.

The helper requires exactly one persistent NM keyfile for the physical active
UUID and fails closed for legacy ifcfg or ambiguous profiles. Resolve those
through a separately reviewed conversion; do not improvise a live conversion.
Snapshots are private and contain the scoped profile (possibly credentials),
physical global addresses/main-table routes, RA setting and owned Flannel
drop-in. Restore reloads/reactivates the original UUID then reconciles only
physical addresses/routes; WireGuard addresses/routes and pod interfaces remain
intact. Reapplying the old physical configuration can briefly disrupt it, and
restoring k3s after a
promotion requires a serial restart. No other NM, k3s drop-in or physical
address/route writers may run while a transaction is unconfirmed. NM DHCP
renewal and device ifindex behavior across reboot must pass native acceptance.

After promotion and runtime restoration, the helper refreshes WireGuard's
cached source selection by reapplying wg0's existing firewall mark unchanged.
Otherwise a retained old local address can remain WireGuard's UDP source after
the gateway changes and miss the new router's NAT policy. The helper validates
the mark and verifies it remains unchanged; it never reloads the tunnel or
changes keys, peers, endpoints, AllowedIPs or the listen port. It skips this
step when wg0 is absent during early boot. A refresh failure retains the
pending journal. The Linux 5.14
[netlink setter](https://github.com/torvalds/linux/blob/v5.14/drivers/net/wireguard/netlink.c)
clears peer source caches on a mark update, including the current value.

If endpoint changes are included, create `endpoints-required` before starting.
The p24 root-owned `/usr/local/sbin/gonf-endpoint-rollback` must implement
`snapshot DIR`, `restore-boot DIR`, `restore-runtime DIR`, `confirm DIR`. Snapshot
captures source/persistent/runtime scope before address mutation. Persistent
endpoint restoration precedes boot networking; runtime endpoint restore follows
address restoration. Hook failure retains the journal, preventing confirmation.
Coordinate authoritative controller source restoration in p24; this local tool
cannot guarantee an unreachable controller's source rollback. No whole-tunnel
reload or actual endpoint change is performed by this helper.

## Old-address retirement (i24)

`request retire` (task `*_network_retire`) removes only the retained old IPv4
once the isolated promote is accepted. It requires `activation-approved` and a
single-use `retire-approved` gate; k3s nodes also need `serial-k3s-approved`
naming exactly this node, one etcd member at a time. It needs no
`igc1-approved`: retire does not change the topology, it requires the isolated
one to be in place. Before arming it refuses unless the lab and recovery
addresses are present, the runtime gateway is 10.20.0.1, the laptop
`10.99.0.2/32` route uses 10.98.0.1, the global-DNS drop-in
`/etc/NetworkManager/conf.d/90-gonf-dns.conf` (`rocky_resolver_conf`) exists,
and the persistent profile still lists the old /24 beside lab and recovery,
saves `ipv4.gateway` 10.20.0.1 and the `10.99.0.2/32 10.98.0.1` route, and has
no saved route with an old-/24 destination or next hop (the next boot applies
the profile, not the runtime state).

Like stage/promote, `request retire` only queues the PID1 worker
(`systemd-run --no-block`) and returns before anything changed; a zero exit
is not success. Check `status` (pending deadline) and the journal of the
`gonf-network-change-*` unit before the acceptance checks. It then arms the ordinary
journal, drops the old /24 from `ipv4.addresses` and old-/24 entries from the
per-connection `ipv4.dns` (the global-DNS drop-in owns resolv.conf), deletes
the runtime address without reactivating the connection, and refreshes the
WireGuard source cache. Confirm, the 900 s watchdog and the boot hooks treat
it like stage/promote: an unconfirmed retire restores the old address.
Firewalls, hosts files and service listeners keep their own owners; retire
them separately once real traffic shows no old-address use.

Run `python3 check-offline.py` here for disposable command-fixture validation.
Bash/shellcheck and Go checks supplement it; offline checks do not replace
per-host native expiry/reboot and fresh remote connection proof.

Resolver role (`524`, accepted 2026-11-07): explicit `role=resolver` uses
`eth0` with native ULA and no k3s/Flannel actions. A staged Pi-hole compose
candidate joins the same journal/watchdog: snapshot compose and private TOML,
apply after network staging, and restore both during timed/boot rollback.
Restoration stops FTL before recopying TOML to defeat its shutdown write and
waits for Docker health before archiving the journal. The resolver boot runtime
unit orders after Docker without blocking network-online. Pi2 passed actual
combined expiry and boot recovery; both Pis passed confirmed NEW boots and
former-deadline observations. See the
[resolver record](../../../pihole/assets/network-migration/README.md) for current
service, wildcard and fresh-client acceptance. These results do not imply new
native acceptance for other host roles.

References: [NetworkManager properties](https://networkmanager.dev/docs/api/1.42/nm-settings-nmcli.html)
(primary address order, manual IPv6 and route policy),
[systemd timers](https://www.freedesktop.org/software/systemd/man/252/systemd.timer.html),
[boot network ordering](https://github.com/systemd/systemd/blob/main/docs/NETWORK_ONLINE.md).

Standalone guest preparation (q24, 2026-11-05): the explicit
`rocky-guest-migration` cluster exposes only `rocky_guest_network_*` actions
for rocky/.123. Read-only inspection verified active enp0s5, persistent NM
keyfile `/etc/NetworkManager/system-connections/enp0s5.nmconnection`, manual
IPv4 and automatic IPv6. This is a standalone build VM, with no k3s role.
Its `role=standalone`, `ipv6_policy=disabled`, empty ULA configuration removes
upstream native IPv6 on that interface without allocating a guest ULA or
changing wg0. Unknown policies fail closed. No flannel file is read, saved,
written or removed and no k3s gate/restart is used; a standalone boot runtime
unit has no k3s dependency. Snapshots record standalone ownership, so rollback
continues to leave unrelated k3s files alone. The normal r-node ULA and serial
restart contract remains unchanged.

The same old-IP retention, shared .2 return route, separately gated isolated
recovery, independent watchdog/boot rollback, endpoint hook and fresh service
acceptance requirements apply. Guest-native timer expiry, reboot recovery,
connectivity and endpoint acceptance remain unperformed. Native Bash syntax
and `nmcli --offline connection add` accepted the prepared address order,
return route and disabled-IPv6 policy without changing any live connection.
The disposable fixture exercises guest stage/promote/boot restore and rejects
an unallocated policy, checking no k3s restart or drop-in changes. Separate
synthetic WireGuard fixtures check unchanged-mark refresh and failure recovery.

## Recovery-path retirement (j24)

`request retire-recovery` (`*_network_retire_recovery`) runs after i24 and
removes the igc1 recovery address and the laptop's `10.99.0.2/32` route via
10.98.0.1 from the NM profile and at runtime (no reactivation), so laptop
replies follow the 10.20.0.1 default. Gates: `activation-approved`, single-use
`retire-recovery-approved`, plus `serial-k3s-approved` naming the node on
r-nodes. It refuses before arming unless the old address is gone (runtime and
profile), the lab and recovery addresses are present, the gateway is
10.20.0.1 (runtime and saved), the route uses 10.98.0.1 (runtime and saved),
no other saved route uses 10.98.x and the global DNS drop-in exists. The same
journal, PID1 watchdog and boot restore bring both back. Live 2026-11-11 on
rocky, pi3, pi2, r2, r1, r0.
