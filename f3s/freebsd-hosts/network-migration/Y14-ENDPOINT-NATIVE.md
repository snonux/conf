# y14 endpoint recovery — native execution record

2026-11-05, UTC. This record covers only the explicitly coordinated bounded
canary trials below. No authoritative YAML promotion or production endpoint
acceptance is implied. All old/new destination addresses remain configured,
physical primaries/defaults remain old, and the f3 PF source guard is undeployed.

## First direction: r2 referring to f3, independent inner expiry

Root approved the exact Y14-ENDPOINT-R2.md scope after both standalone guest
former-deadline observations and restoration of FreeBSD's stopped policy.
The helper and four unit files were frozen from generator git object 511aa23;
the moving worktree was not used. Deployed helper SHA256 was
`670cd63c8175623f2c19b40cab47967646d0d6cec358a4ba05c1476c5b663af1`.
The six introduced files were absent before provisioning, then installed
root-owned with their reviewed modes and exact manifest hashes.

Native Python 3.9 compile/systemd unit validation passed. Enforcing SELinux
labels were restored (helper bin_t, config usr_t, state var_lib_t). Boot/runtime
unit dependency links match actual wg-quick@wg0/k3s ordering; the independent
15-second timer was enabled/active. A PID1-owned transient **actually executed**
the helper's harmless scope action successfully at 13:53:10; this was not an
absent-record condition skip. No wg0/k3s restart occurred during provisioning.

Four fresh authenticated old/new f3/r2 literal SSH paths passed. A native f3
capture at 13:53:57 observed the diagnostic UDP56709 packet from .20.122 to
.20.133; reverse explicitly new-source ICMP passed. The diagnostic packet was
unauthenticated and was not treated as a WireGuard response/handshake proof.
Actual authenticated traffic was verified after the bounded apply.

Canary `y14_r2_f3_expiry_20261105` applied at 13:54:49. Its inner deadline was
13:56:49 (1793887009), outer hard lease 13:58:49 (1793887129). No production
recovery/accept marker, controller transaction, confirm or release was used.
Only r2's existing f3 Endpoint changed from .1.133:56709 to .20.133:56709.
The saved semantic delta was exactly that selected endpoint; all keys, peers,
AllowedIPs, keepalives, public endpoints and tunnel addressing were preserved.

Bidirectional overlay IPv4/IPv6 probes passed two of two, with handshake and
transfer progress captured. Health before/applied/after restoration passed:
three Ready nodes; all three etcd members' `has_leader` metric true; NFS sample
consistency; cross-node TCP/UDP DNS; API Service; direct Pi-hole DNS and
verified-TLS Player ingress 401. This does **not** assert three elected leaders
or compare a common leader ID. Earlier inherited runner output used the
misleading `etcdLeaders:3` label; the valid underlying metric evidence is kept,
and future runner output says `etcdMembersWithLeader`.

The originating snapshot/apply SSH and all traffic/health SSH processes exited
before expiry. The native watchdog's first eligible post-inner execution was
13:57:04 (the 13:56:48 tick preceded the deadline). A fresh old management
connection independently observed `restored` at **13:58:00, before outer
expiry and before any manual restore**. Saved file bytes exactly matched the
original, selected runtime Endpoint exactly matched captured .1.133:56709.
Thus this is inner-expiry evidence, not an outer-lease substitute.

Only after that observation, two explicit restores succeeded. Fresh old
transport and full runtime semantics matched the captured original; the
complete saved file remained byte-exact. The full journal was privately archived
at `/var/lib/gonf-endpoint/archived-y14_r2_f3_expiry_20261105-1793887198` with
lock held; pending became absent. The exact frozen installation reapplied as
six unchanged/zero changed without creating a journal or authorization.
The watchdog remains dormant with no record. Only owned y14 probe pods were
deleted with exact UID preconditions.

Authoritative source SHA256 stayed
`41386d059b09101a4a4f8e2a5315345a2d6180ae111fb483ada527d9a9e653d0`,
with no production controller marker. No source YAML, address/default, PF,
router/AP/controller setting, wg1, key or AllowedIPs was changed.

## Admission preparation for later slices

Read-only comparison found f3's saved configuration declares the exact .200/32
and ::200/128 prefixes on two gateway peers in order; runtime assigns them to
the last declaring peer. Canonical exact-prefix ownership projection removed
all apparent mismatch while preserving nested different-length prefixes and
all other fields. No actual unexplained drift remained on either host.
The old helper still correctly refused that saved/runtime representation;
no host AllowedIPs edits or drift waiver were used.

Separate task 134's generator commit 4d71e3f implements this strict snapshot
admission projection only. Its helper was privately frozen for a later reviewed
update, SHA256
`0a8b54b416cd493f04f14c5e8ea095ed70c8ae969044b4af664baa153616c197`.
The new function passed read-only strict admission against both captured actual
host configurations. At the first-expiry boundary the old deployed helper was
still frozen; the separately approved subsequent update is recorded below.

## First direction: confirmation and independent outer expiry

With the first journal restored/archived and pending absent, root approved the
exact 4d71e3f helper and new scope `y14_r2_f3_confirm_20261105`. Only helper
and dormant scope config changed; all six hashes/modes and the unchanged four
unit bodies were verified. Native compile and another actual PID1 scope
execution passed. Four fresh old/new SSH paths, native UDP rules, OLD source
digest and before-trial full service health passed again.

The canary applied at 14:05:57, inner deadline 14:07:57, outer 14:09:57.
Only after full applied-state health and bidirectional overlay IPv4/IPv6 passed,
canary confirmation occurred at 14:06:31. A separate observation at 14:09:02
proved phase confirmed and both saved/live **NEW** after the inner deadline
plus native watcher cadence, with fresh working overlay traffic. All trial SSH
connections then closed; no manual restoration was issued.

At 14:10:34 a fresh old-management connection independently observed phase
restored with the accepted flag still true, saved bytes exactly original and
**all runtime semantics** exactly captured original, including the selected
OLD endpoint. Thus native outer expiry restored an actually confirmed canary;
neither immediate confirm success nor unconfirmed inner expiry substitutes
for this proof. Two explicit restores followed, then fresh overlay/full service
health passed. The complete journal was archived privately at
`/var/lib/gonf-endpoint/archived-y14_r2_f3_confirm_20261105-1793888003`.
Pending became absent, six-file dormant reapply was unchanged and only owned
probe UIDs were removed. Source YAML remained OLD with its exact receipt.

## First direction: actual r2 boot

Root separately approved scope `y14_r2_f3_boot_20261105`, the frozen 0a8b54…
helper and exact boot config, plus two temporary read-only ExecStartPost proof
drop-ins. The four base unit bodies were unchanged. Native PID1 could read and
compile the exact proof source under SELinux; unit/drop-in validation passed.
The preboot full health checks passed, both other etcd members were healthy,
four literal old/new SSH paths worked, source YAML was OLD and the actual
updater was inactive/next 14:45. Only the task-owned r2 restartPolicy Never
probe was deleted with its UID precondition before arming.

The canary applied at 14:23:18, inner deadline 14:25:18, outer 14:27:18.
Saved/live NEW and real overlay IPv4/IPv6 traffic were verified. A normal r2-only
OS reboot was requested at 14:23:53, with NEW still present and more than a
minute left before inner expiry. Kernel boot ID changed from a87a9bb7… to
61109352…. No other node, hypervisor or guest rebooted in this slice.

The native read-only receipts and journal monotonic timestamps establish:

| Event | UTC | New-boot monotonic time |
|---|---|---|
| Saved bytes exact OLD, phase boot-restored | 14:24:48.636 | 6.058736 s |
| wg0 startup begins | 14:24:48.885 | 6.307914 s |
| wg0 startup finishes | 14:24:49.206 | 6.628229 s |
| Actual selected runtime OLD and strict non-Endpoint fields pass | 14:24:49.322 | 6.744475 s |
| k3s startup begins | 14:24:49.424 | 6.846211 s |

Both proof receipts required the new kernel ID and time **before inner expiry**.
Fresh old-management SSH returned at 14:25:03, also before inner expiry.
This causally proves saved restoration before wg0 and runtime restoration
before k3s; a later timer expiration is not substituted for boot recovery.

After boot, selected actual runtime matched its captured OLD endpoint, saved
bytes matched original and strict non-Endpoint fields passed. Accepted dual
addresses, old primary/default, laptop return route and native ULA persisted.
Fresh new management, bidirectional overlay IPv4/IPv6 two-of-two and full
cluster/storage/DNS/Service/TLS checks passed with a fresh owned r2 probe.
The independent postboot updater moved to 14:34:42; no schedule was changed.

Two explicit restores succeeded. The complete private journal was archived at
`/var/lib/gonf-endpoint/archived-y14_r2_f3_boot_20261105-1793888852`.
Only the two introduced proof drop-ins were removed, followed by daemon-reload,
base-unit verification and unchanged six-file dormant reapply. Pending was
absent; source YAML and all scope protections remained unchanged. Only owned
probe UIDs were cleaned. The private causal timeline is
`evidence/endpoint-r2-boot-causal-timeline.json` with the original boot journal
beside it. Proof source/context remain private evidence, not active hooks.

The r2 referring-host direction now has native independent inner expiry,
confirmation surviving inner expiry, hard outer restoration of confirmed state,
actual early boot restoration and scoped reapply evidence. This is not full
fanout endpoint source acceptance. Reverse f3 trials and all primary/gateway
changes remain separate root slice reviews.

## Reverse direction: f3 referring to r2, independent inner expiry

After root's separately guarded S14 router transaction was confirmed/no pending,
fresh old/new literal f3/r2 management, source receipt and f3's actual strict
kernel-projected admission passed. r2's 14:45 updater completed successfully;
its next 15:45 run left a clear trial window. Native AF_PACKET observation on
r2 proved the diagnostic .20.133 to .20.122 UDP56709 datagram arrived; no packet
capture package or firewall change was introduced.

Root reviewed and approved the frozen 4d71e3f helper, two exact Python3.12 rc
hooks, private scope and append-only unmanaged system crontab change. Four
previously absent paths were installed root:wheel with reviewed hashes/modes.
All original /etc/crontab bytes/uid/gid/mode were preserved; its mtime advanced
so cron could reload. The independent command was actually dispatched at
15:05:00. Native compile, shell syntax, scope and rcorder passed: early hook
before netif/wireguard, runtime hook after wireguard/before LOGIN. No interpreter
symlink, root-managed crontab/task 914 change or service restart occurred.

Canary `y14_f3_r2_expiry_20261105` applied at 15:06:08, inner deadline
15:08:08 and outer 15:10:08. Only f3's r2 Endpoint changed from .1.122:56709
to .20.122:56709. Saved/live NEW, exact selected saved delta, bidirectional
overlay IPv4/IPv6 two-of-two and full cluster/storage/DNS/Service/TLS health
passed. All origin/traffic/health SSH processes then exited.

The native minute-cron restored at **15:09:00.074559**, captured from the
record's untouched mtime before manual actions. Fresh old SSH independently
observed restored state at **15:09:23, before outer 15:10:08 and before any
manual restore**: saved bytes exactly original, selected actual runtime exactly
captured OLD .1.122:56709, and all runtime semantics exact original, including
the expected kernel AllowedIPs ownership. This proves inner expiry through
FreeBSD's minute cadence, not a later outer recovery.

Two explicit restores and full post-restoration health passed. The complete
private scope was archived at
`/var/db/gonf-endpoint/archived-y14_f3_r2_expiry_20261105-1793891501`.
Pending became absent; four-file plus system-cron dormant reapply was unchanged,
with no new journal/authorization. Both owned probe UIDs were deleted. Source
YAML stayed OLD. Reverse confirmation/outer expiry and actual f3 boot remain
separate subsequent root slices.

Private evidence is under `~/.local/state/conf/y14-preparation/evidence/`;
exact pinned installation/config manifests, original private semantic snapshots
and read-only ownership comparisons are under its sibling `endpoint/`.
No key/configuration contents are published here.
