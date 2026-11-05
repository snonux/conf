# v14 bounded native canary proposal

Prepared 2026-11-05. **Review required before any installation, activation,
address change or reboot.** No native acceptance is claimed by this document.

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
