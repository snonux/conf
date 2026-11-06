# Deferred management boot role — source candidate

This role has no native installation or acceptance. It is separate from the
unchanged synchronous `management-boot` and full-primary `pre-app` paths. The
new fail-open wrapper must never replace a full-primary transaction guard.
No build, stage, hook installation or reboot follows from these source changes.

The early userdata invocation cannot assume the factory MAC or eth0 UP: the
accepted v33 observation saw another MAC and DOWN at uptime 0.82, before vendor
network initialization. The deferred role leaves that initialization, its MAC
setter, DHCP owner, callbacks and application startup alone. Only this explicit
claim-bound management role permits the vendor application alongside it.

`management-deferred` requires an explicit IPvlan profile whose MAC is the
pinned factory MAC, an existing private Store, no pending primary transaction,
and an unconfirmed/unexpired active canary with one new-boot allowance. Both
management modules, the child, parent upper links, selected table and rule must
be absent. Nonzero existing ownership intent, including OLD-boot intent, is
refused rather than borrowed or silently rewritten. Therefore ordinary pending
`bootstrap` followed by reboot is NOT a supported entry store: bootstrap leaves
nonzero intent. The proposed first trial instead requires a separately reviewed
root file-only setup of explicit authorization/active state with ZERO ownership
intent before reboot, while preserving all prior evidence. No helper action
manufactures that setup or resets a historical store. A historical zero-intent
record with the matching profile may remain. Existing deferred metadata requires
separate explicit reconciliation; a live or unknown continuation is never
replaced. An existing prior watchdog receipt must exactly match the old active
canary before it can be replaced after an actual boot change.

Under the stable Store lock, the early entry consumes the allowance, persists
the actual current boot and exactly `CLOCK_BOOTTIME(now)+120s`, then persists a
separate claim with `wait_until=that same now+30s`. It records an attempt before
each spawn. It launches and verifies the independent `deferred-canary-worker`, reusing the
existing canary deadline/readiness loop, before launching a separately identified detached waiter. `worker.json` remains
the watchdog's identity; `management-waiter.json` belongs only to the waiter.
Neither the claim nor the waiter is a full acceptance receipt.

The early lock acquisition/setup checks have an eight-second context; watchdog
and waiter handshakes have existing five-second and new three-second bounds.
The proposed wrapper additionally bounds the foreground CLI with BusyBox
`timeout -k 1 12` and always returns zero to vendor startup. BusyBox timeout is
PID-scoped: it does not promise to kill every descendant. Successful detached
processes are intentional: the waiter has an immutable readiness deadline and
the independent watchdog retains the original 120-second deadline. Failed
waiter handshakes kill and reap that child. As with the existing code, these
bounds cannot cancel uninterruptible local kernel/filesystem I/O.

The waiter performs two stable readiness observations WITHOUT holding the Store
lock, retrying only those read-only observations within its original 30-second
window. Readiness requires current-boot tmpfs lease provenance, strict pinned
owner/handler/no executable hooks, stable PID/birth/argv/executable, no callback
children or unknown process reads, and actual applied lease/IP/default/DNS plus
factory MAC/UP. It does not manufacture a DHCP completion event or require a
lease timestamp after daemon birth. v35 established steady-state representation
availability, not this waiter's boot timing.

Before any Ensure, the waiter reacquires the same lock and rechecks the exact
claim, active generation/profile/current boot/immutable deadline, consumed
allowance, live watchdog and its own published PID/birth. It refuses pending
primary work, modules/child/upper links/policy/ownership, then takes one immediate
readiness observation under the lock and rechecks the claim/watchdog/clock.
Only then does existing typed Bootstrap/Ensure create the additive path. These
kernel operations retain their existing per-operation bounds and must finish
before the original 120-second deadline; the 30-second limit bounds readiness
waiting rather than extending that canary lease. Clock/watchdog or Ensure
failure attempts typed owned cleanup; cleanup errors remain errors and the
independent watchdog retains its cleanup responsibility.

A failure before watchdog setup completes cannot reach Ensure. A crash between
allowance consumption and full process publication may leave consumed pending
metadata and no usable continuation: startup remains fail-open, reentry cannot
mint another allowance/deadline, and operator inspection is required. After a
verified watchdog exists, loss of the launcher or waiter does not cancel it.
On expiry, the deferred watchdog can retire without physical-parent admission
only after exact profile/claim/current-boot/terminal validation, no pending
primary, ZERO ownership intent, and observed global modules/child/upper links/
table/rule absence. That read-only terminal path changes no parent/network or
ownership bytes: failed factory-MAC/UP readiness alone cannot keep an empty
watchdog retrying forever. Changed claim/profile/boot identity is refused.
For nonzero, foreign or unknown resource observations the role falls back to
existing typed Cleanup, retaining parent admission, preservation/refusal and
retry behavior. A foreign resource that prevents typed cleanup remains unresolved
cleanup/HOLD, never a claim that foreign state was drained. Legacy WorkCanary
continues to pass Adapter.Cleanup directly to the unchanged deadline loop.
Actual watchdog drain must still be verified in a native trial.
Readiness timeout records failure and leaves the original expiry authoritative.
Failed continuation evidence is retained privately; it does not turn an error
into acceptance. Unknown/foreign state is never repaired or reset.

Same-boot reentry only verifies an existing live waiter, or probes a completed
pending path; it does not relaunch, reapply Ensure or renew the deadline. A
second boot cannot reuse the consumed allowance. When an unconfirmed pending
active record belongs to a previous boot and has no allowance, this entry marks
it expired/no-allowance under the lock, preserving its historical boot/deadline
and all ownership bytes. This is only a terminal metadata marker: the previous
watchdog is gone, and no owned cleanup is claimed. After vendor readiness root
must perform explicit reviewed ownership/store reconciliation before another
trial: verify actual current-boot parent readiness, global module/child/table/
rule and owned-process absence, then preserve the whole terminal store in an
exclusive archive under its stable lock before preparing fresh supported input.
Actual cleanup remains unproved until that separately reviewed reconciliation; foreign resources remain preserved/refused. Confirmed/expired authorization
is refused by this role; the separately reviewed terminal cleanup CLI remains
the operator's path on its matching boot. Current historical OLD-boot stores
must not be relabelled just to make cleanup or this role pass.

Native acceptance still needs a separately reviewed source/artifact/store
transition, explicit boot window and attachment/outage scope, complete private
beforeimages, actual early/continuation/watchdog process receipts, fresh fixed
HTTP/SSH and forced media, independent expiry/typed cleanup, unchanged parent
and full authentication/access cleanup. Source tests grant none of those facts.
