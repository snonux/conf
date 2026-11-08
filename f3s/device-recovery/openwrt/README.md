# OpenWrt AP network recovery (task d24)

Installed on the AP on 2026-11-08 with **no transaction armed**. Native
checks accepted exact installed source hashes, a root watchdog owned by procd,
stable polling, the enabled S18 hook and unchanged network/DHCP/firewall/wireless
hashes. Native live timed restoration is now accepted: the new alias and exact f0
return route worked, a fresh new-IP authenticated SSH succeeded, originating
connections closed, then the independent watchdog restored all files and running
LAN/DHCP state and a fresh old-IP login passed. Boot restoration and explicit
confirmation/persistence remain unaccepted. The alias candidate is prepared but
currently restored/inactive. Private receipt:
`~/.local/state/conf/d24-ap-preparation-20261108/watchdog-installed-native-verified.json`.
The user explicitly resumed Triggerfish migration after the household TV pause;
physical cable gates and independent-controller requirements still apply.

The intended final DHCP server is **Triggerfish**, on port2/igc2,
10.30.0.1/24, with pool 10.30.0.100–239. OpenWrt becomes the bridge/AP at
10.30.0.2, with its DHCP/RA/DHCPv6 services disabled appropriately. The old
192.168.1.101 management address stays until its separate retirement gate.
The general Wi-Fi pool must never start on the current shared home/lab wire.
OpenWrt's existing DHCP configuration remains active during this preparation.

Read IQ `topics/infra/openbsdrouter-plan/NEXT-SESSION.md`, `RECOVERY.md` and
`ADDRESS-RECOVERY.md`, and task d24's full annotations before live work.
Triggerfish changes remain conf/gonf-owned. This directory supplies only the
currently unmanaged AP's recovery implementation; it is not a cutover command.

## Ownership and recovery protocol

`ap-network-recovery.sh` is intended for
`/usr/sbin/f3s-ap-network-recovery` (root-owned 0755).
`f3s_ap_recovery.init` is intended for `/etc/init.d/f3s_ap_recovery`
(root-owned 0755). Its procd instance owns the watchdog independently of the
operator's SSH session. Enabling it creates the S18 boot hook, preceding the
observed AP's S19dnsmasq and S20network hooks. Native supervision and the enabled hook are verified; execution through an
actual AP reboot still needs acceptance.

The helper requires `/etc/f3s-ap-recovery`, root-owned mode 0700.
The native AP has no `stat` command or BusyBox stat applet; the helper checks
the first numeric `ls -ldn` metadata row for owner0 and mode0700, ignoring
filenames and later lines. Native preflight confirmed that output format. It owns its
lock, transaction snapshots and history there. Backups contain private UCI
configuration; never print or commit them. Only `network`, `dhcp` and `firewall`
are replaced. Wireless and SSH configuration are outside its write set.
Do not modify UCI through LuCI or another operator during a pending trial.

The executable offers these operations:

| Operation | Effect |
| --- | --- |
| `status` | Reports token, phase, deadline and pending status, or no transaction. |
| `arm TOKEN SECONDS CANDIDATE` | Requires active watchdog/boot hook and no UCI edits; validates and freezes private before/candidate files, socket baseline and durable pending state. No live configuration replacement. |
| `activate TOKEN` | Applies the frozen candidate and reloads firewall/network, restarts dnsmasq/odhcpd. Partial failures remain pending. |
| `confirm TOKEN services-verified` | Requires a fresh established IPv4 SSH socket targeting 10.30.0.2:22, absent from the pre-arm snapshot, matching live candidate bytes and an unexpired trial; clears pending state. |
| `restore TOKEN` | Restores saved files and reloads services; clears pending only after success. |
| `tick` | Restores an expired pending trial, including an invalid clock or changed boot identity. Called by watchdog. |
| `boot` | Restores an unconfirmed previous-boot trial's files before normal networking starts. |
| `watch` | Runs a tick every two seconds; procd respawns a failed watchdog. |

Tokens are unique 8–64-character strings containing letters, digits, underscores
or hyphens. Timeouts are integer seconds from 60 to 1800, without leading zeroes.
The candidate directory must be root-owned mode 0700 and contain regular,
non-symlink `network`, `dhcp` and `firewall` files. Native UCI validates their
syntax. LAN must remain static on br-lan and retain 192.168.1.101/24. This is
only a minimum safety check, not validation of the entire intended policy.

Prepare candidate UCI changes with isolated `uci -c DIR -t SAVEDIR` directories;
never use global `uci set` to prepare an inactive candidate. Keep wireless
credentials and radio settings unchanged. The planned staged candidate also
needs a measured return path for new-address management and appropriate scoped
DNS/rebind handling; `prepare-stage.sh` now prepares only the inactive first-stage candidate. It
adds `ap_migration` (10.30.0.2/24 on `@lan`) and `ap_migration_admin`
(f0 10.20.0.130/32 via30.1), preserving main LAN and byte-identical DHCP/firewall.
Run it on the AP with a new private path under
`/etc/f3s-ap-recovery/candidates/NAME`; it refuses pending transactions, existing
owned sections and reused directories. Native UCI validation passed for
`stage-20261108`; its live expiry trial passed and it is now restored/inactive. A final cutover candidate is separate.

`services-verified` is an operator assertion, not an automated service test.
Before confirmation, establish fresh old and new management sessions and
actually verify the required DNS, client, controller and device behavior.
Never substitute an old multiplexed session or a fabricated SSH environment.
Snapshots persist after confirmation/restoration for evidence. A confirmed
transaction is not automatically reverted at the next boot.

## Offline verification

Run locally, without contacting the AP:

```sh
sh -n f3s/device-recovery/openwrt/ap-network-recovery.sh
sh -n f3s/device-recovery/openwrt/f3s_ap_recovery.init
shellcheck -s sh f3s/device-recovery/openwrt/ap-network-recovery.sh \
  f3s/device-recovery/openwrt/f3s_ap_recovery.init
python3 f3s/device-recovery/openwrt/check-offline.py
```

The 13 fixtures exercise the production shell with real private files, hashing,
replacement and locking. They cover expiry, early boot restoration, fresh-socket
confirmation, deadline rejection, token reuse, frozen candidates, reload failure,
corrupt snapshots, old-address retention, invalid-clock fallback, private
directory ownership/modes and filenames containing forged metadata lines. UCI,
procd, sockets and boot/uptime are simulated; passing fixtures do not prove
native UCI grammar, process supervision, network reachability or a real reboot.

## Native acceptance still required

For a later owner reapply, preserve the private full AP backup and first verify
no pending UCI edits or conflicting owned paths. Install the helper/init and
empty private state, enable/start the watchdog without arming a trial, then
verify its real procd ownership, enabled boot hook, no-transaction status and
unchanged network/DHCP/firewall/wireless hashes. Initial installation passed
these checks. Do not erase an existing transaction to reapply. Record exact
receipts and any
live worker handle; do not relaunch an unknown in-flight operation.

Before applying any network candidate, re-establish an independent controller
attachment (Earth returned to Mars11 after the user resumed migration; check
its current attachment and establish a fresh independent router session), check the accepted router recovery tunnel and stage only the
required scoped management flow through gonf. Timed expiry on the existing wire passed (private `ap-expiry-accepted.json`).
Prove persistent boot restoration, including actual running services and fresh
SSH, before confirming a staged configuration. A real AP reboot interrupts
Wi-Fi and remains a separate explicit recorded trial. Owner reapply and confirmed-state
persistence also require native proof. Task d24 remains incomplete and has not
had its final independent review.

Physical gates b24/e24 remain explicit human actions. Check the still-unanswered
AP PoE source before issuing cable instructions. The intended topology is
ISP→Triggerfish port0, lab switch→port1, OpenWrt→port2 and future cameras→port3.
A carrier or ping does not satisfy either physical gate.
