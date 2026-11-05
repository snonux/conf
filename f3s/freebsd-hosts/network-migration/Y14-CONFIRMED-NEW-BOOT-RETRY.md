# f3 confirmed-NEW boot retry — installation refused; identity HOLD

The dormant file-install attempt at21:44:47 refused admission **before mkdir or
any file write**: the frozen context expected `kern.boottime` seconds1793912987,
while the same running system reported1793912986. Read-only verification at
21:46:31 proved all three proposed native objects absent (private directory and
both rc hooks), no address/endpoint/guard/activation state, and normal Rocky
RUNNING. No object was installed or reset; no VM stop, promotion, confirmation,
guard, reboot or release was attempted in this retry.

Root revoked all retry source-install/live GO pending boot-identity diagnosis.
The13 frozen hashes below are historical reviewed inputs to the refused
installation, not current authority to deploy. They have not been silently
refreshed. Continuous monotonic uptime and PID1 show the existing boot continued;
the one-second timestamp change is not a new kernel or address-state drift.

FreeBSD's [timecounter source](https://raw.githubusercontent.com/freebsd/freebsd-src/releng/15.0/sys/kern/kern_tc.c)
documents that its estimated boot time changes with wall-clock adjustments.
The [kernel boot-ID source](https://raw.githubusercontent.com/freebsd/freebsd-src/releng/15.0/sys/kern/kern_mib.c)
provides16 cached per-boot random bytes after entropy is ready. Ordinary textual
`sysctl -n kern.boot_id` does not display this opaque type; correct read-only
`sysctl -b kern.boot_id` returned16 identical bytes across repeated reads.
No identifier bytes or private key values are reproduced here.

The stable-ID source fix and durable generic regressions have since passed the
single full offline checker and root review, committed as7a37ac4. A separately
authorized file-only slice deployed helperce4728 and random-dependent rcefdeb1
at22:20:48UTC. Exact two root:wheel0755 files changed; five other resources and
all OLD protected/network/WG/PF/VM state stayed unchanged. Native syntax,
random-before-rollback-before-NETIF order and unchanged reapply passed; markers
remain absent. No address/guard/VM/reboot action was authorized or performed.
Private receipts are `evidence/boot-identity-file-only-*.json`.

The frozen13-artifact bundle below remains historical and uninstalled. Next work
is a distinct source-only stable-ID consumer/context proposal covering
guard arm/admission, boot claim, watchdog, release and all controller/diagnostic
identity consumers. Timestamp refresh, tolerance and fallback identity guesses
are prohibited. No immutable historical context is changed, and no new consumer
installation or live trial is authorized; opaque read failure must fail closed. Boot time may remain a diagnostic
timestamp, but it cannot establish per-boot identity.

The first actual boot attempt remains failed and fully cleaned up; see
[its record](Y14-CONFIRMED-NEW-BOOT.md). Its original bundle, native causal
receipts, journals and private evidence are preserved. f3 remains OLD-primary
dual Stage. This proposal does not authorize installation or another trial;
root must review the frozen bundle and serialize the native window.

The controller bundle is
`~/.local/state/conf/y14-preparation/confirmed-new-boot-retry-reviewed/`.
`frozen-pins.json` pins all13 artifacts. Its critical entry points are:

| Artifact | SHA-256 |
| --- | --- |
| `driver.py` | `6beda59d0fc06ff42901e63175d37ad1848d08ef85e49df4bdee235d8c66f556` |
| `operator.py` | `25518596b5f5ca3eaecd3b8939809110c046add96c09e3fc2ae45ca5f0dad91f` |
| `public_metadata.py` | `85bb1cd666cee17796e1280580c45730eeeb0667976801eae497a626cbca93cc` |
| Installation manifest | `c73a601031ac9e7d23bfdbffd9075cdfdf13473ab4559a48d965e8bacc6cc09d` |
| Unarmed private context | `f41920071b9e300284b7ea9fa261f22fa307b078a2018a0fb8a4751b0cecf09e` |

The native helper2fb, endpoint projection0a8, current Go controller9603,
configuration, keys, endpoint source YAML, six-line PF guard and task914 are
unchanged. The ordinary300/guard600/full1005-second protocol, trusted original
OLD snapshot, causal NEW startup receipts, postboot NAT/service/guest proofs,
release and former-deadline observation remain required.

The retry uses before-kernel1793912987 and a distinct scope/context directory:
`/var/db/gonf-network-migration/y14-confirmed-new-boot-retry-proof`.
Its only diagnostic rc files are `/etc/rc.d/y14_confirmed_new_boot_retry_proof`
and `/etc/rc.d/y14_confirmed_new_runtime_retry_proof`. All must be absent before
installation; the original evidence directory is neither reset nor reused.
Root:wheel modes stay0700 for the private directory,0600 for context/program,
and0555 for the two rc files. Verify actual installed graph, hashes, owners,
modes, absent receipts and original protected OLD state before any arm.

The reporter uses an explicit flat schema containing only enumerated stages,
booleans, bounded numeric epochs and enumerated error classes. Full context,
exceptions and child diagnostics go only to mode0600 files. Handled failure
exits without an unhandled traceback. The separate pinned operator quarantines
all raw driver stdout/stderr and unexpected operator tracebacks into mode0600
files, and forwards only schema-validated metadata. Launch through this
operator; never stream the driver directly or print raw private logs. Native
proof stdout also contains only explicit booleans/epochs; full receipts remain
private. Actual exception/finally and real operator-process sentinel tests
verify these boundaries, including malformed and oversized output.

After original NEW startup receipts and strict WG/protected-state checks, make
**one bounded diagnostic readiness round** before the measured traffic phase.
It attempts both overlay families in both directions, requires authenticated
SSH and expected identities, preserves any diagnostic loss privately, and
requires guard0. It is not acceptance and does not repeat until success. Then
start a fresh OLD-source capture and run the unchanged strict all-six2/2 checks.
The fixture proves that the same loss tolerated as diagnostic still fails the
fresh strict measurement. Any guard drop or failed requirement restores OLD.
No key, peer, route, keepalive or service repair accompanies this round. The
original first-packet loss remains unexplained; a cold-session cause is only a
hypothesis.

All12 focused checks and controller/embedded remote compilation pass, recorded
privately in `evidence/confirmed-new-boot-retry-source-checks-private.json`.
The only retry native operation was the refused pre-mutation file-install
attempt described above. A future candidate window still needs
actual scheduler completion, fresh full health and all-six source/path checks,
no pending/guard/permission markers, the full1005-second budget before VM stop
and immediately before arm, and postboot calendar/OnBoot admission. Do not
shorten the budget or assume a reserved window is available. Success cleanup
must preserve NEW and must never call `restore-owned`; archive the exact
accepted/released journals and original receipts, remove only the two owned
diagnostic hooks and permission marker, and complete unchanged owner reapply,
health and exact UID cleanup under a separately reviewed concrete procedure.
