# Shelly native recovery acceptance — 2026-11-05

The user explicitly approved disabling Matter and the fully shut-down
MCU/control maintenance window. Root gracefully stopped all VMs and f0–f3,
verified host silence, cut shelly2 AC and verified both outputs OFF. Fresh
pi0/pi1/triggerfish SSH and AP connectivity still worked after that AC loss.
This measured independence supplements the user's supply confirmation.

Trials run serially, shelly1 first. No AP/SSID/controller IP/routing, firmware,
Wi-Fi credential, Switch configuration or schedule was changed. The intended permanent targets
remain 10.30.0.28/.29 on Wi-Fi. Task s14 returns each device to its original
192.168.1.28/.29; permanent promotion belongs to task724.

## Checkpoints

| Check | shelly1 | shelly2 |
|---|---|---|
| Exact ID/model/firmware2.1.0/digest preflight | PASS | PASS |
| Initially Matter=true, fabrics0, scripts0, schedule1, initial_state=off | PASS | PASS |
| Matter disable and required MCU restart | PASS | PASS |
| Dedicated script1 upload/readback/boot-enable/native execution | PASS | PASS |
| Independent120s expiry after originating controller exited | PASS | PASS |
| Fresh original-IP restore, idempotence and guarded acknowledgement | PASS | PASS |
| Actual MCU boot pending with old-IP self gateway/DNS | PASS | PASS |
| Fresh new-IP confirmation survives former120s deadline | PASS | PASS |
| Separate armed return-original transaction and confirmation | PASS | PASS |
| Late original-IP/pending=false check after return deadline+130s | PASS | PASS |

Both Matter.SetConfig responses required restart; the driver checked actual
Sys restart state and performed the approved required MCU reboot once per
transition. No precautionary reboot or firmware update was used. Script bytes
were uploaded in1024-byte chunks with only EXPECTED_ID assigned, reassembled
via GetCode and compared exactly before enabling/start. Native2.1.0 script
status and idle/pending=false/error=null passed before any arm or IP write.

The timed trial arms120s, changes only the five IPv4 fields to the real target,
verifies fresh authenticated new-IP identity/config/runtime/output, and exits
the originating controller process. After130s, a new process verifies restored
original-IP access. Repeating restoreAttempt preserves cfg_rev and state;
acknowledgeRestored closes that trial without deleting code/storage.

The boot trial keeps the old IP, sets only gateway and DNS to that same self
IP, verifies the bad route over a fresh direct old-LAN connection, then sends
the approved MCU reboot there. This avoids any dangling reboot timer. On
shelly1, uptime reset232→8s; its boot script had restored original gateway/DNS
and reported restored/pending=true while unixtime and last_sync_ts were null.
That establishes observed restore before NTP synchronization. On shelly2,
uptime reset217→9s and the clock had already synchronized at the restored
checkpoint; that observation alone does not establish its order relative to
NTP synchronization. Neither trial claims total IPv6/RA isolation. Timing
remains independent of wall time in
the recovery code. The marker is acknowledged only after fresh old-IP checks.

After new-IP confirmation, the controller exits again and waits past the
former120s timer before checking that new access still works. Return to the
original address is another armed transaction: its fallback is the proven new
address, and fresh old-IP identity/config/runtime/output checks precede its
confirmation. Late checks then verify no stale timer moves it back again.

Each device's output remained OFF during its own IPv4 trials. MCU/output tests occur only in
the approved fully-down window. Root owns subsequent authenticated power/fan
control and restores outputs ON before starting hosts. Final routed caller
proof from promoted Pi/f-host addresses remains a later migration step.

Late rechecks passed at12:49:47UTC (shelly2) and12:49:48UTC (shelly1), both
original addresses, phase=confirmed, pending=false, error=null. At those
checkpoints root's subsequent control window had shelly1 ON and shelly2 OFF;
the driver only observed outputs. Root records the later final power/control
and host-startup results separately.

| Device | Verified uploaded bytes | SHA256 of device-specific code |
|---|---|---|
| shelly1 | 7797 | db635814ef79c81e2268e0abf626e4911adebc8cc8fcba88d7953b2bcd6bf154 |
| shelly2 | 7797 | 053200e6ab8e4e6f5a9f8f5ec791db964c2600325b68cac52a578fec80abe194 |

## Evidence and limits

Controller and full request-response results/config snapshots stay private:
`~/.local/state/conf/s14-shelly-live-20261105/` (0700; files0600), with device
subdirectories s1/s2 and a saved controller.py copy. Digest credentials are
read internally from ~/.shelly_plug, never request arguments or logged headers.
The private uploaded.js files retain the exact verified device-specific code.

Final checks compare the entire Wi-Fi configuration to its original values,
all Switch fields and the unchanged existing schedule. Other full-config
components also match, excluding only expected sys.cfg_rev revision metadata
after an exact private diff identified that counter. The dedicated slot is
the sole script, boot enabled and running; Matter remains disabled while this
recovery mechanism is needed. No credentials or full configs enter git.

Offline fixtures passed thirteen recovery scenarios, probe redaction checks,
and actual controller return-order/deadline and boot sequencing checks. Native
checks above are separate evidence; no future physical topology or routed
Pi/f-host source-policy acceptance is inferred from this shared-wire trial.

## Root control and restoration result

After both late checks, Pi0 switched rack fans on. Pi1's unprivileged local
fans-off command correctly refused because it could not read the protected
power-job API key; no output changed. The same command through the existing
noninteractive doas privilege path completed, followed by Pi1 fans-on and
AC-on commands with verified read-back. No force flag or permission relaxation
was used. Fans and f-host AC were ON before the remote power-all-on job.

The normal wired Pi power-all-on job completed with all f0–f3 and r0–r2
accepting fresh SSH and monitoring alerting restored on both frontends.
Post-start checks passed three Ready nodes, all three etcd members reporting a leader, matching
bounded NFS reads, cross-node UDP/TCP DNS, API Service and verified-TLS
Player ingress. Both accepted f3/r2 dual-address profiles survived the full
power cycle with old primaries/defaults retained. f3's rocky guest auto-started;
its FreeBSD guest returned to its normal stopped policy. The two owned
post-maintenance probe pods were deleted with UID preconditions and absence
verified. Private post-start evidence is under
`~/.local/state/conf/s14-post-maintenance/evidence/`.

This completes the Shelly portion of s14. JetKVM recovery remains a separate
unproved portion; the task stays open. Both plugs retain original addresses,
Matter disabled, script1 enabled/running and no pending restore.
