# Device recovery preparation (s14)

**2026-11-07 task724:** Shelly1/2 now confirmed at10.30.0.28/.29. Both
passed native former-deadline, exact configuration/output and routed Pi caller
checks; script1 remains boot-enabled/running, Matter off, no pending restore.
Pi0/Pi1 adopt only the two plug endpoints through operational
`pis_netbsd_shelly_prepare` / `pis_netbsd_shelly_activate` (source0104db1).
The partial configuration preserves other compiled defaults and refuses custom
existing configs rather than overwriting them. Native missing-gate refusal,
normal no-override CLI/CGI, guarded fan OFF/ON and idempotent AC ON passed.
Owned approvals were archived/removed; unchanged-source reapply preserved
controller configuration and protected network state. No MCU reboot or rack
AC cut occurred. The first Shelly2 confirmation window was missed: its native
timer restored OLD and guarded acknowledgement preceded a successful fresh
trial. Private evidence: `~/.local/state/conf/724-devices-20261107/`.

The rack is intentionally shut down for the next storage/endpoint work, with
fans and AC ON. Existing f-host boot fan helper addresses and broader canonical
mappings still belong to task924; this checkpoint does not claim full service
endpoint cutover or cluster restart. Four prepared JetKVM factory-MAC
reservations match source/IQ and native future DHCP syntax passes. Shared-wire
DHCP remains disabled; JetKVM devices and AP management/radio/DHCP are untouched.
Final native review passed with no blocking findings.


**2026-11-06: JetKVM plan simplified on user instruction.** Keep vendor DHCP,
use the four prepared MAC reservations, and verify new leases/device identity/
video/HID after the final human-confirmed rack move. Custom JetKVM helper,
profiler, IPvlan and boot experiments below are historical and must not resume
as migration prerequisites. See IQ `topics/infra/openbsdrouter-plan/JETKVM-DHCP.md`.
The first device is cleaned with an inert helper; incomplete v44 was never run.
Shelly and host recovery requirements remain unchanged.

Initial read-only discovery on **2026-11-05** preceded the reviewed temporary
JetKVM developer-access inspections, which restored the original key and mode.
The user subsequently approved the Shelly Matter transition and fully-down
MCU/control maintenance window. Its actual native checkpoints are recorded in
[Shelly acceptance](SHELLY-NATIVE-2026-11-05.md). Keep s14 open until every
device's required native rollback and service checks have passed.

## Installed evidence

**Discovery baseline before the approved transition:** both authenticated
Shelly Plug M Gen3 devices reported model S3PL-30110EU,
firmware 2.1.0, digest authentication enabled, output on, `initial_state=off`,
Matter enabled, zero scripts and one existing schedule. Both have current
system time and `restart_required=false`. Station 0 is static at the retained
address with /24, gateway/DNS 192.168.1.1; station 1 is disabled.
Read-only `Matter.GetStatus` on the same date reports `num_fabrics=0` and
`commissionable=false` on both devices. Thus neither reports an enrolled
fabric; the commissioning window is closed. This does not establish that
disabling Matter has no integration impact or remove the IQ approval gate.
No setup/pairing code was requested.

| Device | Retained address | Target | Role |
|---|---|---|---|
| shelly1 / 0892725e366c | 192.168.1.28 | 10.30.0.28 | Rack fans |
| shelly2 / 0892725f8a04 | 192.168.1.29 | 10.30.0.29 | f0–f3 and JetKVM AC |

Keep shelly1 on while dependent hosts run and during IPv4 trials. An MCU
restart of shelly2 may open its relay because its
initial state is off, independently of whether a Switch.Set RPC is sent.
No Wi-Fi change or MCU reboot is safe while its hosts are running. Cluster
shutdown alone does not establish that controller/router/AP supplies survive
loss of this AC feed; that remains a separate measured acceptance condition.

All four JetKVM IDs were authenticated through fresh HTTP logins and WebRTC
RPC sessions. App **0.5.10**, system **0.2.8** on every unit. Older IQ 0.5.9
observations remain historical. The read-only `getDevModeState` returns false
on every unit; TCP22 refuses connections. No SSH access has been established.

| Device ID | Current DHCP address | Reserved target | DHCP client | Video |
|---|---|---|---|---|
| e10a0883debf7d4a | 192.168.1.241 | 10.20.0.158 | udhcpc | 1080p ready |
| f72978d7f87aa969 | 192.168.1.244 | 10.20.0.191 | udhcpc | 1080p ready |
| cee6ab4df2f88754 | 192.168.1.245 | 10.20.0.198 | udhcpc | 1080p ready |
| 46793b0d4c8d45d | 192.168.1.248 | 10.20.0.151 | jetdhcpc | no_lock |

The returned MACs match the dated IQ ledger. IPv4 mode is DHCP and IPv6 mode
SLAAC on all four; USB state is configured. Only the last device is mapped
to f2. Do not infer other attachments or treat video-state RPC as an actual
view of the console. DHCP addresses must be rediscovered on subsequent runs.

## Supported mechanisms and unresolved limits

The [model-specific Shelly KB](https://kb.shelly.cloud/knowledge-base/shelly-plug-m-gen3)
limits scripting to Matter-disabled operation. The generic
[Script service](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Script/)
supports boot-enabled scripts. Installed firmware lists Script.Start and
Script.List succeeds, but neither establishes that scripts can execute with
Matter enabled. No Create/Start test was attempted. The IQ recovery ledger
explicitly says: “Do not disable Matter or another integration to obtain
rollback without separate approval.” Preserve Matter while evaluating the
following equivalent mechanisms.

The [Schedule service](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Schedule/)
supports cron jobs invoking RPC methods. Both installed devices advertise
Schedule.Create/List; their existing jobs were counted without printing or
changing job bodies. A recurring job restoring only old station IPv4 fields
is a **candidate** timed/persistent fallback, using authenticated deletion of
the exact new job ID as confirmation. Preserve the existing schedule, its
enabled state and all outputs. Unlike an absolute one-shot deadline, a
recurring job could still run after a reboot has passed the first deadline.
It must be proven to fire while unsynchronized and network-isolated before
it qualifies as independent boot recovery. The vendor describes local-time
cron execution, not a monotonic boot timer, and
[Sys status](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Sys/)
documents null time before NTP synchronization. The reviewed docs provide no
guarantee for this failure condition. Current valid time is not proof after
reboot. Do not count a Schedule.Create response as working network rollback.

[Wi-Fi station 1](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/WiFi/)
is a fallback for failure to connect to station 0's network. It is not a
second simultaneous IPv4 address or a reachability-confirmation timer.
Keeping the same working SSID with a wrong IPv4 route can leave association
successful; this mechanism alone does not cover renumbering failure.
The built-in AP would require an independently tested remote Wi-Fi client
and power/route, and changes transport under the current no-AP-change scope;
it is not established recovery here. An external polling job also cannot
restore a device whose only address is unreachable. DHCP reservation/lease
expiry does not meet the independent bounded recovery requirement.

JetKVM vendor [developer tools](https://jetkvm.com/docs/advanced-usage/developing)
provide key-based SSH on the full device. They do not specify a persistent
user boot hook. Installed-version upstream source was inspected at
[f152581df82c519ecc4349f471dfc8612fbf0c48](https://github.com/jetkvm/kvm/tree/f152581df82c519ecc4349f471dfc8612fbf0c48),
the release/0.5.10 tag. In
[network.go](https://github.com/jetkvm/kvm/blob/f152581df82c519ecc4349f471dfc8612fbf0c48/network.go),
`rpcSetNetworkSettings` applies/saves state and can reboot immediately;
IPv4-mode/static-address changes require reboot. Its browser post-reboot
healthcheck/redirect is not automatic rollback. Do not use that setter as a
harmless staging command. Source shows network state within
`/userdata/kvm_config.json`; developer state at
`/userdata/jetkvm/devmode.enable`, keys under `/userdata/dropbear/.ssh/`.
These paths are source evidence, not verified installed mounts or boot hooks.
The full config can contain credentials and must never be printed or committed.

No built-in confirmed timed+boot revert was established for these installed
JetKVMs. Generic BusyBox init assumptions, app crash recovery and firmware
recovery modes do not prove network-config restoration. Any custom hook must
be inspected on system 0.2.8, tested before renumbering, and revalidated after
app/system upgrades. Do not update firmware to obtain recovery in this task.

## Next reviewed trials, strictly serial

These are review inputs; this directory deliberately contains no device
mutation or reboot implementation. Root coordinates timing with host trials.

1. Preserve current cabling, AP management/radio/SSID and controller old IP.
   Save private snapshots; reserve both address allocations. Establish fresh
   digest status reads and JetKVM ID checks immediately before each trial.
   Keep router, AP, controller Pis and laptop access independent of f-host AC.
2. After host rollback trials and safe cluster shutdown, evaluate Shelly
   Schedule without changing Matter. First add an explicitly identified,
   bounded diagnostic job calling a harmless read method, observe execution
   through private logs, then delete only that job and verify the preexisting
   schedule unchanged. Capability enumeration alone does not test execution.
3. Before any address change, determine whether that job survives MCU reboot
   and executes with no DNS/NTP/network access. Reboot only within the approved
   power window, with output-state implications explicitly handled. Keep fan
   output stable during IP trials and while cooled hosts are running; planned
   MCU boot/control trials belong after safe shutdown and independent power
   verification. If the clock requirement
   cannot be met, stop: Schedule cannot be accepted as boot fallback.
4. Only if that proof succeeds, prepare an exact recurring old-IP restore
   job per device, changing only station IPv4 mode/address/mask/gateway/DNS,
   never SSID/password, Matter, Switch settings or output. Use a reviewed
   window and exact-ID cancellation after a fresh authenticated new-IP check.
   Test timeout, repeated idempotent restoration, confirmation, and boot
   fallback separately on each installed device. Wi-Fi credentials are
   write-only; do not replay redacted Wifi.GetConfig as a whole config.
5. For JetKVM, review enabling developer mode plus one temporary public key
   on one identified canary. First inspect filesystem/mount persistence,
   boot order and network-owner/service behavior using read-only SSH commands.
   Do not install a guessed /etc/init.d hook. Prepare a device-local detached
   monotonic timer and persistent pending marker/boot restore only after a
   supported installed hook is established. Privately back up network state
   and preserve unrelated authentication, USB/video and host-power settings.
6. Prove timer expiry and boot restoration first using a reserved additive
   canary address if installed networking supports it. Restore both running
   and persistent state, verify fresh old-IP authenticated ID access, test
   cancellation and repeated rollback. For original DHCP state, restoring
   DHCP alone is insufficient: prove which old address/path will be bounded
   and collision-free, or establish a tested retained management alias before
   primary renumbering. Reboot trials cannot assume the lease is unchanged.
7. Only after every device's own recovery passes, test candidate routed
   access and scheduled power/fan control with the cluster down; root owns
   shutdown/startup and power acceptance. Shelly targets remain Wi-Fi
   10.30.0.28/.29. Test only approved source/destination pairs, literal TCP80,
   digest auth and output read-back. Recheck JetKVM HTTP/WebRTC from wg2 and
   derive actual negotiated transport ports before proposing firewall changes.
8. Restore each device's previous developer mode/key state after acceptance,
   without removing any still-required recovery hook. If disabling developer
   mode would prevent an armed rollback, defer cleanup until it is confirmed
   and disarmed. Keep old allocations until explicit retirement.

**Initial discovery blockers:** no proven Matter-preserving Shelly boot restore;
Schedule offline-boot behavior untested; JetKVM's installed userdata boot hook
is verified on one canary, but no recovery helper is installed or exercised.
Safe owner restart/watchdog behavior and deterministic old DHCP recovery remain
unproved; see [the JetKVM design](jetkvm/README.md). Temporary developer mode
is restored off after each inspection. s14 and device-renumbering acceptance remain open.
No blind replacement is authorized by this plan.

## Narrow JetKVM first step for root review

This step only enables authenticated inspection on one canary. It does not
install rollback, alter networking, reboot, or touch USB/video. IQ permits
establishing developer SSH as part of s14; root still reviews the exact live
operation. Candidate canary is `e10a0883debf7d4a` at observed `.241`; verify
the ID and current address again first. Its f-host attachment stays unknown.

Pinned source is app commit `f152581df82c519ecc4349f471dfc8612fbf0c48` and
system 0.2.8 tag commit `dc2b6d967fc16aa59b412ae306eb34f4c1ec3d0b`.
[App RPC handlers](https://github.com/jetkvm/kvm/blob/f152581df82c519ecc4349f471dfc8612fbf0c48/jsonrpc.go)
show key enrollment overwrites the entire authorized_keys file, not an
append operation. Developer mode creates/removes its marker and invokes
`dropbear.sh`; there is no direct reboot or USB/video operation in either
handler. The mode setter's failure text suggests a reboot, but **failure is
a stop condition**, not permission to reboot. Existing marker on enable is
an early return, so repeated enable is not a reliable daemon restart.

The pinned system
[dropbear.sh](https://github.com/jetkvm/rv1106-system/blob/dc2b6d967fc16aa59b412ae306eb34f4c1ec3d0b/project/app/jetkvm/dropbear.sh)
creates persistent SSH directories/symlinks and launches `dropbear -R -E`.
Disable runs `killall dropbear`. It has no reboot/USB/video commands.
`-R` can generate missing server host keys. This source lacks an explicit
`-s` password-disable flag; vendor documentation says key-only access, but
installed daemon auth methods must be checked rather than inferred from
this script. Do not attempt device-password or empty-password SSH login.

Reviewable authenticated sequence (all RPC payloads go through the existing
logged-in WebRTC `rpc` data channel, not guessed HTTP RPC URLs):

1. HTTP `POST /auth/login-local` reads `~/.jetkvm` internally; retain its
   cookie only in memory. Exchange the video-recvonly WebRTC offer through
   `/webrtc/signaling/client` and create only the `rpc` data channel. Read
   `getDeviceID`, `getLocalVersion`, `getDevModeState`, `getVideoState` and
   `getUSBState`. Abort on wrong identity, version drift or unexpected state.
2. Read `getSSHKeyState {}`. Save its complete string privately (0700 state
   directory, 0600 file), never stdout. Record only whether it was empty and
   its hash. Prepare a temporary local Ed25519 key; preserve the private key
   locally with mode0600. Construct prior authorized-key text plus the one
   temporary public key, preserving prior content and separating newlines.
3. Send exactly `{"jsonrpc":"2.0","id":101,"method":"setSSHKeyState","params":{"sshKey":"<saved content plus temporary PUBLIC key>"}}`.
   Read `getSSHKeyState` privately and compare exact bytes. Existing keys must
   remain. On any error, restore the saved text through the same setter and
   stop. An empty original is restored with `sshKey:""` (deletes the file).
4. Send exactly `{"jsonrpc":"2.0","id":102,"method":"setDevModeState","params":{"enabled":true}}`.
   Require success, then read `getDevModeState`; do not reboot on failure.
   On failure, restore the saved false mode and key content over the still
   authenticated RPC, verify read-backs and stop. Record the first-use SSH
   host key in a separate private known_hosts file after the authenticated
   device-ID/IP check. This is TOFU, not an independently attested host key;
   pin the recorded fingerprint for subsequent inspection connections.
   First use an SSH `none` authentication request to inspect the server's
   offered methods without submitting a password. If password or
   keyboard-interactive is offered, disable mode immediately via the still
   authenticated RPC and stop for security review; key-only behavior has not
   been established. No password is sent in this check.
5. Connect as root using only that temporary identity, `IdentitiesOnly=yes`,
   `BatchMode=yes`, `PasswordAuthentication=no`,
   `KbdInteractiveAuthentication=no`. First commands are read-only: identify
   system version; inspect `mount`, `ip -4 addr show dev eth0`,
   `ip -4 route show`, `/proc/uptime`, and exact init/dropbear script paths.
   Inventory boot files and inspect only those needed to identify startup
   ordering and persistence. Do not run a full config dump, process-command
   dump or environment dump that may reveal credentials. Inspect only the
   network subtree privately if needed. Do not start/restart any service.
6. Recheck fresh HTTP identity, video and USB state. For inspection-only
   cleanup, set developer mode back to its saved false value through RPC,
   restore the exact saved key content, verify both read-backs and TCP22
   refusal. Keep a private evidence record of errors. If the HTTP session
   fails, do not improvise network/USB/reboot actions.

This procedure exposes SSH briefly as an intended developer-access change;
the advertised authentication check occurs immediately after enable. Source
inspection provides no broader guarantee about installed build options or
unseen firmware modifications. Root should assess that bounded exposure when
scheduling this first step. No permission question is needed merely because
the step uses task-authorized developer mode.

### Reviewed canary attempt, 2026-11-05

Root approved exactly the sequence above after independent review. The
canary ID, app0.5.10/system0.2.8, disabled developer state and original empty
authorized_keys were read and privately saved. One temporary public key was
enrolled and exact read-back matched. The developer-mode enable RPC then
timed out after eight seconds without a reply. The required failure path ran:
the still-authenticated RPC session disabled developer mode, restored the
original empty key state and verified exact read-backs of both. An independent
TCP22 check afterward returned connection refused. No SSH authentication or
filesystem inspection occurred. No reboot, retry, network/service/USB/power
operation was attempted. Root must review any changed trial sequence.

The source handler runs `dropbear.sh` via Go `CombinedOutput`, and its daemon
launch uses `-E`. A child retaining the captured stderr pipe could delay the
enable RPC response until disable kills Dropbear. This is an unverified
source-based explanation of the timeout, not evidence of successful SSH or
permission to ignore the success condition. The private evidence directory is
`~/.local/state/conf/s14-jetkvm-inspection-20261105/` (0700, files0600).
The original empty authorized_keys content and its hash were preserved there;
no credential or HTTP authorization value was printed.

### Revised effective-enable trial and installed authentication gate

The initial sequential helper required a reply before making the independent
mode read. The revised private helper is `/tmp/conf-s14-inspect-v2.py`;
root reviewed and approved its exact one-shot execution on the same canary.
The helper's actual RPC functions passed offline concurrent-ID,
out-of-order-response, timeout-preservation and late-drain checks.

Pinned [webrtc.go](https://github.com/jetkvm/kvm/blob/f152581df82c519ecc4349f471dfc8612fbf0c48/webrtc.go)
dispatches each received RPC in its own goroutine. Thus the mode read and
disable are not inherently queued behind a blocked enable. The setter writes
the flag before invoking the shell; after daemon launch, it makes no later
flag write. [Go exec documentation](https://pkg.go.dev/os/exec#Cmd)
describes waiting on child-held output pipes when WaitDelay is zero.
Together with CombinedOutput and Dropbear `-E`, this supports the inherited
stderr hypothesis, but does not establish the installed daemon's exact
behavior. No diagnostic process, service change or reboot is needed to try
the revised bounded observation.

After the same identity/version/key preflight, send exactly one enable
request and keep its future registered under an immutable request ID.
Do not cancel or discard it after a caller timeout. Within five seconds,
require a separate authenticated `getDevModeState` result of true and TCP22
open. Those establish effective enable, not SSH authentication. Immediately
perform the actual OpenSSH none-auth request (five-second subprocess bound).
Only an exclusively publickey advertised set permits the temporary-key
read-only SSH command (20-second bound). A 35-second outer scope bounds the
whole enabled inspection. All other preflight, access and state-preservation
conditions above still apply. No second enable is sent.

Finally send disable as an independent RPC (eight-second reply bound), then
wait up to eight seconds for the original enable request to reach a terminal
response. An error reply is still terminal; the response is retained privately
without printing arbitrary firmware error text. Only after that response may
false mode and closed TCP22 be called verified cleanup. If the original is
terminal but mode or listener is unexpectedly active, one corrective disable
is allowed by this proposed cleanup sequence, followed by another readback.
Restore the exact saved authorized-key content regardless of drain outcome.

This ordering addresses a race: disabling before an in-flight startup shell
has reached its daemon launch could otherwise leave a late listener. If the
enable request will not terminate after disable, record that unresolved
state plus fresh mode/TCP observations and stop. Do not infer its PID, issue
guessed shell commands, reboot, retry enable, or declare cleanup complete.
The existing authenticated RPC interface has no established process-list
method, so unresolved command ownership would require another reviewed step.

**Observed outcome, 2026-11-05:** authenticated mode=true plus open TCP22
established effective enable while its original request remained pending.
The immediate OpenSSH none-auth exchange advertised **password and publickey**.
The helper therefore stopped before any SSH password or public-key login and
did not inspect the filesystem. Disable completed; the original enable request
then reached a terminal response, and mode=false/TCP22-closed plus the exact
original empty key content were verified. Cleanup reported no errors. A
subsequent independent TCP22 probe also returned connection refused.

The vendor key-only statement does not mean password disappears from the
compiled method advertisement. Subsequent pinned source review and root's
v3 authorization resolved this distinction through a mandatory effective-root
classification before client-publickey-only SSH inspection. Installed root
was locked and used shadow; the source rejects locked password fields. No
password or empty-password attempt occurred. See the separate
[security review](jetkvm-security-review.md) for source paths and evidence.

Root then approved v4 with only the read command and private state path
changed. Installed S21appinit invokes /oem/usr/bin/RkLunch.sh; S20linkmount
mounts userdata ext4. Installed RkLunch includes the /userdata/init.d/S??*
hook before post_chk starts network_init, Dropbear and the app. That hook
directory is currently absent. Both trials ended with the original enable
request terminal, developer mode false, TCP22 closed, original empty key
restored, and identity/USB/video checks passed. No hook or rollback was
installed. v4's exit1 only reflects the absent optional hook-directory listing.

## Matter-off Shelly transition design (approved 2026-11-05)

The user approved this transition and its fully shut-down MCU/control window
on 2026-11-05. Actual execution is tracked in the linked native record above;
the design below preserves the original rationale and guarded protocol.
The required IQ approval was obtained. Pre-transition metadata was zero enrolled fabrics
on both devices, but disabling the feature changes integration capability.
Neither fabric reset nor pairing-code access is needed.

The exact authenticated RPC would be `Matter.SetConfig` with
`{"config":{"enable":false}}`, individually on one identified device.
The vendor API's example reports `restart_required:false`, while its Gen3
integration guidance describes reboot on enabling Matter. Neither documents
the installed 2.1.0 disable path sufficiently to promise no reboot. Treat the
setting itself as potentially rebooting or requiring a later `Shelly.Reboot`;
use both the response flag and fresh `Sys.GetStatus`/`Matter.GetConfig`
read-backs. A no-restart response does not authorize a precautionary reboot.
No firmware update or downgrade is part of this proposal.

There is an explicit output constraint before this setting can be attempted:
both relays have `initial_state=off`. A restart can open shelly2's AC relay,
and can interrupt shelly1's fans. Preserving `Switch.GetConfig` does not mean
preserving actual output across MCU restart. Changing initial_state to `on`
or `restore` would be an additional Switch configuration change and still
would not guarantee uninterrupted mains during MCU initialization.

For shelly2, root must first finish host trials, gracefully stop workloads,
VMs and all f0–f3, and verify the approved independent controller/router/AP
power path with the cluster down. The maintenance window must explicitly
allow possible AC interruption and authenticated AC restoration after the
MCU reconnects; no automatic host/VM restart is assumed safe. Keep shelly1 on
throughout. Never attempt this setting while shelly2's hosts are running.

For shelly1, keep fans on during address/script trials and while cooled hosts
run. The earlier absolute always-on interpretation was stricter than the IQ
source: it requires output stability during renumbering and explicitly calls
for power/fan control tests with the cluster down. Root clarified that a
disclosed MCU boot/control trial after safe f0–f3 shutdown, independent power
proof and restoration of outputs before startup fits that existing maintenance
scope. No new fan-only approval gate is inferred. Matter itself still needs
the explicit separate approval required by IQ; its concrete plan must disclose
possible relay-off during that planned shutdown window. No power change has
been performed in this work.

If that transition and its output effects are separately accepted, the
recovery implementation would use:

- `Script.Create {"name":"conf-network-recovery"}` to obtain a new dedicated
  slot; never replace an existing script or schedule. Upload reviewed code
  using Script.PutCode, then `Script.SetConfig` with that ID and
  `{"enable":true}`. Start with no pending marker and prove the script running
  before changing any IP. Enable means boot startup, not proof it has run.
- Per-script [persistent storage](https://shelly-api-docs.shelly.cloud/gen2/Scripts/APIs/Shelly/)
  for a small record containing version, device ID, saved station IPv4 fields
  and pending state. No SSID, Wi-Fi password, digest credential or output
  setting belongs in the record. Script.storage is documented since1.5.0;
  availability and persistence must still be tested on2.1.0.
- An exposed script function `armRestore(120000)`, invoked only through
  authenticated `Script.Eval` after the script is already running, would
  first persist the old IPv4-only snapshot and pending marker, then start a
  two-minute [Timer.set](https://shelly-api-docs.shelly.cloud/gen2/Scripts/APIs/Timer/)
  one-shot. The Timer API measures its next invocation in uptime milliseconds,
  so this candidate runtime timeout does not require calendar/NTP time.
  Controller code must verify armed status before any separate Wifi.SetConfig.
- On boot, a pending record immediately invokes Wifi.SetConfig for only
  `sta.ipv4mode`, `ip`, `netmask`, `gw`, `nameserver`. A failed restore retains
  pending state and schedules a bounded retry without clearing recovery.
  A successful read-back records restored status. It never invokes Switch,
  Matter, factory reset, OTA, or reboot. It never prints raw config/errors.
  If configuration already matches saved values, avoid another reconnection.
- `confirmRestore()` rejects confirmation once restoration has begun; while
  armed it persists pending=false before cancelling the runtime timer. The
  controller calls it only following fresh authenticated new-IP identity and
  output checks. A reboot after durable confirmation must not restore old IP.
  Retain the saved snapshot until acceptance and explicit cleanup. A power
  loss racing confirmation must conservatively leave recovery pending.

The original old station values are .28/.29, /24, gateway/DNS192.168.1.1.
Save those exact installed values rather than a generic default. Configure
only IPv4 fields on the existing SSID; Wi-Fi credentials are write-only and
must not be replayed from a redacted GetConfig response. Keep digest enabled.

Before IP replacement, fixtures must exercise timer expiry, startup with a
pending record, confirmation-before-expiry, rejection during restoration,
restore error/retry and malformed-record handling. Live acceptance must then
prove the same on each installed device: expiry without a controller session,
idempotent old-IP restoration, fresh reconnect, cancellation, and boot pending
recovery with network/NTP unavailable. Installed power and fan constraints
apply to every such trial. No recovery script has been uploaded and no live
script proof has occurred. The prepared
[script and controller protocol](shelly-script-protocol.md) now implement this
contract with offline fixtures; uploaded firmware execution remains unverified.

Do not re-enable Matter while this script is the required recovery path;
doing so could disable scripting. After device/network acceptance and final
recovery retirement, remove only this script's artifacts and, if separately
approved, restore prior Matter=true with its own reboot/output assessment.
If this lifecycle is unacceptable, retain the existing addresses and record
the device as unsupported rather than weakening the recovery requirement.

## Deterministic retained old-LAN recovery candidate

Consider `192.168.1.140–143/24`, mapped explicitly by device ID, **only as
unallocated candidates**. They sit below both current old-LAN server pools
(.200–239 and .240–254) and outside the former OpenWrt .150–199 pool. They
are absent from the reviewed fixed inventory; that does not prove they are
free or reserved. Reconcile active reservations and outstanding historical
leases on both DHCP authorities, then perform fresh duplicate-address/ARP
checks before allocation. Vivacom's lease database was not inspected here,
so availability is unresolved. Do not use the current .241/.244/.245/.248
leases as permanent static aliases or claim a silent ping proves freedom.

Assigning a recovery alias would be a separate reviewed live step after SSH
inspection and discovery of an actual boot hook. It must add to the working
DHCP address first, prove fresh authenticated connections to both, survive
lease renewal, app/network restart and reboot, and remain reachable through
the current shared wire. No default-route change is needed for a same-LAN
controller; routed wg2 access requires a separately verified return route
and scoped PF rule. The 10.98 subnet remains igc1-only and is unsuitable as
a shared-port0 replacement. The old-LAN alias does not prove port1 recovery.

There is a specific source-level obstacle: app0.5.10
[`ReconcileLink`](https://github.com/jetkvm/kvm/blob/f152581df82c519ecc4349f471dfc8612fbf0c48/pkg/nmlite/link/manager.go)
removes non-link-local addresses outside its expected list. Static config
contains one IPv4 address, and the jetdhcpc lease path supplies a single
expected IPv4 entry. Thus a shell-added alias can disappear during owner
reconciliation, especially on the f2 device already using jetdhcpc. A boot
script that adds it once, or a periodic add race, is insufficient evidence.
Do not patch upstream app code to obtain multi-address behavior in s14.

If a supported persistent alias cannot coexist, a device-local timer/boot
restore might instead restore a preverified, collision-free old-LAN static
recovery primary at .140–143. This establishes a revised deterministic
recovery baseline, not a claim that restored DHCP returns its old lease.
It still requires a tested hook before replacing the working address and
separate acceptance of that baseline. Do not overwrite the only usable
address to set it up. Stop if no supported additive staging or independent
restoration can be established.

## Reproducible checks

`python3 f3s/device-recovery/check-offline.py` validates the read-only RPC
allowlist and redaction on synthetic secret-bearing responses. It contacts
no device. `python3 f3s/device-recovery/shelly-probe.py 192.168.1.28 192.168.1.29`
uses digest auth internally, disables proxies/redirects and emits only selected
non-secret fields. It never prints full configs, error messages or job bodies.
The offline check and live read-only probe passed on 2026-11-05.

JetKVM discovery reused IQ's read-only `scripts/jetkvm-probe.py` with the
existing `/tmp/conf-jetkvm-check` venv (aiohttp/aiortc). Additional read-only
methods were `getDevModeState` and `getNetworkSettings`; only relevant scalar
network evidence is retained above. Do not print a full general settings RPC.
No timed/boot rollback, output-control, remote console interaction, routed
negative test or power-independence trial passed or was attempted here.
