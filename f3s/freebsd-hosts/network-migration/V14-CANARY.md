# v14 bounded native canaries

Reviewed and executed 2026-11-05. The sections below preserve the approved
plan and its preflight observations; native results follow at the end.

Use f3 (`paul@192.168.1.133`, noninteractive doas, re0) and then r2
(`root@192.168.1.122`, enp0s5). These exercise the real hypervisor/ULA and
k3s/ULA branches. Guest-only tests would not establish those branches.
All cabling, AP settings, wg0 endpoints, controller addresses and production
NFS owners stay as they are. No isolated-profile gate is created.

## Fresh read-only observations

Both hosts have only their old physical IPv4; neither helper nor pending
journal exists. f3 cron is running; its helper/state paths are on root ZFS.
Its PF bridge hooks are both zero, temporary IPv6 addressing is disabled,
and sshd listens on all IPv4 addresses. Its PF asset matches the live file
except the proposed narrow wg2 SSH rule, which passed native `pfctl -nf -`.
f3 runs only standalone `rocky`; `freebsd` and the three lab guests are
stopped. vm_list is `rocky`, and rcshutdown_timeout is 90 seconds.

r2's physical profile is manual IPv4/automatic IPv6; NM and k3s are active,
firewalld disabled. Its active nft f3s_guard source filter nevertheless rejects
10.99.0.2; a second narrow owner-managed prerequisite is required. Inspection
of all nft base chains, iptables INPUT/KUBE chains and legacy IPv4 tables
found no additional TCP22 drop for that source/destination. Legacy IPv4
INPUT/FORWARD/OUTPUT are empty ACCEPT; no legacy IPv6 table file exists.
All r0/r1/r2 were Ready and API readyz returned `ok`.
The inspected unattended schedule was f3 minute 15 hourly and r2 minute 45
hourly; inspect the actual upcoming runs again before each trial. Do not
alter independent task 914 or race a reboot-capable upgrade.

## Prerequisite and baseline

1. Review the single added PF rule in `../pf/pf.conf`: source 10.99.0.2,
   host-local re0 destinations, TCP 22 only. Apply it to **f3 only**, through
   `freebsd_firewall_config`, preserving states. Establish fresh old-IP SSH
   before/after and verify only that intended PF delta. Accept this additive
   prerequisite separately before capturing the address rollback baseline.
   Its reversal is the prior managed PF asset loaded with `pfctl -f`, without
   flushing states. Do not widen trusted_lan for the migration.
   The concrete f3 owner preview (`push -n -privilege doas -- -p 22 ...`)
   reported only `/etc/pf.conf` would change; loader/sysctl/rc/pflog ownership
   was current, bridge sysctl and PF enable/module operations were skipped.
   Preview does not predict the dependent PF reload because its file is not
   actually changed; the real task must perform its checksum-gated load.
   For r2, the existing hostfirewall template now adds only
   `ip saddr 10.99.0.2 tcp dport 22 return` before its final drop. Stage with
   `rnodes_firewall_stage` on r2 alone, then use the existing guarded
   `f3s-firewall apply/confirm` protocol with two held independent old-IP
   sessions and fresh confirmation. Accept this firewall transaction before
   any address transaction. Its five-minute rollback and boot policy are
   owned by f3s-firewall; do not leave both journals unconfirmed together.
   Native nft candidate check passed. The concrete r2 Stage preview changes
   only candidate.nft and runs its existing restorecon command; daemon-reload
   is skipped and no rules are activated by Stage.
2. Save private persistent/runtime address, route, RA, interface, service,
   PF and wg0 semantic baselines. Do not print profile credentials or keys.
   Capture boot ID/time, f3 bridge/taps and VM state, k3s Flannel addresses,
   all-node readiness/quorum and NFS health. Keep fresh old-IP SSH available.
   f3 baseline must use `ifconfig -L re0 inet6` and record autoconf flags and
   finite lifetimes, not just addresses. The old helper would restore these
   as immortal static aliases; the prepared correction captures metadata,
   deducts elapsed time, preserves autoconf and skips expired addresses.
   Its regression passes and rejects the original helper. Native read-only
   metadata capture passed on both current autoconf addresses; actual native
   restoration still requires the bounded trial. f3's IPv6 default route has
   expire 0 and a dynamic router-list entry; recheck native RA/router ownership
   after rollback as well as the textual default route.
3. Install only the target's Operational Prepare task using a single-host
   gonf push (f3: freebsd_network_migration_prepare; r2:
   rocky_network_migration_prepare). Verify native script syntax and exact
   deployed assets. f3: inspect root cron, rcorder before netif/routing and
   early filesystem availability. r2: run explicit helper `init`, then verify
   timer enabled/active and both boot dependencies required by NM/k3s.
4. Provision only the documented root-only bounded canary gates. Never create
   endpoints-required, igc1-approved or serial-k3s-approved for Stage tests.

## Serial native trials

Run f3 trials before r2 trials, with no other unconfirmed network change.
Each transaction requires a new snapshot of the currently tested baseline.

* Expiry: f3 `stage shared 180`; r2 `request stage shared` (its reviewed
  deadline is fixed at 900 seconds). Disconnect the originating SSH process.
  Independently inspect the durable pending deadline/watchdog, then use
  fresh authenticated old-IP and new-IP sessions. New target is
  10.20.0.133 or 10.20.0.122, through laptop wg2 source 10.99.0.2; verify the
  return /32 is through 192.168.1.2. Old remains primary/default unchanged.
  Wait beyond the actual deadline and watcher interval without calling
  watchdog manually. Verify no pending journal, restored persistent keys/NM
  profile, scoped runtime addresses/routes/RA, protected wg0/PF/bridge state,
  fresh old-IP reconnect and disappearance of the introduced lab alias.
  Invoke explicit rollback twice and verify no additional change.
* Boot: arm a new Stage with sufficient time (f3 1800 seconds, r2 900), prove
  old/new fresh access, then reboot well before expiry. For f3 **first stop
  running guests gracefully outside rc.shutdown**, poll vm list until no
  guest is Running, then use `doas reboot` per the IQ safe-reboot runbook.
  Do not reboot with bhyve still running or force-kill a guest. On return,
  verify changed boot time, native boot restoration before deadline, no
  pending state, restored persistent/runtime baseline and fresh old SSH.
  Verify rocky auto-started; leave freebsd stopped. r2 is rebooted alone;
  verify the other two etcd members healthy before doing so, and require
  all three Ready, API/etcd quorum, cross-node pod/Service DNS/TCP, ingress,
  NFS health and unchanged Flannel/wg0 endpoints after return. A boot hook
  invoked manually is not boot acceptance.
* Confirmation: only after both platforms pass expiry and boot, Stage f3
  then r2 separately, prove fresh old/new access and required services,
  provide r2 confirm-approved, and explicitly confirm. Wait beyond the
  former deadline and verify accepted persistent/runtime state survives;
  reapply the scoped Prepare and normal owner recipes only after reviewing
  their concrete dry-run, proving they do not undo accepted Stage. Remove
  short-lived activation/action gates after testing. No promotion or k3s
  restart is needed for this Stage acceptance.

Root review accepts that the confirmation trial leaves f3/r2 in dual-address
Stage with old primary/default retained, only after **both** platform expiry
and actual boot proofs pass. This is the pilot baseline for subsequent
migration. Confirm archives its baseline; do not manually repoint journals.

## Scope of acceptance

Shared old/new and independent native rollback are the v14 claims. Dedicated
igc1 recovery remains unproved until the later physical test-device gate.
Promotion/k3s rebinding, endpoint transport changes, standalone guest runtime
acceptance and production rollout remain their separately guarded stages.
No new production address should be accepted until both native platforms
have passed recovery trials. Stop on a failed restore, missing fresh path,
unhealthy NFS/quorum or unexpected reboot; retain the pending journal for its
independent retry and resolve that failure before continuing.

## Native results — 2026-11-05

Both real platform branches passed, serially, with current cabling unchanged.
All times below are UTC. Private execution evidence is under
`~/.local/state/conf/v14-preparation/evidence/`; it includes protected snapshots
and must remain private.

| Platform | Native expiry | Actual unconfirmed boot | Final confirmation |
| --- | --- | --- | --- |
| f3 / re0 / FreeBSD | Deadline 10:50:49; cron restored by 10:51 | Reboot requested 10:53:23 after all VMs stopped; new kernel boot about 10:55:05; fresh old SSH/no pending at 10:56:35, before 11:21:45 deadline | 11:21:38, `accepted-1793877698.r7cKha` |
| r2 / enp0s5 / Rocky k3s | Deadline 11:14:08; PID1 watchdog restored 11:14:28 | Early persistent and pre-k3s runtime units succeeded 11:17:48; changed boot ID/no pending, before 11:31:06 deadline | 11:22:52, `confirmed-1793877772-16511` |

The initial f3 attempt exposed a router forwarding denial before any SYN
reached f3. Its expiry also exposed loss of dynamic IPv6 default-route
ownership. The router's gonf-managed tagged transit correction and FreeBSD
ownership/lifetime corrections were applied and tested before the successful
retry. The original IPv6 route flag was repaired explicitly from its saved
baseline; the defective static route was never adopted as a new baseline.
Finite autoconf lifetimes and the dynamic default-route flag then passed
native timed restoration. Negative fixtures reject the former behavior.

Each successful expiry had its initiating SSH process gone, independently
authenticated old/new paths before expiry, restored persistent/runtime state,
and two subsequent no-op rollbacks. f3 boot used graceful vm-bhyve shutdown
and `reboot`; standalone rocky auto-started and accepted fresh SSH, while
freebsd and the lab guests remained stopped. Only r2 rebooted among the etcd
members; the other two members retained quorum.

Both confirmations survived their former deadlines, including r2's 11:36:49
deadline plus the watchdog interval. Scoped Prepare/firewall owner previews
and actual reapplies changed no managed file or service; only the read-only
pending admission command was counted as changed. Four fresh old/new SSH
connections passed after reapply. Final state retains old IPv4 primary and
192.168.1.1 default, adds the reserved lab IPv4 and local ULA, and returns the
laptop /32 through 192.168.1.2. No pending journal or action gate remains.

All-node readiness, API and all three etcd members reporting a leader, bounded NFS public-index reads, r1/r2
unprivileged cross-node UDP/TCP DNS and API Service checks, direct Pi-hole
DNS, and verified-TLS Player ingress through the old VIP passed. Flannel
annotations/local bindings and running wg0 endpoints were unchanged.
f3-to-r0 wg0 IPv4/IPv6 and r2-to-f3 wg0 IPv6 each passed two of two pings.
The independent r2 updater ran successfully at 11:27:54 without rebooting or
changing accepted addresses; task 914 was untouched.

Both actual frontends reached r1/r2 WireGuard ports 30001/30800/30222
(12 of 12 TCP checks). Port 31637 was unavailable on r1/r2 as expected for
Traefik's current externalTrafficPolicy Local with its sole ready endpoint
on r0. Application relay-table health was not inferred from these TCP checks.
Gogios mute markers stayed absent. Separate frontend checks cleared transient
legacy HTTP errors; ordinary report federation can lag one cycle. Syncthing
state-file permission errors remain separately tracked in y24. Controller
resolver 192.168.1.101 blocks the LAN wildcard; direct Pi-hole queries and
explicit-VIP TLS ingress were used without changing controller DNS.

Both owned probe pods were deleted with UID preconditions, their label query
returned no objects, and owned held SSH clients were closed. This establishes
shared-LAN platform rollback and pilot Stage acceptance. Promotion, standalone
guest native acceptance, endpoint changes and physical igc1 recovery retain
their separate gates.
