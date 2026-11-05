# f3 primary native acceptance — 2026-11-05

Current state is accepted OLD-primary dual Stage: .1.133 primary/default .1.1,
.20.133 alias, local ULA ::133 and laptop .99.2/32 via .1.2. The six-peer UDP
source guard remains installed, with zero additional drops. Source YAML and all
saved/runtime endpoint configuration remain OLD. No final primary confirmation
or confirmed-NEW cold boot is claimed here.

The first accepted 300-second primary expiry and actual NEW-source public NAT
are recorded in [Y14-PRIMARY-FIRST.md](Y14-PRIMARY-FIRST.md), together with both
failed earlier attempts and their original evidence. The source routing fix is
88db0d2, native helper SHA ef75c78d…; originals were retained rather than replaced
with a contaminated passing baseline.

## Actual 600-second boot recovery

Root reviewed the exact boot protocol and diagnostics. The native installed
graph passed: ldconfig40, network rollback42, early proof57, netif61, routing71,
PF80, WireGuard93, endpoint runtime100, runtime proof103, LOGIN173. The existing
vm_network-before-unknown-dnsmasq warning was identical without both diagnostics;
no new cycle or startup prerequisite was waived. Proof/context were root:wheel
0700/0600 and both rc hooks 0755; Python3.12 compile/native shell syntax passed.

r2's actual 17:45:02 updater succeeded, inactive/next18:45, without queued reboot.
Fresh full health and all six OLD-source peer paths passed. Standalone Rocky's
updater was idle before `doas vm stopall`: normal graceful stop ran
**17:46:09.810–17:46:13.959**, no force. Every guest was stopped, vm_list remained
rocky and FreeBSD/lab stopped policy remained unchanged.

The first controller preflight refused a raw interface-text difference before
any permission or journal existed: re0 options gained only TXCSUM bit0x2 when
the last guest tap disappeared. Other interface flags/MTU/addresses/ND6/WG/bridge
were exact. FreeBSD's [bridge capability code](https://raw.githubusercontent.com/freebsd/freebsd-src/releng/15.0/sys/net/if_bridge.c)
recalculates member capabilities on member removal. Root approved masking only
that observed capability bit/name for this explicit stopped-guest comparison;
other-bit/name/address/MTU negatives remained rejected. No checksum setting was
written and no address baseline was replaced. Final driver was 40002c9c… .

Native `promote shared 600` armed **17:49:48**, deadline **17:59:48**. Actual NEW
primary/default retained OLD alias/ULA/laptop return. Four held management paths,
all six bidirectional WG IPv4/IPv6 and actual outgoing OLD-source UDP, protected
fields, active guard zero and full cluster/storage/DNS/API/TLS health passed.
Inside pending, certificate-verified example.com HTTPS bound .20.133:49333 returned
200 from current-resolved **104.20.23.154**. Router igc0 captured original and
translated **.1.2:56332** SYNs, same sequence **2289715108**, 102 microseconds
apart, with matching actual PF original/translated tuples.

Immediately before boot, the complete actual pending snapshot was privately
copied and context atomically bound its actual deadline and original kernel
1793895125; original receipts and any matching prior archive were absent. More
than 360 seconds remained. The controller guard watcher stopped/joined and all
four held SSH sessions closed before one normal **doas reboot** at
**17:50:35.900**. No shutdown -r, forced VM stop, confirmation or other host reboot.

| Original native evidence | Actual result |
|---|---|
| New kernel epoch | 1793901137, **17:52:17 UTC** |
| Early receipt | **17:52:28.486703**, monotonic10.668702; OLD rc semantics/native archive, re0 IPv4 unconfigured and wg0 absent before netif |
| Runtime receipt | **17:52:37.186698**, monotonic19.368697; OLD primary/default/alias/ULA/return, strict captured WG and zero-drop active guard before LOGIN |
| Fresh independent OLD SSH | **17:52:42.260**, before17:59:48 deadline |
| Actual rollback archive | `/var/db/gonf-network-migration/rolled-back-1793901147.1SEqWn/snapshot` |

The original durable receipt mtimes were retained and read, not replayed manually.
This establishes genuine boot restoration rather than a later cron expiry.
Postboot original v4/v6, WG, bridge and protected non-rc files were exact;
helper-owned rc semantics/unrelated bytes were restored. Owned rc ordering and
explicit absent-key representation are not claimed byte-identical.

## Guest and service acceptance; terminal cleanup

Full postboot three Ready nodes/all three etcd members reporting a leader, NFS
same-hash, cross-node UDP/TCP DNS, API Service and explicit-VIP verified TLS401
passed, as did six peer identities/IPv4/IPv6, four literal management paths and
native ULA router/r2 diagnostics. Rocky automatically returned Running PID7185;
vm_list was unchanged and FreeBSD/lab guests stayed stopped.

Standalone Rocky retained accepted Stage, old default, new alias and laptop
return, old/new SSH, physical IPv6 disabled and no physical link-local/ULA.
Its actual two public-gateway host targets .2.110/::110 and .2.111/::111 passed
all four IPv4/IPv6 probes with fresh handshakes and transfer progress. Saved WG
hash remained original3074b411… . Normal DNS/certificate-verified public HTTPS200
passed. Its ordinary 17:55:00 updater then succeeded/inactive without changing
its new guest kernel ID; this occurred after the host journal had already been
archived at17:52, not during an unconfirmed address transaction.

Two extra diagnostic targets were unsuitable acceptance criteria: r2 overlay
is not a configured guest direct peer/route, and forwarded .2.200/::200 prefixes
are not the public gateways' own host addresses. Their unreachable responses
are retained separately, not reported as guest regressions. No keys, AllowedIPs,
endpoint, route or tunnel-reload repair occurred. The distinct FreeBSD guest's
pre-existing identity baseline remains unchanged and it stayed stopped.

Two explicit restores succeeded. Complete native journal and original private
proof/context/receipts were copied to controller evidence. Only the two owned
rc diagnostics were removed, with exact hashes checked; original base order
passed and private proof evidence remained. Scoped network Prepare/PF Config
reapply changed no files and skipped PF reload, preserving states. Own activation
permission was removed, address/endpoint pending were absent, and both exact
owned health-probe UIDs were deleted with UID preconditions.

This completes OLD-recovery boot acceptance. Final primary selection and actual
confirmed-NEW cold-boot acceptance remain the next reviewed slices, with a
bounded OLD-snapshot recovery scope. The destination-scoped full-referrer source
gate remains separate; no dependency on all-destination network promotion is
introduced. Private evidence: `~/.local/state/conf/y14-preparation/evidence/`
with primary-boot progress, original causal receipts, postboot original-baseline
comparisons, native snapshot/proof tar backups and terminal cleanup receipts.

## First confirmed-state guard trial stopped before confirmation

Conf c09e806's isolated confirmed-boot guard passed its source fixture and both
review corrections. The dormant helper SHA2fb69f5f… was installed at19:10:50
using the freshly rebuilt Go client containing both Prepare admission checks;
all other resources and protected live state matched. Installation did not arm
the guard. See [the guard contract](Y14-CONFIRMED-BOOT-NEXT.md).

After actual f0/r0 20:05 jobs and fresh health/six-peer checks, the first trial
promoted at20:06:14 with ordinary deadline20:08:14. NEW primary/default, retained
OLD alias/ULA/controller return, four held SSH paths, unchanged protected fields
and all six OLD-source peer transports passed. The complete original pending
snapshot was copied before confirmation and validated on the controller against
the original OLD beforeimage, with its canonical hash pinned locally.

The router NAT observer's local18-second wait timed out. The native command used
timeout15 without a hard-kill grace; why TERM did not finish remains unproved.
The original failed driver/result are retained. Strict failure recovery restored
OLD at20:06:54, before ordinary expiry. No ordinary confirmation, confirmed-state
guard activation, independent guard expiry or reboot occurred. No guard acceptance
is claimed from this attempt.

Postfailure verification matched protected files/rc semantics, IPv4/IPv6/WG,
bridge, VM state, OLD default and laptop return; pending scopes were absent and
the source guard remained zero. Full cluster/storage/DNS/API/verified TLS and
all six bidirectional overlay IPv4/IPv6 checks passed. Rollback twice was a no-op;
only the trial's empty private activation gate was removed. The whole matching
`rolled-back-1793909214.FyxjmZ` journal was archived privately. No router capture
or timeout process remained, and both exact health-probe UIDs were deleted.
The revised observer must preserve partial evidence and enforce finite native
and controller cleanup before a separately reviewed retry.
