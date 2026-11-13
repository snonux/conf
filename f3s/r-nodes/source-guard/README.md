# r-node source guard and NodePort pod-source guard (o14)

`f3s-guard.nft` (here) and `../nodeport-guard/f3s-nodeport-guard.nft` are the
live policies of the r-nodes. The initial o14 preparation was dormant; during
the approved r-node migration on 2026-11-06, canonical `rnodes_firewall_stage`
and `rnodes_node_port_guard_stage` were switched to these files (then named
`migration.nft`). Each stage task installs its file as `candidate.nft`;
activation still requires each loader's guarded apply and fresh SSH
confirmation, which persists it as `confirmed.nft` (the boot policy); staging
alone never loads rules. Native acceptance is recorded in the individual
224/324/424 reports. Packet checks load rules only in disposable network
namespaces.

Task f74 (2026-11-13) renamed the files, removed the Operational tasks
`rnodes_firewall_migration_stage` and `rnodes_node_port_guard_migration_stage`
and, with a one-shot retire, their dormant on-host copies
`/etc/f3s-firewall/migration.nft` and `/etc/f3s-nodeport-guard/migration.nft`
(byte copies of `candidate.nft` that no loader read).

The host policy owns only `inet f3s_guard`, at prerouting -150 before DNAT.
It preserves homelab-local service trust for 10.20.0.0/24 and native
fd02:ca0b:f1cb:20::/64, with no new Wi-Fi/camera source trust. Task 074
(2026-11-11) removed all old 192.168.1.x trust after i24 retired those
addresses: the inventoried old infrastructure /32 set (incl. CARP VIP .138),
the controller's old 192.168.1.236 SSH permission and the old destination
/24. Old IPv4 sources now fall through to the drop. The old physical ULA /64 remains
a temporary compatibility permission until all physical IPv6 moves are accepted;
it grants no new global-prefix trust. Native ICMPv6 errors and NDP are allowed;
RA/DHCPv6 receive no service exception. Host RA suppression remains n14's job.

Per-peer wg2 permissions match the router policy: laptop 10.99.0.2 TCP22/80/443
plus the Syncthing NodePort TCP32200 (n44, see `f3s/syncthing/README.md`),
phone 10.99.0.3 TCP443, to lab addresses only. The igc1 recovery aliases
10.98.0.120–122 and their laptop-only TCP22 chain were retired in task j24
(the nodes no longer carry them; laptop replies use the 10.20.0.1 default).
Existing established
traffic may finish. Additional direct wg2 services require a scoped matching
router/host/pod review; no direct SMTP/IMAP/DTail/NodePort exception is implied.
Public wg0 transport UDP56709 stays available. No wg0 identity, IPv6, peers,
AllowedIPs, node/advertise-address, pod or Service CIDR changes are made.

The independent `inet f3s_nodeport_guard` retains its -151 pod-source check,
receiver port list, pod CIDR and fib-local catch. Its policy lists
all three lab IPv4 destinations alongside the wg0 addresses (the old physical
192.168.1.12x were removed in task 074, the recovery 10.98.0.12x in j24). This prevents a pod targeting another node's future alias from
escaping the receiver-port restriction before DNAT/Flannel source NAT. Never
combine the tables or flush kube-proxy/CNI tables. They have separate loaders,
confirmation state and rollback timers.

A policy change is a guarded operation, one node at a time: run the stage
task (it installs `candidate.nft` only), then on the node the loader's
`check`, `apply "$SSH_CONNECTION"` with two independent SSH sessions open,
and `confirm "$SSH_CONNECTION"` from a fresh connection before its five-minute
rollback timer fires. Never load these files with bare `nft -f` on a
production host.

History: during shared-wire staging n14 used the 192.168.1.2 gateway for the
laptop /32 return route, after the cable move the igc1 recovery gateway
10.98.0.1. Task j24 retired both the recovery aliases and that /32 route
(`retire-recovery`), so the nodes answer the laptop through their default.

Flannel has one configuration owner: n14's guarded Promote writes
`/etc/rancher/k3s/config.yaml.d/60-network-migration.yaml` with
`flannel-iface: enp0s5`, promotes the intended 10.20.0.12N primary while keeping
the old secondary, and requests a serial k3s restart under its existing gate.
No alternate selector is added to the routine k3s template. Before activation,
inspect startup flags/environment and later drop-ins for overrides. Verify the
actual primary, VXLAN local address, annotations/FDB endpoints, quorum,
cross-node pod TCP/DNS, Services, ingress and NFS after each node. A file or
Node Ready alone does not establish correct transport. Rollback of a promotion
must restore the physical primary and selector and refresh k3s serially.

`python3 check-offline.py` runs packet checks against the real candidate bytes
only in disposable user/network namespaces. It does not contact lab hosts.
