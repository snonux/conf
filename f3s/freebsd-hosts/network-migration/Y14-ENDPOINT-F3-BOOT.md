# f3 endpoint actual boot — accepted native slice

Historical reviewed procedure, completed on 2026-11-05 after reverse
confirmation and outer-expiry cleanup. Native boot recovery passed; diagnostic
rc hooks were removed and the scope was restored and archived. Exact receipts
and terminal acceptance are recorded in [Y14-ENDPOINT-NATIVE.md](Y14-ENDPOINT-NATIVE.md#reverse-direction-actual-parent-boot-and-terminal-cleanup).
The procedure below records the authorized slice; it does not authorize a repeat.

The IQ safe-reboot runbook requires graceful `doas vm stopall` first, verifying
no Running guest, then **`doas reboot`**, avoiding rc.shutdown's watchdog path.
Stop guests while no endpoint journal is pending, before arming any lease.
Preserve vm_list=rocky and FreeBSD's normal stopped policy. Check standalone
Rocky's actual updater idle/next before stopping it; do not race its :55 run.

The earlier v14 f3 reboot took about 102 seconds to a new kernel and 192 seconds
to fresh SSH. Propose **inner 480 / outer 900 seconds**, within the existing
maximum bounded-canary lease. A 120-second inner deadline cannot honestly cover
that measured SSH return. No recovery helper source change is needed.

## Exact frozen scope and diagnostic artifacts

Helper remains reviewed 4d71e3f SHA256
`0a8b54b416cd493f04f14c5e8ea095ed70c8ae969044b4af664baa153616c197`;
base runtime hook c07d9d… and system cron 6c21ab… remain unchanged. Before the
first actual boot, root tightened the early hook's explicit ldconfig dependency
in generator 9e459b6; its deployed/frozen SHA256 is
`dead65c0752a30e616ade6f511e1d45b0e354c1949d607b64225301ad62bff49`.
This startup prerequisite correction does not reset completed timer proofs.
New ID `y14_f3_r2_boot_20261105`, same existing r2 identity, OLD .1.122:56709
to NEW .20.122:56709, protocol canary, same saved/wg paths and OLD source receipt.
Exact boot config SHA256:
`7eb3f4a54594080aa2f9e30b08c0d51ca51fa928de32a6537f0430aefac4d2ca`.

Private frozen manifest/source:
`~/.local/state/conf/y14-preparation/endpoint/f3-reverse-reviewed/f3-boot-proposal-manifest.json`.
Proposed three temporary root:wheel instrumentation files:

| Destination | Mode | SHA256 |
|---|---|---|
| /var/db/gonf-endpoint/y14-proof/proof.py | 0700 | 683801881a6e3fb97533615f47bbe27f3fc1f914559b565ee50587bcef2d1b23 |
| /etc/rc.d/y14_endpoint_boot_proof | 0755 | 7206931a9481edc8ae1cff181f6c1c538c594808821c5fd501f8668b7d0477b9 |
| /etc/rc.d/y14_endpoint_runtime_proof | 0755 | 17ecef41daa373f2138d067707501e8cdb176577168ba23bb6c56e02429a221d |

Proof context is root-private 0600 in a 0700 directory and binds the unique
scope/preboot kern.boottime epoch. The Python program reads config/journal/
network state; it writes **only isolated private evidence receipts**, not keys,
endpoints, routes, configuration or journal state. Durable receipts are necessary
because early syslog may not yet be available. They are atomic, mode 0600 and
must not pre-exist; no prior receipt can be overwritten to fabricate a new boot.

The early proof requires the base endpoint rollback and ldconfig, precedes
netif/wireguard, and asserts new kernel boot epoch, before-inner time, phase
boot-restored, saved OLD exact and **wg0 absent**. Runtime proof requires the
base endpoint runtime hook, precedes LOGIN, and asserts selected captured
runtime OLD, exact saved OLD, strict projected peer/interface fields, new boot
and before-inner time. Both invoke package Python3.12 explicitly.

Native Python compile and both shell syntax checks must pass. Inspect actual
rcorder **after adding the diagnostics**: ldconfig before Python-dependent base
early hook/proof, both before netif/wireguard, then base runtime/proof before
LOGIN. This matters: the interpreter has no ELF RPATH and needs /usr/local/lib
libraries. The existing pre-instrumentation order passed, but is not substituted
for the new actual graph. Refuse any new dependency cycle/order failure.

## Coordinated native sequence

1. Previous scope restored/archived, no router/address/endpoint pending work;
   fresh full cluster/storage/DNS/Service/TLS and literal old/new f3/r2 SSH,
   explicit UDP path/strict admission/source OLD receipt. Check f3 actual
   unattended writer, r2 updater and guest updater schedules. Leave the other
   hosts/three k3s members and power/cabling unchanged.
2. With no pending record, install only exact boot config and diagnostics;
   validate hashes/owner/mode/graph. Gracefully stop all f3 guests and verify
   none Running; no force kill or vm_list modification. Preserve stopped policy
   and preboot VM/bridge/tap/sysrc/PF/wg0/primary/default evidence privately.
3. Generate host-clock canary permission inner now+480/outer now+900 with the
   exact config digest. Snapshot/apply, prove saved/live NEW plus actual fresh
   overlay IPv4/IPv6 and old/new management. Recheck enough time before inner,
   no active conflicting writer, all VMs stopped; issue `doas reboot` on f3 only.
4. Independently reconnect through literal old management before inner expiry.
   Require changed kern.boottime and the original durable native receipts:
   saved OLD while wg0 absent before netif/wireguard, then actual runtime OLD
   before LOGIN. Do not invoke those proofs manually to supply missing evidence.
   A later inner/outer cron restore does not count as early boot recovery.
5. Verify exact saved original/selected actual runtime original and strict
   protected fields; f3 dual Stage, local ULA, old primary/default and laptop
   /32 survive. Verify rocky auto-started with its accepted old/new guest Stage,
   FreeBSD remains stopped and no lab guest unexpectedly starts. Full service,
   DNS/NFS/cluster/overlay IPv4/IPv6/TLS checks and fresh management must pass.
6. Explicit restore twice; privately archive full scope with lock held, remove
   only the two introduced diagnostic rc scripts, preserve context/program/
   receipts as private evidence, and verify original rc order. Reapply dormant
   four-file/system-cron plan with no new record/authorization and clean only
   owned probe UIDs. No source promotion, PF guard or primary change follows
   automatically; those remain separate root slices.

## Failure handling

A failed proof is recorded as FAIL, even if normal services later recover.
These rc diagnostics do not themselves stop all FreeBSD boot scripts. Physical
old/new SSH and the existing independent scoped cron restoration remain the
recovery path; do not depend on JetKVM/physical access or move a cable.
Preserve failed journal/receipts, inspect actual phase/order, remove only owned
diagnostics if they interfere, and use the existing scoped restore protocol.
Do not replay a whole stale tunnel, change peer fields, delete a pending record,
force-kill guests or reboot another host to make this trial pass. Root coordinates
recovery and any later retry.
