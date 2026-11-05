# f3 primary boot recovery and final confirmation — next review slices

The documented 600-second OLD-recovery boot slice completed; actual receipts
and terminal acceptance are in [Y14-PRIMARY-NATIVE.md](Y14-PRIMARY-NATIVE.md).
Final confirmation and confirmed-NEW cold-boot recovery tooling remain source
preparation; no repeat boot or final-confirmation permission is implied.
The preceding third 300-second expiry trial also completed independent native
restoration, original-baseline comparison, full health and terminal cleanup.
The two earlier failed attempts remain failures in Y14-PRIMARY-FIRST.md.

## Scope and prerequisites

Only f3/re0 physical primary/default is in scope. Accepted fallback remains
192.168.1.133 primary / 192.168.1.1 default, .20.133 alias, local ULA ::133 and
laptop .99.2/32 via .1.2 on the shared wire. Preserve all saved/runtime endpoints,
crypto, peer sets, AllowedIPs, configured public-gateway endpoints, wg1, resolver,
bridge/VM policy and the exact six-line owned PF guard. No source-YAML promotion,
new router change, physical-port claim, old-alias retirement or other host reboot.

Use committed 88db0d2 native helper SHA256
`ef75c78d2b99c4f8ebf4b758414faa391410b3953736bdd01b9bdb462ead7cd4`.
Its absent-default add/change and post-detachment laptop-return handling have
separate positive/negative fixtures; the earlier actual retry also proved native
OLD default/return restoration. No recovery-protocol bypass or new helper mode.

Fresh admission for each slice: no address/endpoint/router pending work, all
three nodes Ready/all three members reporting a leader, NFS/DNS/API/verified
TLS checks, six-peer IDs/endpoints and bidirectional WG IPv4/IPv6, fresh four
old/new management sessions, source YAML receipt 41386d… and no production
marker. Guard must be active before pass-out, unchanged pretrial counter zero,
with no original or translated NEW-source admission state; preserve ordinary
OLD states. No broad state flush. Any required check or extra guard drop requires
immediate owned address rollback, even when the initiating command errors.

Inspect actual f3 and all relevant host/guest updater processes, daily stamps,
next deadlines and native watcher dispatch. A completed daily stamp plus f3's
actual ALLOW_REBOOT=0 branch is a proven nonmutating hourly tick; an unexplained
missing cron log alone is not proof of execution. Fit the full lease, native
minute cadence and measured postcheck margin before any potentially disruptive
writer. Do not alter task 914 or assume f3 reboots every :15. Root holds other
router/PF/Jet/network and production NFS/CARP writers during a pending trial.

## Separate actual-boot slice: promote shared 600

600 seconds covers measured f3 reboot-to-old-SSH (about 149 seconds on the
endpoint boot) plus initial checks and margin; avoid an unneeded 900-second wait.
Stop VMs before arming, while no record exists: check standalone Rocky updater,
`doas vm stopall`, verify none Running without force, preserve vm_list=rocky and
FreeBSD's ordinary stopped policy. Do not confuse standalone Rocky .123 with
r2/k3s. Recheck all timing after the graceful stop.

Private prepared artifacts are under
`~/.local/state/conf/y14-preparation/primary-boot-reviewed/`: proof.py and two
rc diagnostics. They observe state and write only isolated root-private durable
receipts. Native installation requires a root-reviewed exact manifest; do not
install or activate merely from this proposal.

The private context is 0600 under a 0700 directory, unique scope
`y14_f3_primary_boot_20261105`, preboot kern.boottime, exact original accepted
Stage owned-rc values and address order, protected hashes excluding /etc/rc.conf
ordering, captured original actual runtime WG and selected 600-second deadline.
Preserve the first attempt's original baseline. Before arming require both
receipts absent and no archive already matching the forthcoming deadline.
Capture the actual pending snapshot/deadline immediately before requesting boot.

Install only the reviewed proof/context and two previously absent rc scripts;
all script hashes/modes/root:wheel ownership and native Python3.12 compile/sh
syntax must pass. Inspect actual installed rcorder: base network rollback before
early proof, ldconfig before Python early proof, early proof before netif/routing;
netif/routing/WireGuard/PF and endpoint runtime before runtime proof, before LOGIN.
No service restart or rc-wrapper change is included. Refuse a cycle/order defect.

Arm only fresh root-private activation permission and `promote shared 600`.
Require actual NEW primary/default plus retained aliases/ULA/return, four paths,
unchanged fields, six-peer real traffic and outgoing OLD-source UDP, counter
unchanged, and the now validated line-buffered paired-SYN/PF-tuple/verified-TLS
NEW-source NAT proof. If enough deadline remains, issue normal **doas reboot**
on f3 only. Do not use shutdown -r or force a guest stop.

The early receipt requires a genuinely new kernel, before-deadline execution,
completed native rollback archive with the exact deadline and OLD primary/default
snapshot, absent pending, restored owned rc semantics, wg0 absent and re0 physical
IPv4 still unconfigured. The runtime receipt requires the same original archive,
actual OLD primary/default/.20 alias/ULA/.99.2 return, exact captured runtime WG,
protected fields and active zero-drop guard before LOGIN. Receipts cannot be
manually replayed or overwritten; a later cron restoration cannot substitute.

Fresh old literal SSH must return before the deadline. Verify original durable
receipts and kernel epoch, full health, all six peer paths and protected values.
Rocky must return automatically with accepted Stage; FreeBSD and lab guests must
remain normally stopped and vm_list unchanged. Explicit restore twice; archive
whole native scope; remove only two owned diagnostic rc hooks, preserving private
receipts/context/program; verify original base order and dormant scoped owner
reapply unchanged; remove only own permission and exact probe UIDs. A failed
receipt remains a failure even if services recover. No final primary is accepted
by this recovery test: the expected return state is accepted OLD-primary Stage.

## Separate final-primary slice: promote shared 300, confirm, observe

After successful boot-recovery terminal cleanup and a separate root checkpoint,
repeat the same fresh state/timing/strict-field/old-source-six-peer/NEW-NAT/full
health admission, using the original accepted Stage. Arm shared 300, prove all
pending criteria before expiry, then confirm through the native helper only.
Retain OLD aliases/guard and all source/endpoint protections.

Independently observe the actual NEW primary/.20.1 default beyond the former
300-second deadline plus a full native minute cadence. Require no pending
journal/temporary gate, unchanged guard baseline, original nonaddress fields,
full service/peer/NAT/management health and scoped owner reapply without rollback.
This confirms persistent primary selection and deadline survival; it does not
claim that the recovery test booted a permanently confirmed NEW primary. Any
later confirmed-primary cold-boot acceptance must be stated separately, with its
actual native source-selection evidence and approved recovery scope.

## Destination-scoped source gate; no dependency cycle

Address promotion does not silently promote endpoint source. The generator's
later candidate may change **only hosts.f3.lan.ip**; every other destination stays
OLD in the immutable full manifest. This needs the complete six persisted f3
referrers f0/f1/f2/r0/r1/r2, not other destinations' address promotions or task 924.
The current guard deliberately keeps their OLD transport until this gate passes.

For old-only referrers, a reviewed narrow f3 NEW /32 return route via the shared
router may be needed before an endpoint canary; do not require whole-host/new-LAN
promotion. Each referrer's actual saved/live recovery and unattended/boot contract
must be satisfied and recorded. f3/r2 independent platform proofs do not falsely
establish actual f0/f1/f2/r0/r1 native boot recovery. Keep all six OLD destination
aliases/paths reachable while mixed OLD/NEW referrers are migrated serially.
No automatic fanout, AllowedIPs edit or weakened manifest/recovery assertion is
included. Root must reconcile the exact remaining per-referrer admission and
y14 closure scope against task annotations without introducing a dependency on
the later all-destination network task chain.
