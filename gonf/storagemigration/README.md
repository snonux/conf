# Storage endpoint migration (p14 preparation / 824 acceptance)

Task824 was accepted natively on2026-11-07. The five candidate tasks remain
Operational and inactive; ordinary source owners now preserve the accepted
configuration. The runbook below records the guarded cutover, not an instruction
to re-arm its OLD-only transaction against the accepted NEW state.

Candidate tasks (use `./gonf.sh -list` to inspect):

| Target | Task | Inactive destination |
|---|---|---|
| f0/f1 | `freebsd_carp_migration_candidate` | `/etc/rc.conf.carp.triggerfish-candidate` |
| f0/f1 | `freebsd_nfs_migration_candidate` | `/usr/local/etc/stunnel/stunnel.conf.triggerfish-candidate` |
| f0/f1 | `freebsd_relayd_migration_candidate` | `/usr/local/etc/relayd.conf.triggerfish-candidate` |
| r0/r1/r2 | `rnodes_nfs_client_migration_candidate` | `/etc/stunnel/stunnel.conf.triggerfish-candidate` |
| earth | `system_nfs_client_migration_candidate` | `/etc/stunnel/stunnel.conf.triggerfish-candidate` |

The CARP file is a **non-executable template**, never a file to source or copy
into rc.conf. It uses the existing `CarpNode` skew and `CarpPassSecret()` logical
reference, with an unresolved marker instead of the password. Promotion must
resolve the same `MustSecret(CarpPassSecret())` through the existing CARP owner;
never substitute a value in the command line, logs or public candidate. No keys,
CA, client PEMs or server certificates are read or staged. stunnel candidates
reference existing installed paths. r-node rendering reuses its active template;
earth's documented layout must be compared privately against its actual file
before promotion. Abort on differing TLS settings rather than overwriting them.

Existing active owners remain `Carp.RcConf`, `NFS.StunnelConf`, `Relayd.Daemon`,
`Maintenance.NFSClientStunnel` and Earth's named `System.NFSClientStunnel`.
They now preserve NEW alias0 plus OLD compatibility alias1 under the same vhid1,
dual stunnel/relayd listeners, NEW relay backends and all four NEW client
endpoints. Named owned reapply and an actual confirmed-NEW f0/r0 boot passed.
OLD endpoints and host management aliases remain until the separate924 gates.

**Task i24 (2026-11-10) retired the OLD compatibility VIP 192.168.1.138:**
alias1 left rc.conf and the runtime vhid on both members, and the stunnel and
relayd assets are NEW-only. The f0/f1 server and relay candidates therefore
refuse (their renderers need the OLD-only source); the procedure is in IQ
`topics/infra/openbsdrouter-plan/RETIREMENT-I24.md`.

## Approved 824 execution contract (2026-11-07)

Full k3s downtime and gradual original-member recovery are authorized. Hold
all r-nodes with their existing exact-node mode0600 maintenance gate and verify
native start refusal before any storage mutation. Never reset/bootstrap etcd or
remove datastore contents. This replaces the earlier continuous-quorum wording.

Keep OLD endpoint compatibility until task924. Both VIP addresses belong to the
**same existing vhid1**, on **both existing members**, with unchanged password
and skews. Stop CARP participation on both hosts before changing their address
sets: FreeBSD CARP authenticates the set, so mixed sets can elect two masters.
Never create a second independent vhid/owner for the compatibility VIP. The
server and ingress candidates now retain old listeners and use new backends.
The active owners remain unchanged until guarded native admission/review.

The existing f1 production receiver remains `readonly=on`, including while
MASTER. Acceptance is f0 NFS writes, f1 failover reads, then return-to-f0 writes.
Do not make the sink writable, roll it back, or claim writable failover.

Automatic timeout/boot fallback restores the old owned aliases and service
configuration **with CARP held disabled**. This deliberately leaves storage
unavailable while preserving independent old/new host management. Re-enable
CARP only after a fresh controller audit proves that BOTH members have matching
restored address sets and the original known data ownership. Autonomous
re-enablement on one restored member could otherwise create split brain with
an unreachable partner still using the new set. Keep writers gated throughout.
Rollback must not restore an entire stale rc.conf over unrelated task914 keys.

Do not use promiscuous capture on either NetBSD Pi: a previous capture coincided
with loss of remote services. Prefer router-side capture, with `-p`, only when
necessary. Pi controllers must remain independent of the hosts they recover.

## Before the maintenance window

1. Read IQ's REVIEW, RECOVERY, MIGRATION, ADDRESS-RECOVERY and storage CARP/NFS/
   nfs-mount-monitor runbooks. No cables move in this window. Keep OpenWrt
   management 192.168.1.101 and its SSID/DHCP unchanged. Keep triggerfish .1.2,
   all old host management aliases/listeners and wg0/wg1 intact.
2. Require accepted shared-wire new f0/f1 and r0-r2 addresses, fresh old/new
   authenticated connections, source guards and routing, plus client reachability
   from earth's actual path (LAN or routed wg2). Do not rely on old open sockets.
   Do not activate the igc1-only recovery return route on shared igc0.
3. Record f0/f1 CARP states/vhid/skew and authentication counters, NFS/stunnel/
   relayd/devd and failback states, dataset mounts/read-only properties, zrepl
   lag, all mount options, r-node watchdog counters/timer states and workload
   replica/maintenance states. Stop if no single known writable dataset owner
   exists. Do not run `ifconfig -k` or print a password-bearing rc.conf.
4. Save original persistent files privately (0600/root-owned), including CARP
   rc.conf, stunnel server/client configs, relayd, affected PF, hosts/DNS, earth
   config and service state. Existing TLS files remain in place. Snapshot the
   *running* VIP/listeners and ownership separately from persistent files.
5. **Mandatory execution prerequisite:** guarded824 must supply and exercise an
   independent detached deadline, persistent pending marker and boot fallback
   covering this *whole service transaction*, not only host network aliases.
   Stage rollback payloads on every participant before any mutation. A lost
   coordinator must leave writers paused and restore a coherent old pair;
   unconfirmed boot must not auto-promote an inconsistent pair. Coordinate
   deadlines so restore fences both VIP owners before either old VIP returns.
   Test expiry, boot restore, explicit fresh-connection confirmation and repeat
   rollback before relying on it. NetworkMigration's existing address rollback
   does not by itself restore CARP/stunnel/relayd/client/DNS state.
6. Validate candidates: `relayd -n -f <candidate>` on FreeBSD, and the installed
   stunnel version's supported configuration validation in an isolated fixture.
   Do not start stunnel as a supposed syntax check: it can bind production ports.
   Verify exact diffs: same vhid/skew/key reference, only .138 VIP/listeners and
   r0-r2 backend LAN addresses change; TLS verification and identities stay the
   same. Verify cert validity without printing private PEMs. Do not deploy
   packages, enable services or reload daemons as candidate validation.

## Quiesce, then change one coherent endpoint

1. Record and suspend application writers and job/controllers that could restart
   them, including external clients and earth transfers. Flush/sync and release
   mounts cleanly. Use the approved full-cluster maintenance gate; preserve
   original etcd membership/data and recover quorum gradually after acceptance. If clean quiescence cannot be verified,
   abort; do not use lazy/forced unmount or killing pinned writers as a shortcut.
2. Stop `nfs-mount-monitor.timer` on every r-node, wait for any running monitor
   service to finish and record its counter. It can write/remount/restart stunnel,
   cordon and reboot nodes. Suspend the f0 NFS canary cron and any other storage
   writers, pause replication at a clean agreed checkpoint as needed, and block
   CARP auto-failback with the existing CLI. Record each previous state for restore.
3. With rollback armed, inhibit CARP devd actions on **both** f0/f1 before changing
   VIP state (devd stop affects other hooks; keep the window short and restore its
   prior state). Explicitly stop NFS/stunnel on both and ingress as required.
   Verify the previous owner flushed data and the replica checkpoint is known.
4. With CARP disabled on BOTH members, configure both .138 addresses under
   the same existing vhid1 in running and persistent state on BOTH. Verify
   neither currently answers either VIP and neither serves a writable replica.
   Never enable mismatched address sets or independent compatibility owners.
   Keep host management addresses throughout.
5. Through existing owners, install consistent NEW alias0 and OLD alias1 on
   both (same vhid1, existing secret, f0 skew0/f1 skew100), dual stunnel server
   accepts .138:2323, dual relayd VIP80/443 and new r-node backends .120-.122. Publish both hosts' durable
   state before allowing automatic hooks. Select f0 as the initial owner, verify
   f1 BACKUP and no authentication-counter increase, then restore devd/hooks.
   Start the server through existing CARP control on the known writable owner;
   never promote f1's read-only replica merely to make a test pass.
6. Publish the r-node/earth stunnel candidates coherently and restart clients
   serially while mounts/writers stay quiesced. Preserve r-node hard,timeo=600
   and earth soft,timeo=150,retrans=3,nofail,noauto options and local port2323.
   Do not change fstab, idmap domain, exports, NFS root or TLS identity for this
   endpoint move. Bring storage back before any dependent workload.

## Accept before publishing names or resuming writers

- Prove exactly one CARP MASTER, one BACKUP and one writable authoritative data
  owner; inspect NFS/stunnel processes and sockets (stunnel only on MASTER).
  Both VIPs listen on2323/80/443 through the same elected owner. Host old/new SSH
  remain reachable through fresh sessions.
- From **each r-node and earth**, establish fresh TLS sessions to new VIP with
  existing client certs; reject missing/untrusted client certs. Check logs for
  TLS identity/expiry errors. Use a designated scratch export/file: bounded
  write, fsync, read/hash and delete. This is a future live acceptance test,
  never run during preparation. Verify mount and UID/GID behavior and clean
  unmount/remount; earth's two exports are both covered.
- Test ingress with explicit target IP and unchanged Host/SNI names before DNS:
  HTTP and HTTPS to new VIP, real application response/certificate, and each
  r-node backend. Verify public wg0 ingress and application source policies.
- Controlled failover requires zrepl checkpoint evidence. Keep application
  writers and write-probing monitors paused. Verify both VIPs, fresh TLS/NFS
  reads and relayd on f1 with its sink still readonly; then restore f0 and prove
  fresh writes. This is the existing read-only failover contract.
- q14 owns hosts/Pi-hole/public wildcard candidates: see
  [etchosts/MIGRATION.md](../etchosts/MIGRATION.md). Its offline CLI
  `go run ./cmd/hosts-migration -profile target -router-view lab` (in gonf/)
  emits HostsBlock, WireGuardBlock, PiHoleDNSMasq and AuthoritativeLAN; it
  does not install them. Promote them only after
  endpoint tests pass; record old/new TTLs and query both resolvers directly plus
  system resolution from every consumer (hosts can shadow DNS). Preserve names
  and certificates. Keep rollback of hosts/DNS in the same maintenance record.
- Commit/review canonical owner/source changes to match accepted runtime state;
  perform reapply and controlled boot/restore acceptance before retiring any old
  host management address. No concurrent unconfirmed reboot from task914: agree
  a bounded window without taking over or blocking its independent workflow.
- Only after acceptance restore watchdog counters/timers to recorded intended
  state, canary, replication, failback and workload controllers/writers. Confirm
  no accidental cordons or pending reboot escalation. Confirm rollback from fresh
  new sessions only after separately rechecking old management recovery.

## Full restore on any failure or deadline

Keep writers stopped throughout; don't retry writes against inconsistent state.
Fence failback and both devd hooks, stop client transports after clean mount
release, and stop NFS/stunnel/relayd on the pair. Remove the new VIP from BOTH
running/persistent configurations before either old VIP is restored. Restore
saved CARP alias0 with unchanged key/vhid/skew on both through its existing owner,
restore stunnel server/clients, relayd/backends, affected PF and hosts/DNS and
canonical sources so reapply cannot reintroduce the failed stage. Restore the
last accepted writable dataset ownership/replication direction; do not overwrite
newer data with a stale replica. If data ownership is uncertain, remain quiesced
and diagnose remotely rather than promoting another writer.

Keep CARP disabled after automatic restore. Once BOTH restored address sets
are audited together, return old VIP to one known owner; verify the other
BACKUP and authentication counters. Restore devd and start the storage services through the existing CARP
control, restart clients serially, and repeat old-endpoint TLS/NFS read/write and
ingress tests including earth and both DNS resolvers. Reconnect old management
by literal IP from a new session. Restore saved watchdog, cron, failback,
replication and workload states only after consistent storage passes. Retain all
old host addresses, unchanged AP and current cabling. A storage rollback after
physical cutover must keep igc1 recovery intact; it must not transfer router
prefixes back onto igc0 or require a cable move.

Run `python3 gonf/storagemigration/check-offline.py` from the repository root for
public renderer/property/negative checks. These checks neither contact hosts nor
claim that native parsers, deadlines, reboot recovery or service acceptance passed.


## Dormant 824 native transaction helper

`StorageMigration.Prepare` installs the Python3.12 helper, a minutely native
cron watchdog, and an early boot hook ordered after sysctl and before netif.
It stages only inactive candidates. It does not arm a deadline or stop a service.
The helper remains a candidate until independent review and native expiry/boot
proofs pass. An offline fixture is not evidence of native recovery.

All helper state is root-private in `/var/db/gonf-storage-migration`. A coordinator
creates an exact-node, root-owned mode0600 gate only after checking its stated
fact. `approved`, `pair-held`, `stunnel-validated`, `pair-matching`, `accepted`
and `old-pair-matching` attestations expire after120seconds. No gate supplies
shell code or credentials. Create the gate immediately before its operation.

- `arm 60..1800`: snapshot owned alias0/alias1 values and stunnel/relayd configs,
  current service states and an absolute deadline. Refuse existing transactions,
  aliases or a NEW VIP already present. Do not snapshot/restore all rc.conf.
- `hold`: stop devd, set `net.inet.carp.allow=0`, then stop NFS, stunnel and relayd.
  Cron continues, preserving task914 schedules and the rollback watchdog.
- After BOTH members are held, attest `pair-held` on each. Validate the relayd
  candidate with its installed native parser. Separately validate stunnel using
  an isolated loopback listener/ephemeral port fixture with existing TLS files;
  never start a candidate against production VIPs as a syntax check. Then attest
  `stunnel-validated` and `stage` each member before enabling either.
- `stage`: compare saved managed state, update owned alias0NEW/alias1OLD, install
  dual stunnel/relayd candidates and add NEW under existing vhid1. It refuses a
  changed backup or deadline with less than30seconds remaining.
- Independently compare BOTH complete address sets, skews, unchanged secret
  fingerprint (privately), firewall admission and candidate configs. Attest
  `pair-matching`, then `enable` f0 followed by f1. Once election settles,
  `reconcile` applies the existing carpcontrol.sh to the observed state; devd
  does not replay transitions missed while stopped. This never forces MASTER.
- `watchdog` restores on expiry; `boot` restores any unconfirmed transaction
  regardless of deadline. Both leave a persistent held marker and CARP disabled.
  Repeating `rollback` after restore is a no-op. The boot hook retains the fence
  even after the transaction has been archived.
- After BOTH restored members are freshly audited, attest `old-pair-matching`,
  `release-old` on each, then apply the existing CARP hook to settled election
  state. Native old-endpoint acceptance must pass before writers resume.
- After dual-endpoint acceptance AND canonical owner/source promotion so reapply
  retains the accepted state, attest `accepted` and `confirm` each transaction.
  Archive receipts and remove only the coordinator's exact gates. No OLD host
  management address or OLD service compatibility endpoint is retired here.

Run `python3 f3s/freebsd-hosts/storage-migration/check-offline.py` for actual
helper dispatch against disposable files and fake native calls. It verifies
expired-deadline restore, boot persistence, alias/config restore, unrelated file
preservation, repeat rollback and refusal to re-enable without pair attestation.


The narrow wg2 laptop PF prerequisite has a separate journal in `pf-pending`.
`pf-arm 60..900` saves only `/etc/pf.conf` and its managed loaded-file digest;
PF and storage transactions refuse to overlap in either direction. Apply the
additive VIP-only rule through `freebsd_firewall_config`, without flushing
states. The existing watchdog/early boot hook restores the old owned PF files
and reloads the old rules when PF is running; a failed load retains the journal
for retry. On boot with PF not yet running, its ordinary rc.d startup consumes
the restored file. Fresh OLD/NEW management and exact loaded-rule checks precede
`pf-confirm` with a fresh `pf-accepted` gate; an expired journal cannot confirm.
This prerequisite is accepted before arming the coordinated storage transaction.


## Accepted native outcome — 2026-11-07

Commits78482c2/98cca2b prepared and corrected the guarded transaction;
68f3003 adopted the reviewed canonical sources after actual TLS/NFS acceptance.
Native independent expiry, PF/host/client boot restore, fresh OLD/NEW management,
mutual TLS and missing/untrusted-client rejection passed. Both VIPs share vhid1
on f0/f1; f0 is the writable MASTER, f1 the read-only BACKUP. Controlled f1 reads
used the replicated probe through a fresh `ro,nosharecache` mount. Return-f0
writes passed on every client, including both Earth exports. A guest that mounted
f1 during boot needed a normal clean remount after return to f0 (ESTALE); no
transparent writable failover or handle continuity is claimed.

All transaction deadlines were confirmed without extension; their former times
passed. Normal monitor/zrepl/failback controls were restored, exact test files and
gates cleaned/archived, and the original cluster recovered gradually. Original
etcd cluster/member IDs, three Node UIDs and all51 workload UID/spec hashes were
preserved. Verified application HTTPS passed through NEW/OLD VIPs, both relay
origins to all NEW backends, and the public WireGuard ingress path.

The necessary web-policy /32 additions are local commit7d852ea. The existing
manual Argo Application stays pinned to a1112b9 with tracking preserved and no
self-heal; its expected OutOfSync remains until an approved publication/revision
reconciliation. No push occurred. Incremental zrepl completed; sender retention
pruning still cannot destroy the two pre-existing Oct17 held snapshots
(`qf3-upgrade` / `qf3-pre-upgrade`). Those holds were not changed. DNS still returns
OLD VIP, and broad924 endpoint maps, cabling and task914 remain outside this
change. Private receipts are under `~/.local/state/conf/824-storage-20261107/`.
