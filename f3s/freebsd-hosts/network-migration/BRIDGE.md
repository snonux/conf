# Host addresses onto the vm-public bridge (task 764)

On f0–f3 the lab IPv4 `10.20.0.13N/24`, the lab ULA
`fd02:ca0b:f1cb:20::13N/64` and, on f0/f1, the storage CARP VIP
`10.20.0.138/32` (vhid 1) sat on `re0`, a member of the vm-bhyve bridge
`vm-public`. A guest's neighbour solicitation for its own host then arrives
on the guest's tap (IPv6 disabled) and is dropped: ND answers only for
addresses of the receiving interface. On the bridge interface the host
answers for every member. FreeBSD 15 also deprecates addresses on bridge
members (`net.link.bridge.member_ifaddrs`). CARP supports bridges
(`carp_is_supported_if()` accepts `IFT_BRIDGE`; `if_bridge` checks the
bridge's own CARP MAC in `GRAB_OUR_PACKETS`).

## Target configuration

rc.conf (network.subr maps the `-` of `vm-public` to `_`):

```sh
cloned_interfaces="bridge0"
ifconfig_bridge0_name="vm-public"
ifconfig_vm_public="inet 10.20.0.13N/24 addm re0 up"
ifconfig_vm_public_ipv6="inet6 fd02:ca0b:f1cb:20::13N prefixlen 64 auto_linklocal -accept_rtadv"
ifconfig_vm_public_alias0="inet vhid 1 [advskew 100] pass … alias 10.20.0.138/32"   # f0/f1
ifconfig_re0="up"          # no ifconfig_re0_ipv6: netif sets re0 ifdisabled
```

vm-bhyve `/zroot/bhyve/.config/system.conf`: `type_public="manual"`,
`bridge_public="vm-public"`, no `ports_public`. A standard switch would try
to create its own `vm-public` at `vm init`, fail on the existing name and
start no guest; a manual switch adopts the rc.conf bridge. A bridge has no
`auto_linklocal` by default, hence the flag. Wake-on-LAN still uses re0's MAC.

Other owners follow the move: `pf.conf` filters on `{ re0, vm-public }` and
their addresses (one ruleset for both sides of the move), the devd CARP rule
admits `1@vm-public` (devd anchors its patterns), `CarpNode.Interface` in
`gonf/cluster` selects the rc.conf CARP key (switch it only after the host's
move is confirmed; until then never run `freebsd_carp_rc_conf` on it), and
resolvconf's `private_interfaces` lists both. wg0 binds no interface and is
untouched. The old address-migration helper (`README.md`) is history; its
configuration still names re0 and its verbs refuse.

## Helper

`freebsd_bridge_migration_prepare` installs `/usr/local/sbin/gonf-bridge-migration`
(`bridge.sh`), the boot hook `/etc/rc.d/gonf_bridge_rollback` (before netif)
and a minute root cron watchdog. All tasks are Operational.

- `move [s]` (task `_move`, 900 s): needs the root-only single-use gate
  `/var/db/gonf-bridge-migration/move-approved`; on a CARP MASTER also
  `carp-master-approved`, and on f0/f1 devd must be stopped. Preflight
  checks saved and running state (everything on re0, nothing on the bridge,
  default `10.20.0.1` and no other saved or IPv6 default/static route,
  standard switch owning re0, no other migration pending, the loaded PF
  ruleset names vm-public, and on f0/f1 the devd rule admits `-`). It journals the scoped rc.conf/system.conf keys, writes the
  target keys, then moves the running addresses (VIP, IPv4, ULA off re0;
  re0 ifdisabled and its link-local removed; bridge IPv6 enabled; IPv4, ULA,
  VIP on the bridge; default route re-pointed). A MASTER is set MASTER on the
  bridge at once, so f1's master-down timer never fires and NFS stays on f0.
- `confirm`: refuses after the deadline, when the running state is not the
  moved one, and on f0/f1 while devd is stopped.
- `rollback` / watchdog at the deadline: restore keys and running
  addresses (idempotent; a failed step keeps the journal for retry). On
  f0/f1 devd is stopped around the address change (carpcontrol.sh would
  stop NFS on the recreated vhid's INIT->BACKUP step) and started after.
  Archived journals drop the CARP line (the password stays only in rc.conf).
- `boot-trial [s]` (`_boot_trial`, 1200 s): the next boot keeps the
  pending move once while the window lasts; the watchdog still restores
  it at the deadline, and any later unconfirmed boot restores it before
  netif (rc reload with a tagged `unset` line, removed by the watchdog).

`python3 check-bridge.py` runs the transaction against command mocks
(addresses, nd6 flags, CARP, default route, devd, file-backed sysrc),
including refusals, deadline, boot restore with cached rc variables,
boot-trial and failed steps. It is not a native FreeBSD test.

## Per-host procedure

Hold `~/.local/state/conf/live-rollout.lock` (a directory) per host. Receipts:
`~/.local/state/conf/764-YYYYMMDD/<host>/`. Serial: f3, f2, f1 (BACKUP), f0.
f0/f1 are blocked: a CARP member promoted itself while booting on the
bridge (2026-11-12 record below).

0. Once, before the first move: push `freebsd_firewall_config` (pf.conf on
   both interfaces; reload keeps states), `freebsd_carp_devd_hook` (f0/f1)
   and `freebsd_resolver_conf` to all f-hosts. The move refuses without the
   first two. `freebsd_carp_rc_conf` refuses (guard) while a move is pending
   or rc.conf carries the other interface's CARP key.

1. Pre: fresh SSH (laptop wg2 IPv4, lab hosts IPv4/ULA), guest→host ULA
   fails as baseline, wg0 v4/v6, CARP state, services.
2. `freebsd_bridge_migration_prepare`; create the gate(s) as root, mode 0600.
   f0/f1: `doas service devd stop` (f0 also needs `carp-master-approved`).
3. `freebsd_bridge_migration_move`. f0/f1: check `ifconfig vm-public` CARP
   state (f0 MASTER, f1 BACKUP), then `doas service devd start`.
4. Checks: fresh SSH IPv4 (laptop) and ULA (lab hosts), guest→host ULA from
   the host's own guest, wg0 v4/v6 handshakes, CARP, NFS writes from r0–r2,
   zrepl, Garage, k3s nodes/etcd/apps, Gogios.
5. Optional reboot proof while still pending: `boot-trial`, reboot, the
   same checks.
6. `freebsd_bridge_migration_confirm`; f0/f1: switch `CarpNode.Interface`
   to `freebsd.LabBridge`, then `freebsd_carp_rc_conf` must report no change.

## Live record

### 2026-11-11 (paused by the user)

Receipts in `~/.local/state/conf/764-20261111/`. Step 0 is live on f0–f3
(pf.conf on both interfaces, states kept; devd rule on f0/f1; resolver).

- f3 move 16:29:46Z: 0/100 IPv4 pings lost; fresh SSH from the laptop (wg2)
  and ULA TCP 22 from r0–r2, pi0 and f2; the rocky guest now reaches f3 over
  IPv4 (blocked on tap before). The rocky guest has IPv6 disabled on its NIC,
  so guest→host ULA is testable only on f0–f2 (r-nodes). Neighbours with a
  cached re0 MAC re-resolved the ULA within seconds (NUD).
- Native explicit rollback: exact (rc.conf, system.conf, re0 link-local), 0/60 lost.
- Re-move, `boot-trial`, reboot 16:31:51Z: the trial boot created
  `bridge0`, renamed it `vm-public` (clean shutdown and boot). It was not
  confirmed before the 16:51:48Z deadline (the SSH wait used the bare IP
  and failed host-key checks), so the watchdog restored re0 at 16:52.

### 2026-11-12

Receipts in `~/.local/state/conf/764-20261112/`. Wait for fresh SSH with
`-oHostKeyAlias=fN.lan` (or the hostname), not the bare IP.

- **f3 moved and confirmed.** Move 06:41:21Z (0/150 lost), boot-trial
  reboot 06:42:22Z, SSH back 06:44Z on vm-public (members tap0, re0; switch
  `public` manual), cluster checks clean, confirmed 06:45:32Z.
- **f2 moved and confirmed.** Move 06:51:59Z (0/150 lost); r2 (f2's guest)
  → f2 ULA works now, the goal of this task. Boot-trial reboot 06:53:05Z
  (reboots guest r2 too), confirmed 06:57:16Z. The rc.conf bridge keeps its
  MAC across boots but differs from the vm-bhyve bridge's, so a neighbour
  can hold a FAILED entry for the ULA on its first try (NUD re-resolves on
  the next packet).
- **f1 rolled back; CARP hosts blocked.** Move 07:09Z as BACKUP was clean
  (0/150 lost on the host and the VIP). The trial boot was not: the bridge
  came up before re0 had link, f1 heard no advertisements and went
  `BACKUP -> MASTER (master timed out)` at 07:13:01Z for one second. Its
  announcement taught the switch the CARP MAC `00:00:5e:00:01:01` on f1's
  port. f0's advertisements leave with re0's own MAC, so nothing taught it
  back. r1/r2 SYNs and ARPs for 10.20.0.138 reached f1 (BACKUP), their NFS
  mounts failed and public services returned 502/503. The explicit
  rollback at 07:17:45Z restored f1 exactly and the VIP answered again.
  **f0/f1 stay on re0** (f0's helper was never prepared; `CarpNode.Interface`
  stays `re0`). Before a CARP member moves, its boot must not promote on
  the bridge (for example `net.inet.carp.allow=0` until re0 has link), or
  the VIP stays on re0 while only the host addresses move. Needs a decision.
