# y14 standalone guest Stage — native execution record

2026-11-05, UTC. Parent f3's previously accepted dual Stage and unchanged
production cabling remained in place. Only standalone guests were migrated
serially; r2's k3s VM is a different machine and was not changed here.

Root reviewed the concrete Y14-PILOT.md guest slice. The user's Shelly shutdown
window interrupted dormant preparation before any guest address trial. Root's
post-startup health clearance established three Ready/etcd members, NFS/DNS/API
Service/verified-TLS health before trials resumed. Neither independent task 914
nor its timers/services was altered.

## Results

| Guest | Independent expiry | Unconfirmed actual guest boot | Distinct final Stage confirmation |
|---|---|---|---|
| rocky / enp0s5 | Deadline 13:24:43; native PID1 watchdog restored 13:24:47; fresh old SSH 13:24:57 | Boot ID 39171758… → 708849ad…; boot hook 13:25:52–53, runtime restore 13:25:54; fresh old SSH 13:26:08 before 13:40:32 deadline | 13:27:14; separate old/new SSH sessions held during confirmation from a fresh new connection; former deadline 13:41:45 |
| freebsd / vtnet0 | Deadline 13:31:25; native cron restored at 13:32:00; fresh old SSH 13:32:22 | kern.boottime 13:27:38 → 13:33:38; boot restoration archive 13:33:40; fresh old SSH 13:34:09 before 13:48:15 deadline | 13:34:33; separate old/new SSH sessions held during confirmation from a fresh new connection; former deadline 13:49:30 |

Both platforms passed two explicit idempotent rollback calls, scoped Prepare
preview/reapply after recovery and after acceptance, and fresh literal old/new
SSH. All action authorizations were removed after final confirmation and both
journals report no pending transaction. Rocky's post-confirmation unattended
run at 13:35:49 completed successfully without disturbing accepted Stage.

Accepted guest state retains old primary/default 192.168.1.1, adds
10.20.0.123/.139 and the explicit laptop 10.99.0.2/32 return via 192.168.1.2.
Both standalone branches suppress upstream physical IPv6, keep link-local,
allocate no guest ULA and preserve wg0 configuration. No k3s operation or
flannel file is part of standalone Rocky migration.

Rocky restored its original NetworkManager profile byte-for-byte after expiry
and boot (SHA256 4ba730…); original wg0 configuration hash remained 3074b4….
Its original old-only IPv4/default and physical auto-IPv6 with finite lifetimes
and default returned. Final accepted Stage passed DNS, explicitly IPv4 HTTPS,
wg0 overlay IPv4/IPv6 and literal old/new management.

FreeBSD restored original effective rc.conf values, old-only IPv4/default,
RA policy, finite autoconf addresses and native IPv6 default after expiry/boot.
The helper writes explicit inherited defaults `static_routes=""` and
`ipv6_defaultrouter="NO"`, plus its documented tagged cached-rc unset line.
rc.conf is **semantically restored, not byte-identical**. Excluding only those
known helper-owned representations leaves all original unrelated bytes exact.
Root reviewed and accepted this comparison; no semantic failure was rebaselined.
Hosts, resolver and wg0 files remained byte-exact. Final accepted Stage passed
configured DNS, explicitly IPv4 HTTPS and literal old/new management.

Fresh old/new authenticated Rocky SSH after its former 13:41:45 deadline
passed, including another observation at 13:48. FreeBSD's fresh old/new SSH
passed at 13:50:51, after its former 13:49:30 deadline. Both retained accepted
Stage with no pending journal/gates. FreeBSD passed final DNS/explicitly IPv4
HTTPS and unchanged wg0 baseline checks, then graceful shutdown was requested
at 13:50:52. Parent vm-bhyve confirmed it stopped and vm_list remained rocky;
Rocky remained running/AUTO. These are separate observations after the former
deadlines, not inferences from immediate confirmation commands.

## Native-discovered DNS serialization correction

The first Rocky attempt at 13:06:23 armed recovery then failed **before network
mutation**. Native nmcli returned runtime DNS as `a | b | c` on one line;
newline-only joining passed an invalid DNS value to the connection editor.
Explicit PID1 rollback completed 13:07:02 against the original baseline.

Root reviewed the minimal shell normalization and mixed bar/row validating
fixture, then approved scoped helper reapply. The native/source helper hashes
matched. The corrected fixture passes, the original code fails that regression,
and native `nmcli --offline`, Bash syntax and shellcheck pass. The successful
expiry/boot/final trials above used the corrected helper. The failed initial
attempt is retained as evidence and does not count as address-expiry acceptance.

## Existing FreeBSD conditions preserved

The normally stopped FreeBSD guest was temporarily started only for authorized
preparation/tests. Its wg0 had zero handshakes/received bytes and failed overlay
IPv4/IPv6 probes before any migration. Its actual public identity differs from
the gateway's configured identity for .2.202/::202. Root approved preserving
that actual pre-existing behavior. Exact configuration/identity and zero
handshakes remain unchanged; no healthy overlay claim is made and no key/peer
repair was attempted. A separate repair needs its own approval.

An existing noninteractive fish banner prevented gonf's strict integer-version
probe and scp. This is a host startup stdout incompatibility, not a library bug:
`~/git/gonf/internal/remote/sync_probe.go` deliberately rejects contaminated
output. The guest's stale `~/.config/fish/conf.d` symlink targets
`~/git/conf/dotfiles/fish/conf.d`; games.fish lines 13–14 invoke colorscript
without an interactive guard and lines 8–9 emit the banner. Current controller
dotfiles' tracked games.fish is empty, so the future repair is user-home owner
reconciliation of the stale guest checkout, not a library or current-source
patch. No shell startup file was changed here.

The exact source-owned FreeBSD Prepare plan/blobs and same-source native
receiver were transferred privately over tar/SSH; native apply preview and
apply passed without relying on a clean version-probe stream. Reapply used
the same scoped recipe plan, not ad hoc network edits.

## Evidence and remaining scope

Private evidence: `~/.local/state/conf/y14-preparation/evidence/`, including
guest original/post-window baselines, native worker/cron/boot logs, restored
comparisons, `*-final-held-confirm.json`, exact source/deployed checks and
deadline socket proofs. Original guest file archives remain private on the
controller (Rocky) and guest `/var/db/y14-evidence/` (FreeBSD). No key material
is in this record. The sanitized identity-drift receipt and fish source location
are separate JSON evidence files for fresh follow-up workers.

Parent primary/defaults, wg0 endpoints, generator YAML, AP/controller/DNS and
cabling were not changed. The f3 outbound PF guard remains source preparation
only. Primary/gateway and endpoint-native proposals are separate in
Y14-NEXT.md and Y14-ENDPOINT-R2.md and require root slice coordination.
