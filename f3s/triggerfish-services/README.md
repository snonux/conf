# Dependent service candidates (r14)

Prepared only. Current inventory, active configurations, aggregate tasks and
canonical names retain their existing values. No live service, network, power
or relay operation was performed. This directory is outside ArgoCD inputs.

## Candidate preparation

The following explicitly named gonf tasks write inactive adjacent candidates;
they do not copy them over the active file, reload daemons or change outputs:

- `garage_migration_candidate`: root-private Garage TOML next to garage.toml.
  Only the advertised RPC address changes to the reserved lab suffix. RPC and
  S3 wildcard listeners, wg0 admin/metrics bind and credentials remain unchanged.
- `freebsd_apcupsd_migration_candidate`: adjacent apcupsd.conf candidate.
  f0 binds 10.20.0.130; f1-f3 poll 10.20.0.130:3551 and retain loopback NIS.
- `freebsd_dtail_migration_candidate`, `pis_netbsd_dtail_migration_candidate`,
  `rocky_dtail_migration_candidate`: adjacent dtail.json candidate using each
  existing typed LAN bind's reserved lab suffix. Check the task list for exact
  registration prefixes; OpenBSD wg0 binds have no candidate.
- `freebsd_f3sctl_migration_candidate`: adjacent restricted authorized-key
  candidate adds pi0/pi1 10.20.0.125/.126 while keeping old LAN and wg0 pins,
  the public key, forced agent command and `restrict` unchanged.
- `freebsd_shellyfans_migration_candidate`: inactive rc.conf.d snippet,
  `shellyfans_ip="10.30.0.28"`. The rc service accepts this optional setting;
  its default remains the helper's current 192.168.1.28. No service is run.

Install any eventually promoted file with its existing production owner/mode;
the Garage candidate deliberately uses root-only permissions for staging.

These methods are Operational, hence excluded from pattern aggregates. They
expect already provisioned service directories and existing packages/accounts.
The candidate write itself needs neither the future address nor a service
restart. Secrets resolve through existing providers, never stdout or this tree.
Normal aggregate applies continue using current inventory; an eventual live
promotion must change the canonical source in the same reviewed transaction,
so reapply cannot silently undo acceptance. Promotion is deliberately absent.

## f3sctl and device lookup

`f3sctl.triggerfish-candidate.json` is a configuration overlay for pi0/pi1,
loaded only when explicitly selected by `F3SCTL_CONFIG`. Merge its `inventory`
and `peer_nodes` keys into a privately saved copy of each existing node config;
do not replace credentials, API key paths, SSH identity paths or other local
settings. f3sctl reads a single file, not multiple merged overlays. No upstream
source/default change is needed. The full host list preserves MACs, SSH ports,
standalone f3, storage master/backup ordering, gateway wg0 addresses and mute
marker defaults. WoL uses 10.20.0.255; run on a Pi sharing the rack's physical
broadcast domain, never through a routed laptop broadcast. Peer coordination
must work on both new Pi addresses before enabling jobs through either peer.
Do not select this final candidate during a partially migrated peer set: prepare
a per-step merged copy retaining each still-old target and peer address.

`devices.csv` records dated IQ identities (2026-11-04) for explicit device-ID
lookup and review against triggerfish's MAC reservations. Three JetKVM host
attachments remain unknown. Do not create f0/f1/f3 console aliases by guessing.
Current JetKVM .241/.244/.245/.248 are observations, not guaranteed leases.
Device renumbering waits for s14's proven local timed/boot restore; retain
Shelly digest authentication and output state. No device settings are applied
here. Do not edit AP management/SSID/DHCP or controller 192.168.1.236 in Stage A.

## Application source candidates

Run `python3 f3s/triggerfish-services/render-candidates.py /tmp/r14-policies`
with an empty output directory. Requires PyYAML. It copies current namespace
NetworkPolicies and the Prometheus read-proxy policy, adding only equivalent
new /32 source peers to the same existing ingress rules, selectors and ports.
Old /32 entries stay present. Existing laptop/phone wg0 identities receive
10.99.0.2/.3 equivalents only where they already had application access.
It neither changes wg0 nor grants namespace-wide or all-port access.
VIP .138 permission promotion belongs to p14 and is deliberately excluded.

Review each changed rule and measure actual receiver source (including
NodePort/Traefik SNAT) before activation. These files are candidates, not proof
that PF, host guards, forwarding-header trust or a workload permits wg2. The
renderer refuses active policy output trees and nonempty directories. Generated
manifests are not ArgoCD inputs; activation requires an explicit source change.

## Startup and promotion order

1. Keep production cables and controller/AP access unchanged. Add/test new
   host aliases beside old addresses with current-cabling timed/boot rollback.
   Stage source guards and additive application/key permissions first. Preserve
   an SSH management listener on old/recovery addresses. Native ULA is local-only.
2. Accept f0's UPS master listener on its new literal IP, then move UPS clients
   serially. Save/restore master and client service files with network rollback.
   UPS safety thresholds, USB ownership and shutdown scripts do not change.
3. Advertise Garage RPC addresses one node at a time after every peer can reach
   its new TCP3901 endpoint. Existing wildcard binds accept retained old traffic.
   Verify cluster health, replication and S3 access before proceeding. Preserve
   Garage node IDs, RPC secret, storage, layout and wg0 metrics authentication.
4. Promote each dserver bind only after the new IP is configured at boot and
   host guard/source permissions are accepted. Preserve host keys/cache/log
   privileges and existing SSH management access. Restart and probe TCP2222
   separately; a working ordinary SSH session does not prove dserver works.
5. Keep CARP/NFS/stunnel/relayd/DNS transaction under p14; restore storage before
   workloads. Application permissions may be prepared before this transaction;
   do not prematurely repoint VIP-based clients or canonical service names.
6. Shelly device recovery and exact routed path must pass before fan override
   promotion. Existing rc order is `NETWORKING f3skeys`; curl/password must be
   available locally. It makes no cluster/DNS lookup, retries in background,
   and changes only the fan plug. f3sctl AC stays an explicit separate action.
7. Promote merged f3sctl controller config after source pins, both peer APIs,
   SSH target lookup, local WoL broadcast and both authenticated plug paths
   pass. Never migrate a controller or its AP/power dependency in the same
   operation as its recovery targets. Keep old addresses until final retirement.

Prometheus freebsd/exporter/Garage targets and control-plane scrapes stay on
192.168.2.x. Gogios Garage and DTail checks also use wg0; retain those sources
and targets. Verify monitoring after each restart and after cluster restoration;
do not mute monitoring to mask a migration failure. Source evidence:
`f3s/prometheus/{additional-scrape-configs,persistence-values}.yaml`,
`gonf/frontends/monitoring.go`, `gonf/cluster/cluster.go`.

## Exact mixed-stage flow proposals

`mixed-stage-flows.csv` expands every proposal below to literal source and
destination addresses, protocol and port, with an explicitly unverified status.
Select only the needed stage rows after route verification. These are review
inputs for k14, not activated TemporaryFlows. Use the actual
caller source selected by routes; never enable both forms merely by assumption.
Old-to-new traffic on the shared wire may bypass triggerfish entirely, so PF
counters alone cannot establish an application allowlist.

| Source (one literal per rule) | Destination | Protocol/port | Dependency |
|---|---|---|---|
| 10.20.0.130, .131, .132, .133 | 192.168.1.28 | TCP80 | New f-host boot helper to retained fan plug |
| 10.20.0.125, .126 | 192.168.1.28 and .29 separately | TCP80 | New Pis to retained plugs |
| 192.168.1.130, .131, .132, .133 | 10.30.0.28 | TCP80 | Retained f-hosts to renumbered fan plug |
| 192.168.1.125, .126 | 10.30.0.28 and .29 separately | TCP80 | Retained Pis to renumbered plugs |
| 10.99.0.2 | 192.168.1.28 and .29 separately | TCP80 | Authenticated laptop device recovery |
| 10.20.0.125, .126 | 192.168.1.130, .131, .132, .133 individually | TCP22 | New Pis to retained f-host agents |
| 192.168.1.125, .126 | 10.20.0.130, .131, .132, .133 individually | TCP22 | Retained Pis to new f-host agents |
| 10.20.0.131, .132, .133 | 192.168.1.130 | TCP3551 | New clients while UPS master remains old |
| 192.168.1.131, .132, .133 | 10.20.0.130 | TCP3551 | Old clients after UPS master promotion |
| 10.20.0.130, .131, .132 | Other Garage peers' retained 192.168.1.130/.131/.132 | TCP3901 | Mixed advertised RPC addresses; omit self-pairs |
| 192.168.1.130, .131, .132 | Other Garage peers' 10.20.0.130/.131/.132 | TCP3901 | Reverse mixed RPC connections; omit self-pairs |
| 10.20.0.125 | 192.168.1.126 | TCP80 | Mixed Pi peer coordination |
| 10.20.0.126 | 192.168.1.125 | TCP80 | Mixed Pi peer coordination |
| 192.168.1.125 | 10.20.0.126 | TCP80 | Reverse mixed Pi peer coordination |
| 192.168.1.126 | 10.20.0.125 | TCP80 | Reverse mixed Pi peer coordination |

Use literal full addresses when constructing each rule; table shorthand is not
an input format. k14 must keep TemporaryFlows empty until the selected stage,
source routing and exact dependencies are verified. Final fan/plug flows are
f0-f3 new /32s -> 10.30.0.28 TCP80, pi0/pi1 new /32s -> each plug TCP80,
and laptop wg2 10.99.0.2 -> each plug TCP80. No Wi-Fi initiation into the lab,
no general private-network exception and no phone plug control is introduced.
JetKVM laptop HTTP/WebRTC ports require installed-device inspection (s14);
do not invent a broad UDP exception.

## Power independence evidence and remaining acceptance

The current application inventory names f-hosts as shelly2's AC targets;
that is software intent, not measured physical power wiring. IQ requires the
router, AP, controller Pis and recovery path to survive f-host AC loss. Their
independent supplies and switch/AP path have **not** been tested by this work.
This does not block offline preparation. At the authorized power acceptance
step, verify fresh router/AP/Pi connectivity, literal-IP authenticated fan/AC
status and fan boot control with the cluster down, without changing a recovery
controller's power path in that same operation. Record observed supply mapping
and distinguish cluster shutdown from loss of the f-host AC feed. Do not claim
an independent power test merely because services answer with workloads off.

Retain saved service files in the owning host rollback set; expire and exercise
that rollback before live promotion. Reboot/reapply, old/new/recovery reconnect,
negative source/port checks and actual power transition tests remain live
acceptance work. No physical action or future topology is reported as verified.
