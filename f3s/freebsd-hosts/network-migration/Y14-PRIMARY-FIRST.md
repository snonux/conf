# f3 physical-primary trial — native expiry acceptance

Historical first-slice proposal, reviewed and authorized by root on 2026-11-05.
The first invocation failed; the corrected third invocation later passed native
expiry as recorded below. No primary confirmation is claimed. All endpoint canaries are restored/archived, authoritative
YAML is OLD, and f3/r2/standalone guests retain accepted dual Stage with old
primaries/defaults. Root approved this exact first slice before the
PF owner apply and primary activation; any corrected-helper retry requires its
separate source/install checkpoint. No host reboot or second network writer
is included; the first JetKVM alias trial must not overlap.

## Scope and measured budget

Only f3/re0: promote physical primary to 10.20.0.133 and default to 10.20.0.1,
retain 192.168.1.133, local ULA ::133, and laptop 10.99.0.2/32 return via
192.168.1.2. Do not change endpoint source/saved/runtime fields, keys, peers,
AllowedIPs, public gateway destinations, wg1, DNS/hosts, service binds, bridge/
VM configuration or any other host. Do not set endpoints-required or isolated
gates. Old physical endpoints remain deliberately supported by retained aliases.

Use installed source-owned helper `promote shared 300`, not the Go task's
default 900. It supports 120..3600 seconds; 300 is sufficient for this first
nonboot trial. Old-primary six-peer reverse IPv4/IPv6 plus identity checks all
passed in **2.73 seconds** concurrently. Cached full health takes seconds;
forward peer checks/management and a bounded 15-second NAT capture leave ample
margin below 300. Do not switch to 600 unless measured checks cannot fit.

Fresh timing gate: actual f3 writer idle, relevant old-referrer updaters idle,
and full 300 + minute cadence + postcheck margin before a potentially disruptive
writer. r2's next was 16:45 UTC at preparation; if root review/preflight consumes
the available window, wait for its actual successful completion rather than
altering task 914. Then repeat full health/readiness and arm. All VMs remain
running/stopped normally in this slice; no guest stop/reboot is required.

## Narrow PF owner prerequisite, completed before address arm

Source diff is exactly the six prepared lines in
`f3s/freebsd-hosts/pf/pf.conf`: comments plus a block-out-log-quick UDP rule from
.20.133 to old peer .120/.121/.122/.130/.131/.132 port 56709 before pass-out.
The scoped `freebsd_firewall_config` preview on f3 at 16:36 would change only
/etc/pf.conf; bridge/loader/sysctl/rc/service resources were unchanged/skipped.
Native parse must be rerun with current source. Apply only f3 through this
existing owner, preserving states and bridge bypass.

Before load, keep independent authenticated old/new SSH open. Privately save
current PF file, loaded-checksum file, active rules/counters, complete verbose
states and protected values. Immediately inspect all original/translated tuples:
**refuse if any state admits NEW .20.133 → an old peer UDP56709 tuple**.
Ordinary exact OLD .1.133 states cannot admit the different source and must
remain. Do not flush states. The dated 16:20 capture had zero relevant NEW
states, but is not permission to skip the immediate check.

Under accepted old-primary Stage this specific deny is non-disruptive: it does
not match old-source mesh packets or TCP management. Validate installed source/
active rule and old/new fresh SSH, then exercise all six peers with OLD-source
transport before the primary arm. The guard stays in the owner's source and
on the host throughout address rollback; never automatically remove it while
new primary might be pending. Address failback restores old-source packets
without relying on a second remote firewall operation.

If loading the prerequisite itself has unexpected effects, restore its precise
PF before-image/loaded state through the same owner immediately with states
preserved and do not arm addresses. There is no host reboot or concurrent
unconfirmed firewall/controller change in this slice.

## Native 300-second transaction

1. Confirm no router/address/endpoint journal is pending; no production source
   marker and exact YAML receipt 41386d…. Record accepted Stage's owned rc keys,
   physical address order/routes/ULA, PF guard/counters, all protected file
   hashes, VM/bridge state and saved/runtime wg0 privately. Match all six old
   peer identities to actual hosts/generator and exact configured OLD endpoints.
   Source promotion/canonical host mapping remains deferred.
2. Create only fresh root-private network activation permission under this
   reviewed pilot scope. Use the installed helper
   `/usr/local/sbin/gonf-network-migration promote shared 300` via old literal
   paul/doas SSH. Its native minute watcher arms a complete accepted-Stage
   snapshot before changing primary/default. The helper already has tested
   independent expiry/boot recovery. No endpoint permission/journal is created.
3. Prove actual primary .20.133/default .20.1, retained old alias and unchanged
   return .99.2 via .1.2/ULA/no native IPv6 default. Hold fresh old/new management
   while performing the checks below; then close all origin/check SSH for the
   deliberate independent expiry. Never confirm this first transaction.

## Acceptance while pending, otherwise immediate restore

**Six peers, real physical source:** f3 pings each of f0/f1/f2/r0/r1/r2 overlay
IPv4/IPv6 and each peer pings f3 in both families. Compare handshake/transfer
progress and public-key-indexed endpoints against the baseline. A bounded native
f3 capture must show outgoing old-destination UDP56709 source .1.133 for every
peer; inspect new deny counters/logs throughout. If the guard counter increases above its recorded pretrial baseline, immediately restore the address transaction and verify old transport;
do not wait for expiry, alter a remote endpoint or load a whole stale tunnel.

The private six-peer driver is
`~/.local/state/conf/y14-preparation/six-peer-check.py`; actual identities and
exact zero-loss results are saved per host. Forward checks and source capture
complete the opposite direction. All six saved destinations remain OLD.

**Actual new-source public NAT:** inside the pending primary transaction, use
f3's installed curl with `--noproxy '*' -4 --interface 10.20.0.133
--local-port 49333 --connect-timeout 3 --max-time 10` and verified-TLS
https://example.com. Resolve one current IPv4 through the normal resolver and
pin it with `--resolve` for an unambiguous packet/state match. Record local IP,
remote IP and HTTP status. Do not disable certificate validation.

Concurrently use router **read-only** bounded timeout/tcpdump on igc0 to capture
the matching TCP SYN ingress from .20.133 and egress translated to 192.168.1.2;
verify matching remote tuple/sequence and read the corresponding PF state with
original/translated tuples. Router timeout/tcpdump are available. No router
configuration change is made. Old-source curl or wg2 SSH is not this proof.
Any NAT/required service failure triggers immediate address restore.

Also require configured DNS plus explicit Pi-hole checks, public and existing
verified-TLS ingress, native ULA diagnostics, three Ready nodes/all three members
reporting a leader, NFS consistency, cross-node TCP/UDP DNS and API Service.
VMs stay in their normal policy; compare protected state/fields. Keep the known
controller wildcard resolver limitation and FreeBSD guest identity baseline
separate. Only task-owned health probe UIDs may be created/cleaned.

## Independent expiry and terminal cleanup

If every pending check passes and guard counters remain unchanged from the
recorded pretrial baseline, close origin and
check SSH and let the host-owned minute cron expire the 300-second transaction.
After its deadline/cadence, reconnect through old literal management BEFORE any
manual restore. Require archived/no pending address journal and restored
accepted Stage: old primary/default, retained .20 alias/ULA/return route and
owned rc semantics, unchanged unrelated bytes/fields, protected service health,
unchanged endpoint source/saved/runtime scope and the PF guard still installed.

Run explicit rollback twice, scoped network Prepare preview/reapply and guard
owner reapply only after no journal remains; verify they do not undo recovered
Stage or create permissions. Remove only the temporary network authorization,
preserve all private journal/evidence and clean only owned probe UIDs. Report
new-source NAT/source-selection measurements and any failure honestly.

This slice ends with **old primary restored**, even when all checks pass. Actual
primary boot, final confirmed primary and serial guest primary transactions
remain separate root reviews. Full endpoint fanout/source promotion, old alias
retirement and physical cable gates are not satisfied by this one-host trial.


## Actual first invocation and recovery, 2026-11-05 UTC

The exact six-line PF owner apply passed native parse and retained held old/new
SSH plus all six peer IPv4/IPv6/identity checks. One authorized unauthenticated
one-byte UDP packet at **16:49:00.674815**, NEW .20.133:59498 → old r2
.1.122:56709, was recorded by native pflog as **block out on re0**. The first
observer omitted tcpdump `-e`, so its text assertion failed and automatically
restored the PF before-image through the same owner, without flushing states.
Reading the original durable pflog with `-e` recovered the actual action; no
second packet was sent. Root authorized reapplying the identical guard. Its
fresh active pretrial packet counter was **0** after that reload. The counter
must remain unchanged during a later primary trial; the original deliberate
readiness drop is separately preserved evidence.

Before promotion, fresh captures showed OLD .1.133-source UDP to all six old
peers, whose actual endpoints for f3 remained OLD. All six reverse IPv4/IPv6 and
identity checks, three Ready nodes, all three etcd members reporting a leader,
NFS consistency, cross-node UDP/TCP DNS, API Service and verified TLS Player401
passed. Actual r2 updater completed 16:45:02; standalone Rocky completed
16:55:00, both successful and inactive (next 17:45/17:55). f3 writer was idle.

The first native `promote shared 300` armed at **16:58:26**, deadline
**17:03:26**, then failed with `route: route has not been found`. Replacing the
physical primary detached its connected gateway routes. The helper attempted
`route change default` after the default had disappeared. Immediate native
rollback restored the old primary/dual-address set but encountered the same
absent-default error. Through still-working old literal SSH, recovery read the
original pending snapshot's `default4=192.168.1.1`, added exactly that saved old
default, and reran native rollback twice successfully. It restored the laptop
.99.2 host return via .1.2 as well. The complete original failed journal is
`/var/db/gonf-network-migration/rolled-back-1793897955.jkLI6u`, also privately
archived on the controller. This is a **failed promotion**, not timer-expiry or
new-source NAT acceptance; no confirmation or reboot occurred.

Post-recovery old primary/default, .20 alias/ULA and laptop return matched the
accepted Stage. Runtime wg0, all protected non-rc files, bridge settings and VM
state were exact; unrelated rc.conf bytes were exact and helper-owned rc keys
were semantically restored (only ordering and explicit absent-key unset changed).
Guard counter remained zero. All six peer checks and full service health passed
again. Address/endpoint journals and temporary activation permission were absent.
The guard remains installed. The original before-image and failure are retained;
no contaminated replacement baseline is used to claim this attempt passed.

The proposed minimal source correction selects `route add/change default` from
actual route presence in both promotion and restoration, and establishes the
laptop return after replacing the primary. Its offline fixture now models
FreeBSD's removal of gateway routes on connected-address detachment and rejects
changing an absent default. Original source fails this fixture at promotion;
corrected full fixture, shell syntax and shellcheck pass. Native installation
and any retry require root's concrete corrected-helper review.


## Corrected-helper retry and strict observer failure

Root independently reviewed the two-part routing correction, ran shell syntax,
shellcheck and the full fixture, and committed it as **88db0d2**. Scoped native
Prepare at 17:06:15 changed only `/usr/local/sbin/gonf-network-migration`, exact
SHA256 `ef75c78d2b99c4f8ebf4b758414faa391410b3953736bdd01b9bdb462ead7cd4`,
root:wheel 0755; its other six resources were unchanged and native syntax passed.
Original pre-first-failure values were retained as the acceptance baseline.

Fresh full health, six-peer OLD-source capture and actual OLD endpoint checks
passed. r0's 17:05:00 updater completed successfully and was idle (next 18:05);
r2/guest remained idle (next 17:45/17:55). The one approved retry armed
**17:07:05**, deadline **17:12:05**. Actual physical primary/default became
.20.133/.20.1 while OLD alias, ULA and laptop .99.2 return via .1.2 remained.
Four old/new control sessions stayed authenticated. Protected files/bridge/VM/
WG fields matched the original accepted baseline and guard counter stayed zero.
All six peers passed bidirectional IPv4/IPv6 and captured outgoing OLD-source
UDP, with no additional PF guard drop.

Inside pending, curl bound **10.20.0.133:49333** completed certificate-verified
HTTPS to current-resolved example.com **172.66.147.243**, HTTP **200**. The
router's actual PF table recorded the original .20.133:49333→remote:443 and
translated **192.168.1.2:64844**→the same remote:443 tuple. However the bounded
paired-SYN observer returned empty text after timeout, with packets received and
zero kernel drops: it omitted tcpdump `-l`, allowing buffered text to be lost on
termination. The strict paired-capture assertion therefore failed. No weaker
NAT acceptance or native expiry pass is substituted for the missing capture.

Immediate native address rollback completed **17:07:46**, proving the corrected
absent-default restoration added OLD .1.1 and the .99.2 return successfully.
The original retry journal is
`/var/db/gonf-network-migration/rolled-back-1793898466.b6XvYK`. Old primary/default,
NEW alias/ULA/return, original WG and protected values were reverified; full
cluster/storage/DNS/API/TLS health passed again and guard count remained zero.
Pending and the temporary activation gate were absent. No confirmation or
reboot occurred. A line-buffered observer and another transaction require the
next explicit root checkpoint; this retry is recorded as a strict observer
failure despite successful verified new-source HTTPS and PF NAT evidence.


## Third invocation: accepted pending checks and independent native expiry

Root approved exact controller driver 3bdcfd… with line-buffered captures and
matching original/translated SYN sequence required. Its native read-only observer
validation captured a router-origin certificate-verified HTTPS SYN through the
same igc0/filter/timeout path; an OLD-source f3 curl cannot validate this router
capture because the retained old default sends it directly through .1.1.

A hard time-window gate refused to arm before r1's 17:25 updater. No permission
or journal was created by that refused preflight. Actual r1 update completed
**17:25:00**, status0/inactive, next18:25; r2/guest next17:45/17:55. f3's actual
daily stamp matched today and its stamped branch plus ALLOW_REBOOT=0 was
nonmutating; no upgrader process was active. Native network and endpoint minute
watchers explicitly dispatched at 17:16. The missing 17:15 individual CMD log
was not claimed explained and no scheduler/task914 change was made.

Fresh full health, four paths, original baseline semantics, six exact OLD peer
endpoints and actual OLD-source UDP captures passed. The third trial armed
**17:25:43**, deadline **17:30:43**. Actual .20.133 primary/.20.1 default retained
OLD alias/ULA/.99.2 return; all protected fields/VM/bridge and WG state matched
the original pre-first-failure acceptance baseline. Four control sessions stayed
open during the checks. All six peers passed real bidirectional IPv4/IPv6 and
actual outgoing UDP source .1.133; the active PF guard remained zero throughout.

Inside pending, bound **10.20.0.133:49333** completed certificate-verified HTTPS
to example.com **172.66.147.243**, HTTP **200**. Native router igc0 captured the
original SYN at **17:26:08.389752** and translated **192.168.1.2:62911** SYN
98 microseconds later, both exact sequence **1970054197**, identical TCP
timestamps/options and expected decremented TTL. The actual PF table contained
both matching original and translated tuples. Full cluster/storage/DNS/API/TLS
health passed before every origin/held/check SSH session exited. No confirmation
or manual restoration occurred while waiting for the native deadline.

Original untouched native archive/snapshot mtimes show restoration at
**17:31:00.548854 / 17:31:00.530194**. Fresh old-path observation at
**17:31:26**, before any manual action, found pending absent and accepted OLD
primary/default, .20 alias/ULA/laptop return restored. Original v4/v6/WG/bridge/
VM/protected files were exact; helper-owned rc semantics and unrelated bytes
were restored (owned ordering/explicit absent-key representation is not claimed
byte-identical). Guard remained zero. The unique actual native journal is
`/var/db/gonf-network-migration/rolled-back-1793899860.ykLlQ3`, also copied
complete to private controller evidence.

Two explicit restores succeeded with no pending work. Scoped network Prepare
and PF Config reapply changed no files and skipped PF reload, preserving all
states. Native helper hash/mode stayed exact; only own temporary activation
permission was removed. Full restored health and all six peer checks passed;
native ULA router/r2 diagnostics passed. Rocky stayed Running PID6470; FreeBSD
and lab guests stayed stopped. Exact owned probe UIDs were cleaned after health.
This accepts the first primary expiry protocol and NEW-source NAT measurement;
final state remains OLD-primary accepted Stage. No host reboot, primary
confirmation, guest promotion or source/fanout change is inferred.
