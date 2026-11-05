# f3 confirmed-NEW boot: failed first trial and historical protocol

The first actual confirmed-NEW boot trial **failed** its strict postboot peer
check. It did not release the guard. f3 was restored to OLD-primary dual Stage,
and complete postfailure cleanup and health verification passed. Source YAML,
all remote endpoints and all keys remain unchanged.

The original causal receipts are valid and retained: new kernel1793912987,
early persistent NEW at21:09:58.344 before physical/WireGuard configuration,
runtime NEW at21:10:07.139 before LOGIN, and fresh OLD-path SSH at21:10:11.745.
The strict postboot measurement then saw r1 receive only1 of2 IPv4 replies;
its second IPv4 reply, both IPv6 replies, SSH and identity passed. The other five
peers passed. Guard counters stayed zero. A cold-session startup explanation is
a hypothesis, not an accepted cause or a waiver of the failed measurement.

Trusted explicit guard recovery restored actual OLD at21:10:28.107, before the
original21:18:01 guard deadline. No postboot NAT proof or release was reached.
Postfailure original protected state, four fresh control paths, all six strict
IPv4/IPv6 peer checks, strict original standalone-guest WG/configuration and
gateway checks, full cluster/NFS/DNS/API/TLS health and normal Rocky-running /
FreeBSD-and-labs-stopped policy passed. Whole accepted/restored journals and
original proof/context/receipts/rc images were archived privately. Only the two
owned diagnostic rc hooks and the empty activation marker were removed; the
base graph was restored, scoped Prepare/PF reapply changed nothing, and both
exact owned probe UIDs were deleted and drained. Private context and original
receipts remain on f3; no address transaction, guard or activation marker remains.

The failure reporter also exposed private WG context in the tool transcript by
printing a raw nested report. No values are reproduced here. The original
bundle and private evidence are preserved. Retry is blocked until a tested flat
metadata allowlist, private0600 diagnostics and an operator containment wrapper
prevent unstructured stdout/stderr from reaching the transcript. No keys are
rotated or changed by this task.

The table and protocol below describe the **historical frozen first trial**,
not a currently unarmed installation or a new execution authorization. A new
distinct retry bundle and separately reviewed bounded readiness round must
precede an unchanged fresh strict2/2 measurement; the failed first attempt
cannot be relabeled as accepted. The independent no-reboot confirmed-state guard
expiry remains accepted separately.

## Historical frozen artifacts and former dormant installation

Private controller root:
`~/.local/state/conf/y14-preparation/confirmed-new-boot-reviewed/`.

| Artifact | SHA-256 |
| --- | --- |
| `driver.py` | `ce8fa623b657f68e59e3af29c9b5db1d7a252152064d3aac70e892705d541c98` |
| `native_ops.py` | `18749761da3e6fe8cd4f752903171e31c9d85316660e5f1a84cb9468e48fc711` |
| `pending_snapshot.py` | `b11ca0931c0a0380bb250218d571c12bd5d9a184b2d3c537b087f614a9dbc83a` |
| `guest-postboot.py` | `d2e20905b19302adac146c65f566dc011fbcdec9a879e97b6aed1ccf1bf718f5` |
| `restore_normal_rocky.py` | `d36d9186fd94c69beaf72969c092641419855d4bae34b6b46fa6b9e1655ecac9` |
| `proof.py` | `fc92c50a4e06f385a71ff723b9c586a9e529262ff07a482be8e23b24c1616558` |
| `y14_confirmed_new_boot_proof` | `6a01b841400aef227c2a0556ab6e38c22975a15e5abe8aac55649308609a97ce` |
| `y14_confirmed_new_runtime_proof` | `018e2f1293cf459c93886e1665850503a1cba627f2597b1531ae3b717f8cbaf0` |
| Unarmed private context | `0b8067c4396b68fc123976cc5825d52c53f83d99238c1b4d944f7d5c91205577` |

Helper `2fb69f5f…`, current Go controller `96032005…`, endpoint projection
helper `0a8b54b4…`, inventory and the six-line PF guard are unchanged.

Root authorized and accepted the former dormant file-only installation: private proof
and context under `/var/db/gonf-network-migration/y14-confirmed-new-boot-proof`
(directory0700/files0600 root:wheel), plus the two named diagnostic files under
`/etc/rc.d` (0555 root:wheel). Original protected files, rc.conf bytes, addresses,
routes, WG, VM state and guard counters stayed unchanged at installation. The
later original receipts/context are retained privately; the two rc hooks have
now been removed after failure. Native compile and shell syntax passed.

Actual installed rcorder positions were ldconfig40, base rollback42, early
proof57, netif61, routing71, PF80, WireGuard93, endpoint runtime100, runtime
proof103, LOGIN173. The existing vm_network/dnsmasq warning is unchanged.
Recheck this graph, every installed digest/mode, and absent receipts before arm.

## Schedule and controlled entry

Reserve the entire ordinary300 + guard600 + minute60 + margin45 = **1005
seconds** before the next potentially disruptive writer. Do not shorten leases
or modify task914. Check all f0–f3/r0–r2 jobs and standalone Rocky, actual job
completion, no reboot jobs, and both calendar and postboot OnBoot timers.
The f3 daily-stamped hourly task has its proven no-reboot/nonmutating policy.
The 20:45→20:55 interval is insufficient. A possible window is after actual
21:05 f0/r0 completion, before 21:25 f1/r1, subject to fresh evidence.

Before any VM stop: fresh full service/quorum checks, both literal control
addresses, all six peer identities/endpoints and actual outgoing OLD-source UDP,
no PF state admitting guarded NEW-source UDP, guard counter zero, original Stage
comparison, unchanged YAML, and no router/address/endpoint/guard pending journal.
Root holds all other network writers.

Then use one normal `doas vm stopall`, without force; verify all VMs stopped,
`vm_enable=YES` and `vm_list=rocky` unchanged. FreeBSD/labs stay stopped. The
already accepted comparator masks only TXCSUM bit0x2/name on interface options
for this guest-stop comparison; all other original state remains exact.

The launch wrapper pins every artifact and exports `Y14_NEXT_WRITER_EPOCH` from
the verified actual schedule. The driver independently enforces the full1005
budget before preflight and immediately before arm; it never waits and autoarms.

## One guarded actual NEW boot

1. Hold four fresh OLD/NEW literal SSH paths. Native `promote shared 300`; verify
   NEW primary20.133/default20.1, OLD alias, local ULA and .99 return via1.2.
   Capture ORIGINAL pending to the controller before confirmation. Validate the
   complete root-private flat OLD snapshot and compute its expected hash locally.
2. Require unchanged crypto/peers/endpoints, all six bidirectional WG IPv4/IPv6
   paths and actual OLD-source UDP, zero additional PF guard drops, bound NEW
   HTTPS200 with paired router ingress/translated SYN sequence and exact PF tuple,
   and full cluster/NFS/DNS/API/TLS health. Confirm with more than30seconds left.
   Match the unique accepted OLD archive to the controller hash, then arm600.
3. Prove current-Go Prepare refuses the active guard without footprint change.
   Verify installed proof/context/hook hashes and the unarmed context hash.
   Write only actual guard/source/hash/deadlines into the private context, with
   more than540seconds left. Preserve the original guard/context before reboot.
   Stop/join the watcher, drain both captures, close all held paths, require more
   than500seconds left, then request one normal f3-only `doas reboot`.
4. Original early receipt must come from a genuinely new kernel, after the base
   hook durably claimed that kernel, with persistent NEW owned rc fields while
   physical IPv4 is still unconfigured and wg0 absent. Original runtime receipt
   must prove actual NEW first IPv4/default, persistent/runtime OLD alias, ULA,
   return via1.2, strict original WG fields and guard0 before LOGIN. Both proof
   phases check the deadline again after their final query before constructing
   the exclusive durable receipt. The controller requires original receipt epochs
   before the guard deadline; it never manually replays a proof.
5. After fresh OLD-path SSH/new-kernel return, require four fresh control paths,
   normal Rocky AUTO/RUNNING and FreeBSD/labs STOPPED, unchanged vm_list, all
   protected files, exact NEW owned rc semantics and exact unrelated rc content.
   Re-prove all six actual OLD physical UDP sources after new WG startup, both
   overlay families, local ULA connectivity, bound NEW-source NAT/SYN/PF/TLS and
   full service health. Preboot captures do not establish postboot source choice.
6. Standalone Rocky retains its accepted OLD-primary Stage and disabled physical
   IPv6. Its original saved WG hash3074 and original postwindow endpoint mappings
   anchor strict interface/entire-peer/crypto/keepalive/AllowedIPs projection and
   endpoint comparison using frozen0a8. Only its two configured gateway hosts
   receive four overlay probes; both handshakes must be fresh and public HTTPS200
   must pass. The actual next timer, including OnBoot/calendar, must lie beyond
   former outer deadline +65second observation +45second margin, or be proved
   nonmutating: exact native540b source/root0755, rocky hostname, same-day stamp
   through that margin, pinned root0644 oneshot daily unit36e6 with no loaded
   pre/post/OnSuccess/OnFailure/environment/dropin overrides or test environment.
   The source explicitly refuses automatic standalone-Rocky reboot; a stamped
   daily tick returns without package/service/address changes. Preserve actual
   NEXT and report the explicit admission reason; never change task914.
7. After the former ordinary deadline+65, require all acceptance and more than60
   guard seconds remaining. Release only the owned hash/source/current new-kernel
   claim. Continue monitoring guard0 and control paths through former outer
   deadline+65; require confirmed NEW, no guard/pending, strict original WG and
   protected state. This proves persistent NEW startup and release survival.

## Failure and terminal cleanup

Any required check or additional guard drop fails the trial and invokes trusted
owned OLD recovery while SSH is available. Ordinary pending uses mature rollback;
active guard uses explicit guard restore. Confirm-success/arm-failure, or failure
after release, uses the same validated accepted OLD archive through a short120
guard followed immediately by explicit OLD restore. A failed fallback records
actual NEW plus OLD alias and halts for root coordination; ordinary no-op rollback
must never be described as OLD restoration.

On any pre-reboot failure, including a refusal before arm, the separate cleanup
can start **only normal Rocky**, and only after independently verifying actual
OLD primary/default, no pending/guard, helper pin and unchanged normal VM policy.
It refuses NEW, an active guard or an unexpected running lab VM. It never starts
FreeBSD/labs and never forces a lifecycle action. If reboot was requested and a
new kernel has not returned, perform no blind native writes: retain native guard
recovery, report the actual condition and follow up over the OLD path with root.

After successful release/survival, archive complete accepted/guard/released
journals, original context/program/receipts and controller evidence. Remove only
the two named diagnostic rc files; retain private evidence, revalidate the base
graph and unchanged scoped Prepare/PF ownership. Remove only owned permission
markers and exact probe UIDs, drain their deletions, and finish full health and
normal VM policy checks. Keep OLD aliases, OLD endpoint/source YAML, keys and all
remote peer configurations. No guest promotion or endpoint fanout belongs to this
slice.

Eight offline checks cover delayed-final-query receipt refusal (and its mutant),
300-second trusted snapshot admission, confirm/arm failure fallback, owned-release
admission negatives, unrelated/owned rc drift, strict guest runtime drift, timer/no-op admission negatives, and
normal-VM failure cleanup. Controller and every embedded remote Python program
compile. Accepted capture readiness/drain AST is unchanged. Private result:
`evidence/confirmed-new-boot-source-checks.json`.
