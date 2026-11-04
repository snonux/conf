# Storage endpoint migration preparation (p14)

Prepared only. No VIP, network, service, mount, DNS or canonical source changes
are made by these recipes. All five candidate tasks are Operational and omitted
from ordinary aggregates. This sequence belongs to guarded task 824; passing
these offline checks is not live acceptance or authorization to skip its gates.

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

Existing active owners remain unchanged: `Carp.RcConf`, `NFS.StunnelConf`,
`Relayd.Daemon`, `Maintenance.NFSClientStunnel` and earth's existing host config.
The old assets and `carpVIP` remain canonical. Do not copy a candidate live while
ordinary gonf reapply would still undo it. At acceptance, promote endpoint
parameters in these same owners and their tracked sources as one reviewed
change; do not add another recipe owning alias0 or either active config path.

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
   them; scale or pause only the NFS consumers, including external clients and
   earth transfers. Flush/sync and release mounts cleanly. Keep etcd quorum:
   never stop two k3s servers together. If clean quiescence cannot be verified,
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
4. Remove old 192.168.1.138 from **both** members, in running and persistent state,
   before installing new 10.20.0.138 on either. Verify neither answers old VIP and
   neither serves a writable replica. Never use simultaneous old/new VIPs,
   separate vhids, independent owners or a mixed-subnet CARP pair as rollback.
   Keep host management addresses; only the service VIP is exclusive.
5. Through existing owners, install one consistent new alias0 on both (same
   vhid1, existing secret, f0 skew0/f1 skew100), stunnel server accept .138:2323,
   relayd VIP80/443 and new r-node backends .120-.122. Publish both hosts' durable
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
  New VIP listens on2323/80/443; old VIP has no owner/listeners. Host old/new SSH
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
- Controlled failover requires the documented storage/dataset takeover procedure
  and zrepl checkpoint evidence; CARP election alone is not storage failover.
  Keep writers paused during any writable-owner transfer. Verify VIP, TLS/NFS
  and relayd on f1, then return to the intended owner using the same discipline.
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

Return old VIP to one known owner; verify the other BACKUP and authentication
counters. Restore devd and start the storage services through the existing CARP
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
