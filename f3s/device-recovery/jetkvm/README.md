# JetKVM recovery design and unresolved gates

Evidence reviewed 2026-11-05 for task s14. **Preparation only: no recovery
helper is installed, no network setting is changed, and no JetKVM recovery
trial has passed. Renumbering remains gated.**

The mandatory contract is in IQ's
[`RECOVERY.md`](../../../../iq/topics/infra/openbsdrouter-plan/RECOVERY.md)
and [`ADDRESS-RECOVERY.md`](../../../../iq/topics/infra/openbsdrouter-plan/ADDRESS-RECOVERY.md).
Those inventories own allocations; this file does not allocate addresses.
Keep the controller's existing AP connection and every old allocation.

The [first native canary plan](NATIVE-CANARY.md) records source ownership,
actual media-port requirements, temporary access and serial native proof gates.
It remains a review candidate; it allocates and activates nothing.

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

One root-reviewed **read-only** v6 preflight then found executable
`/bin/busybox`, but stopped because its applet list lacked `cmp`, which the
proposed snapshot-comparison wrapper had required. No child was created,
and no module was loaded. The early exit preceded the module-file and
already-loaded-module checks, so this does not establish macvlan's presence
or absence. A later wrapper can compare snapshots on the controller instead
of requiring device-side `cmp`; it must still be reviewed before execution.
The locked-root check passed, the original false developer mode and exact
empty keys were restored with TCP22 closed, and the enable request was
terminal. Fresh identity/version checks passed; USB state was unchanged and
video was ready. Evidence is under the matching private
`s14-jetkvm-inspection-v6-20261105` state directory. No further device trial
was included in v6.

A subsequent single batched read-only v7 inspection collected independent
facts without stopping at missing tools. `/bin/busybox` lists `sh`, `ip`,
`setsid`, `nohup`, `timeout`, `sleep`, `cat`, `awk`, `mktemp`, `rm`, `uname`,
`ls` and `readlink`; it does not list `zcat`. The running kernel reports
5.10.160. `/oem/usr/ko/macvlan.ko` exists (19,596 bytes) and its installed
`modules.dep` entry has no listed dependency. `/sys/module/macvlan` was not
observed. `/oem/usr/ko/insmod_ko.sh` exists; the corresponding `/ko` files and
`/proc/config.gz` were not observed. Shipped module presence does not prove
successful loading or link creation.

The parent eth0 had ifindex 2, its expected MAC, flags `0x1003`, MTU 1500,
operstate up, .241/24, default gateway .1 and the ordinary local/main/default
IPv4 rules. Its existing global/ULA/link-local IPv6 addresses remained present.
The probe-child name was absent. Private route/address details are in
`s14-jetkvm-inspection-v7-20261105`; no network, module, service or child change
was made. The command exited successfully, the enable request was terminal,
and developer mode/closed TCP22/exact prior empty keys were restored. Fresh
ID/version checks passed, USB state matched and video remained ready.

The single root-reviewed v8 native capability probe loaded that exact shipped
module without forcing, created `s14mvprobe` down with no IPv4/IPv6 address,
verified ifindex 6 / parent iflink 2 / expected probe MAC, deleted the owned
child and unloaded its owned module. Both native cleanup checks passed.
**Parent-preservation acceptance failed:** the immediate after-snapshot showed
the parent's global and ULA IPv6 addresses newly tentative, with their local
host routes absent. Those are semantic changes, not ignored lifetime counters.
The parent MAC/flags/MTU and IPv4 path matched; no address, route or rule
command was submitted by the probe. The cause and settling behavior require
investigation; do not claim a harmless additive recovery interface from this
result. No repair or repeat mutation was attempted.

The wrapper stopped on that comparison failure before its USB/video
postcheck, then completed terminal enable-request draining, false developer
mode, closed TCP22 and exact prior empty-key restoration without cleanup
errors. Fresh post-probe ID/USB/video checks were not part of that failed
v8 result. Exact shell, driver and seven offline cleanup-fixture cases are saved
privately alongside `s14-jetkvm-inspection-v8-20261105` evidence. Those fixtures
covered partial-success command failures, but did not predict this native
IPv6 effect; they are not on-device network-preservation proof.

A subsequent root-authorized v9 **read-only** follow-up took two parent
snapshots three seconds apart. Both match the pre-v8 snapshot after ignoring
only numeric lifetimes: the two global/ULA IPv6 addresses are no longer
tentative, their local routes have returned, and the other parent network
semantics match. The probe child and module were absent in both reads. Fresh
authenticated ID/version passed, USB matched the pre-v8 baseline and video
was ready. Terminal enable draining and exact developer-access cleanup passed
again. The transient has settled; its cause remains unexplained and v8 is
still not accepted as a parent-preserving recovery design. Private v9 evidence
includes the exact read-only driver and `settled-comparison.json`.

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

## Why the revised design uses a separate management interface

A saved DHCP profile does not promise the same old lease or a bounded return
address. Nor can an eth0 alias be treated as independently owned: jetdhcpc
reconciliation and the installed udhcpc operations need separate handling.
The revised design below first establishes a fixed management address on a
separate macvlan interface while leaving the original DHCP profile untouched.
It then uses that endpoint throughout any primary-network trial and restore.

An unconditional restore on the first pending kernel boot would immediately
undo the vendor's SetNetworkSettings operation, which itself reboots. The
revised design explicitly permits one target-bearing boot, consumes that
permission before app startup, and starts a new independent local timer.
Every subsequent pending boot restores the old network field before startup.
This replaces the earlier unproved app stop/merge/restart candidate; it avoids
editing disk JSON while a running app can overwrite it and avoids reliance
on a stock-app restart/watchdog contract.

## Independent virtual recovery interface

A separate macvlan interface on eth0 could give the helper an address outside
the app's eth0 address reconciliation. This is a capability proposal, **not
established device support or an allocated recovery path**. It could provide
the deterministic bridge needed before changing the primary address, while
leaving the vendor network JSON intact during its own trial.

v8 established module loading and down-link creation/deletion; it did not
prove addressed traffic or the physical switch's additional-MAC behavior.
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
same authentication/USB/video preservation checks. v8's temporary native
resources were removed; no persistent recovery interface is installed.

### Pinned system capability evidence

The 0.2.8 system source supports further investigation, with these limits:

- [`rv1106-jetkvm-v2_defconfig`](https://github.com/jetkvm/rv1106-system/blob/dc2b6d967fc16aa59b412ae306eb34f4c1ec3d0b/sysdrv/source/kernel/arch/arm/configs/rv1106-jetkvm-v2_defconfig#L327)
  selects `CONFIG_MACVLAN=m` and module loading. This is a module, not a
  built-in driver. It also selects the DesignWare watchdog.
- [`sysdrv/Makefile`](https://github.com/jetkvm/rv1106-system/blob/dc2b6d967fc16aa59b412ae306eb34f4c1ec3d0b/sysdrv/Makefile#L541)
  builds all selected modules, flattens their `.ko` files and generates a
  flattened `modules.dep`. The
  [packager](https://github.com/jetkvm/rv1106-system/blob/dc2b6d967fc16aa59b412ae306eb34f4c1ec3d0b/project/build.sh#L1276)
  copies that directory into the packaged OEM `usr/ko`. Installed RkLunch
  prefers `/oem/usr/ko` and supplies the conventional versioned modules
  symlink. v7 verified `/oem/usr/ko/macvlan.ko` and its dependency entry;
  v8 subsequently proved successful native loading and down-interface creation
  and cleanup. Addressed operation and persistent recovery remain unverified.
- [`config_normal`](https://github.com/jetkvm/rv1106-system/blob/dc2b6d967fc16aa59b412ae306eb34f4c1ec3d0b/sysdrv/tools/board/busybox/config_normal#L927)
  enables BusyBox `ip`/`iplink`/`iprule`, `setsid`, `nohup`, `timeout`,
  `flock` and `modprobe`. The build uses BusyBox 1.36.1. Its shipped tarball's
  `networking/libiproute/iplink.c:686` sends a generic `IFLA_INFO_KIND`, so
  `type macvlan` is supported syntax. Type-specific options are parsed only
  for VLAN and VRF: **do not pass `mode bridge` to this BusyBox parser**.
  [`macvlan.c:1462`](https://github.com/jetkvm/rv1106-system/blob/dc2b6d967fc16aa59b412ae306eb34f4c1ec3d0b/sysdrv/source/kernel/drivers/net/macvlan.c#L1462)
  defaults an unspecified mode to VEPA. This suffices for a down-interface
  capability probe, not a tested final network topology.
- [`rv1106-jetkvm-v2.dtsi`](https://github.com/jetkvm/rv1106-system/blob/dc2b6d967fc16aa59b412ae306eb34f4c1ec3d0b/sysdrv/source/kernel/arch/arm/boot/dts/rv1106-jetkvm-v2.dtsi#L387)
  enables the watchdog. `WATCHDOG_NOWAYOUT` has no default-y in Kconfig and
  is not selected in this defconfig; the driver's initial default is thus
  false before runtime overrides. The
  [driver](https://github.com/jetkvm/rv1106-system/blob/dc2b6d967fc16aa59b412ae306eb34f4c1ec3d0b/sysdrv/source/kernel/drivers/watchdog/dw_wdt.c#L56)
  defaults to 30 seconds if not already running, otherwise it can inherit
  the bootloader's timeout. These source defaults do not establish the
  running device's effective watchdog policy.

The app's
[`runCallbacks`](https://github.com/jetkvm/kvm/blob/f152581df82c519ecc4349f471dfc8612fbf0c48/pkg/nmlite/link/manager.go#L90)
dispatches by the changed interface's name. Only eth0 is registered by the
app. A new child's event therefore has no app interface callback. A parent
event is ignored if operstate, index, MTU and MAC match the previous state
(`link/netlink.go:144`). This is evidence of scoped ownership, not proof
that the hardware driver has no side effect when adding a child.

### Completed down-interface capability canary

The root-reviewed v8 probe temporarily loaded the protected shipped module,
created a uniquely named child down with no addresses, verified its exact
ifindex/parent/MAC, then deleted it and unloaded only the module it owned.
The fixed commands, after module loading and ownership preflight, were:

```sh
/bin/busybox ip link add link eth0 name s14mvprobe address 02:73:14:00:00:01 type macvlan
/bin/busybox ip link delete dev s14mvprobe
```

The helper captured parent link/address/route/rule state and compared it on
the controller, without device-side `cmp`. Seven offline cases covered
normal cleanup, pre-existing modules, failed creation, busy module refusal,
unexpected addresses, and loader/create failures after partial success.
These fixtures did not predict the actual transient parent IPv6 DAD effect.
Native creation and cleanup passed; immediate parent-preservation acceptance
failed; v9 proved the parent subsequently settled back to its baseline.
An addressed/boot trial therefore still needs explicit observation of that
transient, rather than assuming macvlan is invisible to the existing stack.

### What an addressed recovery trial would still need

Macvlan could provide a fixed additional management endpoint while retaining
the vendor's original DHCP configuration, if the separate link, address and
return path are proven. It does not make DHCP itself deterministic. For the
eventual primary-network change, a boot hook could restore the saved vendor
network field while retaining this independently owned recovery endpoint;
runtime restore still needs a coherent owner/apply mechanism.

Before an addressed trial, allocate/reserve its IP and MAC, choose the
current-cabling prefix and prove source-specific return routing. With two
interfaces in the same prefix, Linux ARP policy, route selection and strict
`rp_filter` can defeat a seemingly reachable address. Capture actual
`all`/eth0/child ARP and reverse-path settings before deciding on any change;
do not globally disable reverse-path filtering or overwrite the DHCP default
route. VEPA and parent/child host-isolation behavior also need testing; a
loopback or parent-origin connection does not substitute for an external
controller connection through the switch.

The app's `getBindAddress` binds HTTP to wildcard addresses when IPv4 is
enabled and loopback-only is false; Dropbear's inspected invocation has no
interface-specific bind. Neither fact proves actual authenticated access on
a new link. Verify listeners and preserve the exact SSH authentication
policy. WebRTC/ICE may enumerate additional local candidates, so fresh HTTP
identity proof alone does not prove console/video/virtual-media behavior.
Test the unchanged native DHCP client's renew/rebind and physical-link
lifecycle as well as boot/timeout restoration before accepting this path.

## Revised design: fixed management plus one target-bearing boot

**Offline proposal for root review; no address is allocated or deployed here.**
The first goal is an additional fixed management endpoint independent of the
vendor's primary IPv4 choice. The second is a bounded transaction around the
vendor's rebooting primary-network RPC. They are separate acceptance phases.

### Phase 1: allocate and prepare the shared-wire management path

Proposed planning rows for IQ, subject to root's inventory reconciliation:

| Device ID | Proposed shared recovery | Proposed local management MAC |
|---|---|---|
| e10a0883debf7d4a | 192.168.11.140/24 | 02:52:53:08:97:97 |
| f72978d7f87aa969 | 192.168.11.141/24 | 02:52:53:06:8b:21 |
| cee6ab4df2f88754 | 192.168.11.142/24 | 02:52:53:04:a4:4b |
| 46793b0d4c8d45d | 192.168.11.143/24 | 02:52:53:03:57:38 |

These use the existing provisioning prefix, whose currently documented router
endpoint is 192.168.11.2. Root must verify/record ownership and conflicts,
retain that router endpoint through gonf, and prepare narrow PF and laptop
wg2 routes before an addressed device trial. Permit only the authenticated
10.99.0.2/32 controller and necessary HTTP/temporary SSH/WebRTC traffic.
No DHCP server runs on the child; factory eth0 MAC/DHCP remains unchanged.
This prefix on a shared wire is not layer-2 isolation: router PF controls
routed access, while unchanged device authentication still protects direct
same-wire access.

Use a distinct owned interface, for example `jkrecover`. Give it a private
source-policy table/rule after confirming the numbers are unused. For the
first proposed device, the intended route semantics are:

```text
table 1140: 192.168.11.0/24 dev jkrecover src 192.168.11.140
table 1140: 10.99.0.2/32 via 192.168.11.2 dev jkrecover src 192.168.11.140
priority 1140: from 192.168.11.140/32 to 10.99.0.2/32 lookup 1140
```

This changes no default route. The child has its own connected prefix, so it
does not overlap eth0's old 192.168.1/24 or future 10.20.0/24 primary prefix.
Inspect actual ARP and reverse-path settings and prove replies over this rule;
do not disable filtering globally. If needed and reviewed, alter only the
owned child's setting. Disable unintended IPv6 autoconfiguration on the child
before bringing it up; preserve parent IPv6 settings and observe the v8 DAD
transient explicitly. Kernel/table/rule ownership must be exact and idempotent.

This is a **shared-wire** recovery profile. It does not put 10.98 on igc0 and
does not replace the later igc1 recovery/cable gate. Before a cable move, root
must allocate and test a separate current-cabling profile on igc1, then
transition the management helper/address/return path explicitly. Never count
192.168.11.2 on shared igc0 as reachable after the rack moves to igc1.

### Phase 2: bootstrap and prove the additive management endpoint

Use the existing bounded temporary developer-access procedure. Initially the
helper may manage only its own module/link/address/policy state; it must not
write vendor JSON, change DHCP, renew leases or reboot during that first
additive trial. Arm independent timeout cleanup before bringing up the child.
Keep a fresh authenticated old-DHCP connection and prove new literal recovery
HTTP identity plus authenticated temporary SSH through the routed wg2 path.
Let the timeout remove exactly the trial-owned resources and prove old access
again; separately test confirmation/idempotent reapply. No polling repair race.

After the runtime trial passes, install the verified synchronous userdata boot
hook and retain the fixed management endpoint. Prove boot recreation and the
actual stock `network_init` eth0 down/up sequence, then prove native DHCP
renew/rebind and app owner reapply do not prune or strand the child. Native
tests must cover both udhcpc and jetdhcpc units. Check fresh old/recovery
HTTP/SSH, IPv6 settling, USB/video/media and unchanged authentication after
each trial. This phase requires no replacement of the only primary IPv4.

### Phase 3: implement a durable primary-network transaction

Prefer one small statically linked Go helper for Linux ARM, in its own module
outside `conf/gonf`. Standard JSON/raw-message handling, atomic rename/fsync,
a separate flock file, monotonic timers and fixed-argument BusyBox operations
are sufficient. Reuse actual installed applets; no jq/Python/device `cmp`
dependency, shell JSON edits, permanent HTTP control server or app patch.
Verify ABI and clock behavior on the device before relying on the binary.

The helper owns its management link/policy and transaction state; the vendor
app continues to own eth0 and normal networking. Root-private durable state
contains generation/device identity, saved and expected network fields,
arming boot identity, one boot allowance and phase. Never emit these network
objects or authentication material. A stable separate lock file serializes
arm, boot, expiry, restore and confirm; atomically replacing state must not
replace the lock inode. Reject stale generations and concurrent transactions.

Because the app saves its whole JSON with truncation rather than replacement,
an interrupted save can leave no valid latest JSON to merge. Save a private
last-valid whole-config safety copy as well as the network field. Normally
merge **only** the saved network field into the latest valid JSON, preserving
every unrelated field. Use the whole safety copy only if the latest document
is invalid, report that fallback and verify the saved authentication/USB/video
state. Quiesce other configuration/OTA activity while pending; do not silently
overwrite concurrent edits. This is private local preservation, not a config
dump or a committed secret. Without a valid-document fallback, the claimed
unexpected-boot recovery is incomplete.

Launch the worker with a new session and closed SSH/terminal descriptors; its
generation/boot/PID identity and ready handshake must be checked before the
caller proceeds. Publishing a PID or starting `sleep` is not proof of an
armed worker. Reapply can inspect the existing worker but cannot renew its
deadline. Keep a verified Linux boot identity in state so a same-boot helper
reapply cannot consume the one-boot allowance. Persist each boot's absolute
monotonic deadline with that boot identity; a worker replacement must use the
remaining deadline or restore immediately, never grant a new window. A
Linux `CLOCK_BOOTTIME` reader is preferable if suspend is possible; native
clock/worker tests must establish the claimed bound without wall-clock input.

The state transitions are explicit:

1. **Armed on current boot:** persist the saved/expected fields and one
   allowance, sync all files/directories, start a detached monotonic rollback
   worker and verify it is running before permitting the controller's vendor
   RPC. Use a short bounded issuance window; abandoned arms expire normally.
2. **First target-bearing boot:** before app startup, require the armed
   generation and expected network field. Consume and sync the allowance
   **before** continuing. Start a new independent 120-second monotonic worker,
   verify it armed and establish the fixed management path before returning
   to normal startup. If the expected field or helper readiness does not
   match, restore instead. Do not call SetNetworkSettings from the boot hook.
3. **Any subsequent pending boot, or restoring phase:** merge the saved
   network field before the app runs. Record the restored generation and
   keep the fixed management endpoint. The app then applies its saved
   profile normally; its DHCP lease may change, while recovery stays fixed.
4. **Runtime expiry:** under the lock, durably mark restoring and revoke
   the allowance, then request a JetKVM Linux reboot. The boot hook performs
   the merge before the app. Do not race a live app's in-memory config or
   stop/restart its supervisor/native/watchdog tree. A failed reboot is a
   restore failure, never a successful cancellation; keep restoring state
   and report/retry within the reviewed local recovery policy.
5. **Confirm:** only after the controller establishes fresh authenticated
   literal target-IP and fixed recovery-IP sessions with the correct device
   ID and required services. Clear pending state durably for that generation
   under the lock. An expiry that acquired the lock first wins; stale confirm
   cannot cancel restoring or a later generation. Repeated confirm/restore
   is harmless. Retain recovery until its separate retirement gate.

Installed RkLunch does not stop startup on a nonzero user-hook exit. The hook
therefore cannot report a failed worker start and simply return: it must
synchronously restore the saved field before allowing normal app startup.
Only a ready guarded target boot or a completed saved-profile restoration
may return from the pending-state hook. Native failure injection must exercise
this branch. Permanent storage/hardware failure cannot be disguised as a
successful restore or a bounded software-only guarantee.

The first boot after target save cannot be distinguished reliably from an
unexpected reboot at that same point. The permission therefore means **one
boot carrying the pinned target field**, not a promise that every unexpected
first reboot immediately restores. A second pending boot restores. No timer
may be repeatedly reset by reapply/reconnect; no wall/NTP clock is involved.
The already proven management endpoint remains the deterministic access path
while the vendor's DHCP/static primary and default route change.

The time bound has two parts: each live pending boot has at most 120 seconds
before it enters restoring, plus the natively measured reboot/startup bound
before the old profile runs. At most one target-bearing boot is permitted;
another reboot cannot start another 120-second target trial. Measure and record
both bounds, including worker startup and restoration, before claiming bounded
recovery. A failed kernel boot or failed hardware/network is outside what an
in-memory timer can guarantee; fixed management and tested boot persistence
reduce software-transition risks without inventing such a guarantee.
With the same 120-second limit on the arming boot, the conservative overall
bound is 240 seconds plus two measured reboot/startup bounds: at most one
pre-boot window and one continued target-boot window. If the arming window
expires before a target-bearing boot, it is only 120 seconds plus one reboot
bound. Do not call the whole cross-boot operation a single 120-second deadline.

### Phase 4: prove the transaction before production renumbering

Offline fixtures must cover durable-write interruption, invalid latest JSON,
preservation of unknown/unrelated fields, stale IDs/generations, confirm/expiry
races, repeated commands, lost worker startup, and boots before/after allowance
consumption. Include the first unexpected target-bearing boot in the allowed
case and every subsequent pending boot in the restoring case. Cross-compile
and inspect the binary; offline tests cannot establish device boot ordering.

On the additive management canary, first exercise pending expiry and boot
restore with the **same** primary network field: this proves persistence and
the intentional JetKVM reboot without replacing primary IPv4. Then, after
root's target/PF/routes readiness, perform a bounded target trial using the
vendor RPC. Deliberately withhold confirmation to prove expiry plus fresh
fixed recovery and restored-profile access. Separately prove an additional
unconfirmed reboot restores before app startup. Finally prove fresh target
and recovery confirmation, idempotency, accepted reboot/reapply and exact
temporary developer-access cleanup. Preserve all old allocations throughout.

No host power/ATX action belongs in these tests. A planned JetKVM reboot drops
its sessions and reinitializes native/USB components; acceptance requires
unchanged settings and fresh working USB/video/media/authentication afterward.
Do not treat an existing WebRTC connection as fresh recovery evidence.

### Fit with the mandatory IQ contract

IQ RECOVERY requires rollback to restore "running state and persistent
configuration" and requires a "tested boot-time fallback" for rebooting
changes. ADDRESS-RECOVERY's JetKVM clause requires a "device-local detached
timeout and persistent boot restore of saved network configuration". It does
not contain Shelly's separate requirement to restore at every first pending
restart. The explicit one-target-boot state machine can satisfy the JetKVM
wording when the independent fixed path, timer and subsequent-boot restore
are natively proven; it must not be presented as the earlier unconditional
first-boot policy. Root reviews this revised contract and its measured bounds.

The management-device exception permits tested automatic revert where primary
dual-addressing is unavailable. Here the old primary allocation remains held,
the fixed management endpoint remains configured, and old-profile rollback is
proved before replacing primary IPv4. The remaining work is the concrete
allocation/router preparation, helper review and staged native evidence above;
no additional physical action or implicit user-permission gate is introduced.

## Offline helper implementation

The standalone [Go helper](helper/README.md) now implements the private journal,
monotonic guarded workers, one-target-boot transaction and exact owned additive
network adapter. Its bounded bootstrap/management-boot path does not require a
completed management acceptance receipt; primary-network arms do. The module's
README contains schemas, controller protocol, fail-closed boot wrappers and the
remaining staged native plan. Local tests and a static ARMv7 cross-build are
software evidence only. No helper installation, address allocation or device
network change has been performed by this implementation task.
