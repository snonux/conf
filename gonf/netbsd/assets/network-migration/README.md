# NetBSD controller migration — task 624

Live checkpoint, 2026-11-07 (history; OLD retired 2026-11-11 by i24, below): both controllers have confirmed NEW primaries and
default10.20.0.1 with OLD/ULA/shared return retained, after actual confirmed
boots and management, website, authenticated controller, mesh and external WoL
checks. Original static WG and DTail are preserved. The native clean900s expiry
and actual unconfirmed boot fallback passed on Pi0. The first Promote exposed
NetBSD's deletion of routes tied to the removed primary; the corrected helper
rebuilds default/return routes after address changes. The fixture catches the
original defect, and corrected native Promote/rollback passed before acceptance.
All native former deadlines passed; native archives preceded exact owned-gate
cleanup. Unchanged-source Prepare preserved protected runtime on both controllers,
and fresh OLD/NEW/recovery-source access passed. Final narrow review passed
with no blocking findings; the earlier capture incident remains unexplained.

An earlier optional native mue0 packet capture stalled and Pi0 became unreachable.
The earlier timer did execute rollback at 08:32:01 UTC, but access remained down
until the user restored the host. That incident does not count as clean recovery
acceptance; its cause remains unproven. No native capture was repeated. Actual
WoL is instead verified by an external nonpromiscuous capture on f0, and the
clean trial above restored connectivity without reboot or intervention.
Pi0/pi1 run NetBSD11.0 on mue0;
tun0 remains owned by their existing userspace WireGuard service. Pi2 was
deferred during the earlier controller rollout; task524 later accepted both
resolvers on their NEW addresses. Work is serial and must retain one working
controller/static backend throughout.

`pis_netbsd_network_prepare` installs a base-sh helper, dormant NPF candidate,
minute cron watchdog and an enabled no-op rc hook. Without pending or accepted
state the hook changes nothing. Every method is Operational. Until retire, rc.conf
interface/default settings remain unchanged (retire rewrites them, see below); the accepted mode under
`/var/db/gonf-network-migration/accepted` is the managed persistent override,
applied after base network and before NPF/NETWORKING, WireGuard, HTTP and SSH.
Native rcorder verified that order on both actual controllers.

| Host | OLD (retired i24) | NEW | Recovery | Local ULA |
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

This transaction also owns NPF changes: explicit service targets (since j24
the lab address only; OLD and the recovery laptop-only SSH are retired),
native ULA services/NDP and unchanged tun0 permissions.
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
HTTP/ULA/mesh and unchanged power controls. On the native Pi0 canary, let the
real 900-second watchdog
restore with its SSH origin gone before manual recovery. Verify original
persistent/runtime/NPF state and repeated rollback no-op.

Prove actual NetBSD boot fallback on one controller while the other stays up.
Then separately Stage/confirm, Promote/confirm and observe former deadlines.
Promote requires an accepted Stage and selects NEW/default10.20.0.1 while
retaining OLD. Shared return is10.99.0.2/32 through192.168.1.2. Isolated aliases
and return routes require the later igc1 gate. An accepted `promote shared`
profile can transition directly with `request promote isolated` after the
human-confirmed igc1 connection and dedicated recovery proof. It retains the
NEW primary/default and OLD/ULA, adds the reserved98 address and replaces only
the laptop /32 return gateway with98.1. The same complete900s snapshot restores
the prior accepted mode, addresses, routes and NPF on expiry/unconfirmed boot;
confirmation persists `promote isolated` for subsequent boots. Re-running Stage
on an already promoted host remains forbidden. No cable is changed by the helper.
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

## Old-address retirement (i24)

`request retire` (task `pis_netbsd_network_retire`) drops the retained OLD
IPv4 for good. It needs the accepted `promote isolated` profile, the lab
primary, recovery address, default 10.20.0.1 and return route via 10.98.0.1,
and the `activation-approved`, `igc1-approved` and single-use
`retire-approved` gates. rc.conf must hold exactly one OLD
`ifconfig_mue0="inet OLD netmask 0xffffff00"` line and one `defaultroute=`
line and no other old-/24 reference; both are rewritten to the lab primary
and 10.20.0.1, so the host boots on the lab address even without the boot
hook. The ordinary 900 s snapshot additionally keeps a private rc.conf copy:
expiry, rollback and an unconfirmed boot restore rc.conf, the OLD address,
routes and the prior `promote isolated` mode (a foreign rc.conf edit keeps
the journal). Confirmation persists `retire isolated`; its boot override
adds recovery, ULA and the return route and never re-adds OLD. NPF is not
changed (`$host4` still listed OLD until j24's retire-recovery installed the
candidate without it). Like the other
requests, the result is in `request.*/result`, not the dispatch exit code.

Do not deploy anything that edits `/etc/rc.conf` (for example a `pis_netbsd`
run) while a retire is pending: the watchdog and boot hook then refuse to
restore and retry every minute; after a reboot the host still comes up on
the lab address but without the recovery alias and 10.99.0.2 return route.
After a confirmed retire there is no scripted way back (stage/promote need
OLD): restore rc.conf and `accepted` by hand from the private copy.

## Recovery-path retirement (j24)

`request retire-recovery` (`pis_netbsd_network_retire_recovery`) turns the
accepted `retire isolated` into `retire lab`: no recovery address and no
`10.99.0.2` host route (replies follow 10.20.0.1), and the Prepare-installed
NPF candidate, which no longer admits the recovery or retired OLD address,
replaces `/etc/npf.conf`. rc.conf is unchanged (it already has the lab primary
and 10.20.0.1). Gates: `activation-approved`, single-use
`retire-recovery-approved`; it refuses unless the OLD address is gone, the lab
primary, recovery address, 10.20.0.1 default and 10.98.0.1 route are present,
rc.conf has the lab lines and no 10.98 mention, and the candidate has no
recovery address. The 900 s journal restores the previous mode, NPF, address
and route. Live 2026-11-11 on pi1 (rebooted) and pi0.
