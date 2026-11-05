# Prepared Shelly IPv4 rollback script

`shelly-network-recovery.js` is reviewable offline code, not an installed script.
It deliberately refuses to arm until EXPECTED_ID is replaced with the exact
independently authenticated device ID. It supports the current static IPv4
station configuration; DHCP restoration is refused because its old address
would not be deterministic. It never handles Wi-Fi credentials or output state.

## Documented APIs and compatibility

The vendor [Script service](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Script/)
defines boot enable, upload/start and authenticated Script.Eval. Its
[Core API](https://shelly-api-docs.shelly.cloud/gen2/Scripts/APIs/Shelly/)
documents Script.storage as per-script persistent string storage since1.5.0,
with 16-byte keys, 1024-byte values and12 items. The implementation uses one
11-byte key and a small validated record. Deleting the script clears this
storage. There is no documented flash-flush RPC or atomic power-loss guarantee;
setItem/readback is checked, but actual reboot persistence remains a live gate.

[Timer](https://shelly-api-docs.shelly.cloud/gen2/Scripts/APIs/Timer/)
accepts millisecond delays and reports its next invocation in uptime
milliseconds. The code normally uses one timer and one local RPC at a time, without
DNS, wall time, NTP, HTTP callbacks or a controller heartbeat. Boot starts
restoration immediately for a pending record, rather than restarting its
two-minute confirmation window.

The vendor [language reference](https://shelly-api-docs.shelly.cloud/gen2/Scripts/LanguageReference/)
supports functions, let, JSON, Object.keys and exceptions, but not hoisting,
promises, async or classes. Code calls startup only after defining every
function and uses named top-level callbacks, avoiding nested anonymous
callback limitations. It uses a conservative subset compatible with the
documented interpreter; Node fixtures do not prove firmware compatibility.
Script.storage is documented before installed2.1.0, but Matter-off script
availability and exact runtime behavior still require installed verification.

## Guarded controller sequence (not executed)

1. Root finishes serial host rollback trials and schedules safe shutdown of
   workloads, VMs and f0–f3. Prove that router/AP/Pis/controller access is
   independent of shelly2 AC. Keep outputs stable during IP trials. Disclose
   possible MCU relay-off during the separate boot/control window and restore
   AC/fans before startup. No AP/controller address/radio/SSID change is needed.
2. Obtain the specific IQ-required Matter approval before that setting changes.
   Target one device, read its ID/model/version, digest/output/initial state,
   Matter state, schedules and script list. Privately save current config and
   current allocated script IDs. Preserve existing schedules and integrations.
3. Change only Matter enable=false with authenticated Matter.SetConfig as
   described in README. Treat this setting as potentially rebooting; inspect
   response restart_required and fresh config/system state. Do not factory
   reset, request pairing codes, or update firmware. Reboot only in the reviewed
   shutdown/control window if actually required to obtain script operation.
4. Set EXPECTED_ID to that verified ID in a copy of the script. Create one
   dedicated slot, upload code while stopped, enable boot startup and start
   it. Fresh Script.GetStatus must report running without errors.
   `Script.Eval` code `JSON.stringify(recoveryStatus())` must report idle,
   pending=false and error=null. Startup itself issues no network write when
   no pending record exists. No existing script is replaced.
5. Invoke `Script.Eval` code
   `JSON.stringify({armed:armRestore(120000),status:recoveryStatus()})`.
   Require armed=true, phase=armed, pending=true, error=null before a separate
   IPv4 change. This snapshots only current sta.ipv4mode/ip/netmask/gw/nameserver.
   Storage errors, invalid/static-disabled station or wrong device ID refuse
   arming. A timer allocation failure leaves a pending boot marker and reports
   false; do not change an IP after that failure.
6. Initially test on the existing shared wire, one device at a time, with
   all original IPv4 allocations retained. Change only the same station's
   reviewed IPv4 fields; preserve SSID/pass, digest and all Switch fields.
   A running timeout invokes only Wifi.SetConfig for the saved five fields.
   Do not confirm the first expiry trial; establish a fresh authenticated
   old-IP session and verify saved config, runtime IP, output and script state.
7. Separately test confirmation: a fresh authenticated new-IP ID and output
   check precedes Script.Eval `JSON.stringify(confirmRestore())`. Only true
   accepts confirmation. It first persists pending=false, then clears the
   timer; a queued callback also observes false and does nothing. Confirm at
   or after its uptime deadline is rejected even if the callback is queued.
   Once restoration starts, confirmation is refused. A false/error response
   means do not declare acceptance or remove recovery.
   Timer userdata also carries a generation token: a queued callback from an
   earlier confirmed trial cannot restore a newly armed trial. If clearing an
   old timer failed, its eventual callback is harmless for the same reason.
8. Test boot pending separately in the planned shutdown/control window,
   including unavailable network/NTP, then reconnect through restored old IP.
   For an already-restored pending record, boot verifies saved config/runtime
   IP without causing another reconnection. Pending remains true after restore.
   After a fresh authenticated old-IP ID/output/config/runtime check, invoke
   Script.Eval `JSON.stringify(acknowledgeRestored())`. Only true closes that
   trial: the script independently requires restored phase, matching device,
   exact saved configuration and old runtime IP before persisting pending=false.
   Armed, restoring, mismatched or storage-error state refuses acknowledgement.
   It can then be armed for the next expiry/boot/confirmation trial; no raw
   storage edits, slot deletion or lost snapshot are needed between trials.
   Never delete an armed slot; deletion clears persistent recovery storage.
9. Repeated restore errors retain the pending marker and retry at five-second
   intervals; matching saved config waits for the old runtime IP at one-second
   intervals. At most one local RPC is in flight. A silent local RPC that never
   calls back remains pending, not falsely restored; reboot recovery still has
   its marker. A restart_required response reports fault and never reboots.
   Malformed records/wrong identity report fault without applying guessed data.
10. Only live expiry, cancellation, boot persistence, read-back/output and
    fresh reconnect acceptance qualify this installed device. Restore all
    outputs before host startup. Keep script boot enabled/Matter disabled while
    this recovery path is still required. Any later Matter restoration needs
    its own reviewed lifecycle; it must not silently disable an armed script.

## Exact RPC framing and controller acceptance

The controller uses digest-authenticated HTTP POST to the literal device's
`/rpc` endpoint. A frame is `{"id":1,"method":"Script.Create","params":{"name":"conf-network-recovery"}}`;
increment request IDs and use the actual returned slot ID for every later
request. Never print credentials, Authorization headers or arbitrary firmware
error payloads. Fixed expressions below return JSON text; validate the RPC
response and parse its result as JSON. Unexpected response/type/error stops
the trial; it never authorizes the next mutation.

| Method | Exact params pattern | Required acceptance |
|---|---|---|
| Matter.SetConfig | `{"config":{"enable":false}}` | Approved transition; fresh Matter.GetConfig false and Sys.GetStatus restart state assessed |
| Script.Create | `{"name":"conf-network-recovery"}` | New unoccupied slot ID; preserve initial slot inventory |
| Script.PutCode | `{"id":SLOT,"code":CHUNK,"append":false}` first, `append:true` later | Sequential ASCII chunks of at most1024 bytes from the reviewed script, with only EXPECTED_ID assignment changed; require success on each |
| Script.GetCode | `{"id":SLOT,"offset":OFFSET,"len":1024}` | Reassemble until left=0; compare exact uploaded bytes before start |
| Script.SetConfig | `{"id":SLOT,"config":{"enable":true}}` | Script.GetConfig reports enable=true |
| Script.Start | `{"id":SLOT}` | Script.GetStatus running=true, no error |
| Script.Eval | `{"id":SLOT,"code":"JSON.stringify(recoveryStatus())"}` | Initially exact expected device, idle, pending=false, error=null |
| Script.Eval | `{"id":SLOT,"code":"JSON.stringify({armed:armRestore(120000),status:recoveryStatus()})"}` | armed=true and matching device/armed/pending=true/error=null before any IP write |
| Wifi.SetConfig | `{"config":{"sta":{"ipv4mode":"static","ip":TARGET,"netmask":"255.255.255.0","gw":"10.30.0.1","nameserver":"10.30.0.1"}}}` | Only reviewed target .28 or .29 on10.30.0; current shared-router path/PF acceptance prepared first; no SSID/pass/enable or Switch fields |
| Script.Eval | `{"id":SLOT,"code":"JSON.stringify(confirmRestore())"}` | true only after fresh new-IP authenticated identity/output acceptance; follow with confirmed/pending=false status |
| Script.Eval | `{"id":SLOT,"code":"JSON.stringify(acknowledgeRestored())"}` | true only after fresh old-IP authenticated identity/output/config/runtime acceptance; follow with acknowledged/pending=false status |

SLOT, CHUNK, OFFSET and TARGET are typed controller substitutions, not literal
strings or shell interpolation. Allowed identity/target pairs are the two
dated rows in README; recheck live identity before choosing one. Runtime
state polling never reads or emits full Wi-Fi config. Privately retain the
preflight snapshot and script bytes for reproducibility.

The two-minute arm bounds each IP trial. First let it expire, acknowledge
through the old path, then rearm for boot-pending proof, acknowledge again,
then rearm for confirmation. After all rollback proofs pass, a separate final
guarded promotion may confirm the target IP while retaining allocations.
After the confirmation trial, return to the original address through a
second guarded transaction: arm at the now-proven new address, change only
the same five fields back to the device's original .28/.29, /24 and
gateway/DNS192.168.1.1, then establish fresh authenticated old-IP identity,
output and runtime/config checks before confirming that return transaction.
Its timeout fallback is the just-proven new address. Do not clear storage or
write the original config without arming this return transaction. Final state
for s14 is the original address; task724 owns permanent device promotion.
Canonical controllers/fan helpers and router/service dependencies must be
promoted under their owning reviewed steps. Do not remove the dedicated slot
or re-enable Matter while any pending marker/recovery dependency remains.

The Core docs specify asynchronous result/error callbacks but no local
Shelly.call completion deadline. No speculative overlapping retry/watchdog
was added: if a callback never arrives, the one in-flight operation remains
pending, blocks confirmation/acknowledgement and never claims restoration.
The persistent marker still drives boot recovery. This limitation is explicit
and covered by a silent-callback fixture; installed callback progress and
actual successful recovery remain live acceptance requirements.

## Offline evidence and limits

Run `node f3s/device-recovery/check-shelly-offline.cjs`. Thirteen scenarios run the
actual prepared script inside a fake Shelly/Timer/storage environment. They
cover timer expiry, simulated reboot, persistent confirmation, queued timer
races, in-flight confirmation refusal, error/retry, runtime-IP verification,
idempotency, malformed/stale identity records, invalid masks/timeouts, DHCP
refusal, guarded restored acknowledgement/rearming, a silent callback and
storage/timer/restart-required failures. Fixtures assert the only
RPC is Wifi.SetConfig with exactly the five snapshot fields, and that synthetic
secret fields never enter storage, calls or status. No script output logging
exists. No device is contacted by these tests.

Fixtures do not prove installed interpreter compatibility, flash durability,
boot execution, runtime route application, uninterrupted relay behavior or
remote connectivity. Those remain guarded live tests, with Matter still on
and no recovery script currently installed on either actual device.
