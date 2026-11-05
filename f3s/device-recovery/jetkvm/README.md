# JetKVM recovery design and unresolved gates

Evidence reviewed 2026-11-05 for task s14. **Preparation only: no recovery
helper is installed, no network setting is changed, and no JetKVM recovery
trial has passed. Renumbering remains gated.**

The mandatory contract is in IQ's
[`RECOVERY.md`](../../../../iq/topics/infra/openbsdrouter-plan/RECOVERY.md)
and [`ADDRESS-RECOVERY.md`](../../../../iq/topics/infra/openbsdrouter-plan/ADDRESS-RECOVERY.md).
Those inventories own allocations; this file does not allocate addresses.
Keep the controller's existing AP connection and every old allocation.

## Evidence and limits

Authenticated reads establish app 0.5.10 and system 0.2.8 on all four devices.
These observations identify devices, not permanent old DHCP allocations:

| Device ID | Observed DHCP IPv4 | DHCP client | Allocated target |
|---|---|---|---|
| e10a0883debf7d4a | 192.168.1.241 | udhcpc | 10.20.0.158 |
| f72978d7f87aa969 | 192.168.1.244 | udhcpc | 10.20.0.191 |
| cee6ab4df2f88754 | 192.168.1.245 | udhcpc | 10.20.0.198 |
| 46793b0d4c8d45d | 192.168.1.248 | jetdhcpc | 10.20.0.151 |

Only the last device has a verified host attachment (f2). All use DHCP IPv4
and native SLAAC. Do not infer other host attachments from suffixes.

The narrowly authorized developer-access inspection on the first device
verified these installed files and their order:

1. `/etc/init.d/S20linkmount` mounts `/userdata` before S21appinit.
2. `/etc/init.d/S21appinit` invokes `/oem/usr/bin/RkLunch.sh`.
3. RkLunch synchronously runs `/userdata/init.d/S??*` during `rcS` before
   `post_chk` starts network initialization, Dropbear and the application.
4. `post_chk` starts `/userdata/jetkvm/bin/jetkvm_app` once in the background;
   no application supervisor is present in that inspected script.
5. `/userdata` is writable ext4. `/userdata/kvm_config.json` exists and was
   statted; its full content was not read. `/userdata/init.d` was absent.

The installed hook is evidence for a possible pre-application restore, not
proof that a new hook has been installed or exercised. Its behavior still
needs checking on each remaining unit. Do not rerun S21appinit or RkLunch as
an application restart: they also initialize the network and other hardware.

Developer mode, TCP22 and the exact previous empty authorized-key state were
restored after inspection. Root's effective shadow account was locked; the
server's advertised `password,publickey` methods alone did not prove usable
password authentication. Any later temporary access must repeat the reviewed
identity, authentication-policy and exact-cleanup procedure.

Local private inspection evidence is held under
`~/.local/state/conf/s14-jetkvm-inspection-v4-20261105/inspection.stdout`
and the follow-up `s14-jetkvm-inspection-v5-20261105/inspection.stdout` in
the same parent directory.
Keep full device configuration, credentials and key material out of this tree.

The follow-up read-only inspection verified that installed
`/oem/usr/bin/RkLunch-stop.sh` calls `killall jetkvm_app` and `killall udhcpc`,
waits using `ps | grep`, and runs all OEM/user stop hooks. That is broader
than a narrow stock-app restart adapter. The installed udhcpc default script
sets the primary address with `ifconfig`, replaces default routes and updates
DNS on bound/renew; deconfig sets the interface address to `0.0.0.0`. It then
invokes executable files in `/usr/share/udhcpc/default.script.d/` for every
event. This is a real lease hook, but it does not establish that arbitrary
aliases survive, or provide ownership for the jetdhcpc variant.

The observed same-executable process ancestry was 180 → 263 → 276, with the
first process parented by PID1. Source below explains supervisor/app/native
roles; do not hardcode these transient PIDs. No watchdog sysfs values or tool
lookup results were emitted by the bounded inspection, so it does not prove
watchdog inactivity or the availability/absence of a particular utility.

## Application ownership matters

The following references are pinned to app release 0.5.10 commit
`f152581df82c519ecc4349f471dfc8612fbf0c48`, reviewed from the local source
checkout. System source is pinned separately to 0.2.8 commit
`dc2b6d967fc16aa59b412ae306eb34f4c1ec3d0b`; installed-file observations above
take precedence over generic source layouts.

- [`internal/network/types/config.go`](https://github.com/jetkvm/kvm/blob/f152581df82c519ecc4349f471dfc8612fbf0c48/internal/network/types/config.go#L10)
  has one IPv4 static configuration, not a secondary-address list.
- [`network.go`](https://github.com/jetkvm/kvm/blob/f152581df82c519ecc4349f471dfc8612fbf0c48/network.go#L229)
  requests reboot for IPv4 mode/static changes. `rpcSetNetworkSettings`
  applies the running configuration, saves it, then immediately requests
  reboot. It is a disruptive operation, including when used for staging.
- [`pkg/nmlite/link/manager.go`](https://github.com/jetkvm/kvm/blob/f152581df82c519ecc4349f471dfc8612fbf0c48/pkg/nmlite/link/manager.go#L477)
  removes non-link-local addresses absent from its expected set.
  [`interface.go`](https://github.com/jetkvm/kvm/blob/f152581df82c519ecc4349f471dfc8612fbf0c48/pkg/nmlite/interface.go#L806)
  applies jetdhcpc IPv4 leases through that reconciliation. Its udhcpc
  callback deliberately skips address application; the installed udhcpc
  shell hook's `ifconfig` operations still need alias-lifetime tests. Link-down handling
  removes all IPv4 addresses regardless of which DHCP client is selected.
- [`config.go`](https://github.com/jetkvm/kvm/blob/f152581df82c519ecc4349f471dfc8612fbf0c48/config.go#L353)
  writes the app's whole in-memory configuration using `os.Create` and
  JSON encoding. Editing only the disk network field while the application
  runs can be overwritten by a later save. Restoring a whole old backup can
  instead discard unrelated current authentication/USB/video settings.
- [`main.go`](https://github.com/jetkvm/kvm/blob/f152581df82c519ecc4349f471dfc8612fbf0c48/main.go#L187)
  returns after SIGTERM/SIGINT, and defers app-context cancellation.
  [`hw.go`](https://github.com/jetkvm/kvm/blob/f152581df82c519ecc4349f471dfc8612fbf0c48/hw.go#L101)
  owns a watchdog whose disarm runs asynchronously on context cancellation.
  [`internal/native/proxy.go`](https://github.com/jetkvm/kvm/blob/f152581df82c519ecc4349f471dfc8612fbf0c48/internal/native/proxy.go#L239)
  launches the same binary as a native child with a parent-death SIGTERM.
  These mechanisms do not yet prove a safe installed stock-app restart.
- [`cmd/main.go`](https://github.com/jetkvm/kvm/blob/f152581df82c519ecc4349f471dfc8612fbf0c48/cmd/main.go#L65)
  runs an internal supervisor which starts the app child, forwards SIGTERM
  and waits for that child. `doSupervise` has no respawn loop. The lack of a
  supervisor in RkLunch therefore does not mean the binary has none; it
  means the inspected shell launcher supplies no restart service.

The app rewrites its process title; the native child uses the same executable.
Matching `/proc/PID/exe` alone does not distinguish them. A future restart
adapter must pin the actual app PID, start time and role, and verify exact
termination before writing config; broad `killall` is unsuitable.

## Two gates precede executable implementation

**A deterministic starting endpoint must exist.** Restoring saved DHCP mode
does not guarantee return to .241/.244/.245/.248 or a bounded lease time.
Those leases are in the Vivacom pool. Proposed old-LAN static .140–.143 are
only candidates: being outside known pools does not prove they are unused or
unreserved. Read the relevant lease/reservation tables and reconcile the
inventory before choosing or claiming any fixed recovery addresses.

An additive alias trial on the current wire also needs verified ownership:
the installed udhcpc hook performs primary-address operations and jetdhcpc
explicitly removes stray aliases. A periodic alias-repair race is not an accepted substitute.
The first DHCP-to-static transition therefore needs its own independently
tested recovery path; it cannot bootstrap itself by assuming DHCP return or
the new static address works. The existing provisioning 192.168.11.0/24 is
not an allocated JetKVM recovery network. The dedicated 10.98.0.0/24 remains
on igc1 only and cannot be used on the shared wire.

**Applying and restoring must respect the real network owner.** An
unconditional pending restore at every kernel boot is incompatible with
using SetNetworkSettings to enter a trial: that operation reboots and would
immediately undo the trial. Do not silently exempt a first reboot from the
boot fallback. A consumed-once planned-boot exception would be a different
recovery design needing explicit review.

The candidate retaining the strict boot contract is an independently timed
helper plus a verified stock-application stop/merge/restart adapter. It would
change persistent `network_config` while the owner is stopped and let the
stock app apply it on restart, without a kernel boot. Before implementing it,
establish the installed process/native/watchdog lifecycle and a supported
restart procedure. The vendor developer-deployment script's stop/start of
a debug binary is insufficient proof for unattended recovery of a stock unit.
An app restart reinitializes USB/native components; test and review its effect
on the attached host, media, display, authentication and video first.

Further root-coordinated capability work should resolve the watchdog policy
and exact restart environment without dumping configuration or auth values.
Do not equate the installed stop script or internal supervisor with a verified
restart contract.

## Alternative to evaluate: independent virtual recovery interface

A separate macvlan interface on eth0 could give the helper an address outside
the app's eth0 address reconciliation. This is a capability proposal, **not
established device support or an allocated recovery path**. It could provide
the deterministic bridge needed before changing the primary address, while
leaving the vendor network JSON intact during its own trial.

First resolve installed kernel/module macvlan support, available netlink tools
or a suitable small helper, and the physical switch's additional-MAC behavior.
The existence of a tool's macvlan syntax alone does not prove kernel support.
A future bounded trial would create a uniquely named virtual interface with
an explicitly assigned, collision-checked local MAC and an allocated fixed
recovery IP. Keep its routes, DNS and default gateway changes separate from
eth0. Overlapping old-LAN connected routes and Linux ARP/reply selection need
explicit tests; the existing provisioning subnet is not automatically a
permitted alternative. A default-route change is unnecessary for a directly
connected test, but routed controller access needs a deliberate return path.

Prove fresh authenticated device ID through both old and recovery paths,
timeout deletion and reboot behavior before relying on it. It must survive
the app's actual DHCP renew/rebind and link-down/up lifecycle in a tested
way, without a polling race. A retained recovery interface would need its own
verified boot setup and cleanup; a temporary target-only interface disappears
on reboot and is not final network configuration. Never let later primary
configuration/DHCP assign that same IP on eth0 while it remains on macvlan.
This option still needs a coherent primary-network restore mechanism and the
same authentication/USB/video preservation checks. No virtual interface or
kernel module was created or loaded during this preparation.

## Candidate helper contract after those gates

This describes required behavior, not a ready-to-run implementation:

1. Verify expected device ID and versions over fresh authenticated old-IP
   access. Reserve both old/recovery and target allocations. Save only the
   private network field and necessary restore metadata; preserve every
   unrelated JSON field. Bound concurrent administrative/OTA changes.
2. Install and test the synchronous pre-app hook. A persistent pending
   transaction at any boot restores the saved network field before app
   launch. Never start a new trial implicitly at boot.
3. Launch an independent detached helper and verify its identity and armed
   state before the disruptive operation. Use a monotonic local deadline;
   a disconnected controller, lost SSH process, wall-clock correction or
   cloud session must not cancel or extend it.
4. Persist the saved network field, transaction identity and pending state
   with atomic replacement and directory/file sync before changing anything.
   Serialize confirm/expiry/restore. Pin process identity to avoid PID reuse.
5. Use the reviewed stop/merge/restart adapter for trial and timeout restore.
   While the app is stopped, merge only `network_config` into the latest
   valid JSON; reject malformed/truncated input rather than printing it.
   Restart the exact verified stock executable and environment. A failed
   restore stays pending for boot fallback; do not report it as confirmed.
6. Confirmation binds to this transaction and expected device ID. The
   controller accepts it only after fresh authenticated literal target-IP
   proof and the required old/recovery-path proof. Clear pending durably
   under the same serialization as timeout restore. Repeated confirm or
   restore must be harmless and cannot affect a later transaction.

The helper must not accept shell snippets, raw replacement config or
unauthenticated confirmation requests. Keep saved state root-private. Do not
modify developer-access settings, USB settings, host-power commands, device
identity or video configuration. Restore temporary developer access exactly
after acceptance. Do not count an old WebRTC/SSH connection as fresh proof.

Before deployment, offline fixtures must cover merge preservation (including
unknown fields), invalid JSON, interrupted persistence, confirm/timeout
races, stale transactions/PIDs and boot restoration. Such fixtures cannot
prove on-device detachment, service ownership, watchdog safety or reachability.

The bounded canary sequence then needs timeout expiry, explicit confirmation,
idempotent restore, and a separate unconfirmed-boot trial, each followed by
fresh authenticated ID/version/network/USB/video checks on the intended
literal path. Test the exact DHCP-client variant on each unit. Only after
those device trials and controller-path requirements pass can s14 accept
JetKVM recovery and permit renumbering.
