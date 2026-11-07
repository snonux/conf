# NetBSD controller migration — task 624

Live checkpoint, 2026-11-07: Prepare passed on both controllers without changing
protected runtime or services. Pi0's first native Stage returned success and
fresh OLD/NEW/recovery-source SSH/HTTP, local ULA HTTP and public WG IPv4/IPv6
passed. Its unconfirmed deadline is 1794040291 (08:31:31 UTC). An optional
native mue0 packet capture stalled around 08:20 UTC, after which Pi0 stopped
responding; the cause is unproven. Keep its timer untouched and Pi1 OLD-only
until recovery is established. No native expiry or boot acceptance is claimed.
Pi0/pi1 run NetBSD11.0 on mue0;
tun0 remains owned by their existing userspace WireGuard service. The user
deferred Pi2; healthy Pi3 stays on its old DNS address. Work is serial and must
retain one working controller/static backend throughout.

`pis_netbsd_network_prepare` installs a base-sh helper, dormant NPF candidate,
minute cron watchdog and an enabled no-op rc hook. Without pending or accepted
state the hook changes nothing. Every method is Operational. Existing rc.conf
interface/default settings remain unchanged; the accepted mode under
`/var/db/gonf-network-migration/accepted` is the managed persistent override,
applied after base network and before NPF/NETWORKING, WireGuard, HTTP and SSH.
Native rcorder verified that order on both actual controllers.

| Host | OLD retained | NEW | Recovery | Local ULA |
| --- | --- | --- | --- | --- |
| pi0 | 192.168.1.125 | 10.20.0.125 | 10.98.0.125 | fd02:ca0b:f1cb:20::125 |
| pi1 | 192.168.1.126 | 10.20.0.126 | 10.98.0.126 | fd02:ca0b:f1cb:20::126 |

The complete private snapshot precedes runtime mutation and has a fixed
900-second deadline. It stores owned IPv4 order, ULA presence, default and
controller return route, original NPF file, proposed policy and prior accepted
mode. The watchdog restores at expiry; boot restores any pending state
unconditionally, even with a stale RTC-less boot clock. A crash between publishing
the accepted mode and archiving pending restores the prior snapshot. Publication
and confirmation sync before acknowledging persistence.

This transaction also owns NPF changes: explicit old/new service targets,
recovery laptop-only SSH, native ULA services/NDP and unchanged tun0 permissions.
Runtime reload retains active filtering; no stop/flush is used. Boot restores
the disk policy before ordinary NPF startup. Recovery refuses an unrelated edit
to /etc/npf.conf, retaining its journal rather than overwriting another owner.
No tun0, WG configuration/keys/peers, Wi-Fi or power setting is changed. Unknown
physical addresses/prefixes or RA acceptance refuse admission.

Both hosts provide flock, nohup, timeout, cron, npfctl and ifconfig. Requests
launch a bounded HUP-independent worker with closed SSH/lock descriptors; only
the fresh worker takes the mutation lock. A request receipt is not completion.
Inspect its `request.*/result` and log, then prove fresh authenticated access
and actual services before confirmation.

For each host, capture synchronized time and independent-controller HTTP/WoL
status; preserve private configs/hashes, exact routes and tun0 identity. Install
Prepare and prove no runtime/service change. Check cron/boot prerequisites and
native dispatch independence before creating root0600 exact-host activation and
action approvals. Stage shared; verify OLD/new/controller-bound recovery SSH,
HTTP/ULA/mesh and unchanged power controls. Let the real 900-second watchdog
restore with its SSH origin gone before manual recovery. Verify original
persistent/runtime/NPF state and repeated rollback no-op.

Prove actual NetBSD boot fallback on one controller while the other stays up.
Then separately Stage/confirm, Promote/confirm and observe former deadlines.
Promote requires an accepted Stage and selects NEW/default10.20.0.1 while
retaining OLD. Shared return is10.99.0.2/32 through192.168.1.2. Isolated aliases
and return routes require the later igc1 gate; cables remain untouched.
Verify public WG IPv4/IPv6, static site, CGI/SSH controller paths and an actual
harmless WoL packet for an already-on host; do not turn hosts off just to test
power. Capture broadcast traffic on another LAN host with promiscuous mode
disabled; do not repeat the stalled native mue0 capture. Pi1's unchanged
controller sent the expected f3 magic payload, verified externally on f0.
Archive native evidence before retiring owned approvals, then reapply
unchanged Prepare and check persistence. Native recovery, services, WoL, cleanup
and final review remain mandatory before task completion.

The real-dispatch disposable fixtures cover guards, inactive NPF, missing cron,
request/flock independence, fixed deadlines, expiry/restoration, partial failures,
unconfirmed/confirmed boot and foreign-edit refusal. Native NPF validation and
rcorder receipts are private in `~/.local/state/conf/624-netbsd-20261107/`.
Fixtures and other OS canaries do not replace this task's native recovery proof.
