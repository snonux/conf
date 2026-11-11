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

Hold `~/.local/state/conf/live-rollout.lock`. Receipts:
`~/.local/state/conf/764-20261111/<host>/`. Serial: f3, f2, f1 (BACKUP), f0.

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
