# r-node source guard and NodePort pod-source guard (o14)

`f3s-guard.nft` (here) and `../nodeport-guard/f3s-nodeport-guard.nft` are the
live policies of the r-nodes, staged by `rnodes_firewall_stage` and
`rnodes_node_port_guard_stage`. Each stage task installs its file as
`candidate.nft`; activation requires each loader's guarded apply and fresh
SSH confirmation, which persists it as `confirmed.nft` (the boot policy);
staging alone never loads rules. Packet checks load rules only in disposable
network namespaces. (Both files were named `migration.nft` from the r-node
migration on 2026-11-06 until task f74 renamed them and removed the
migration stage tasks and their dormant on-host copies.)

The host policy owns only `inet f3s_guard`, at prerouting -150 before DNAT.
It preserves homelab-local service trust for 10.20.0.0/24 and native
fd02:ca0b:f1cb:20::/64, with no new Wi-Fi/camera source trust. Task 074
(2026-11-11) removed all old 192.168.1.x trust after i24 retired those
addresses: the inventoried old infrastructure /32 set (incl. CARP VIP .138),
the controller's old 192.168.1.236 SSH permission and the old destination
/24. Old IPv4 sources now fall through to the drop. The old physical ULA /64 remains
a temporary compatibility permission until all physical IPv6 moves are accepted;
it grants no new global-prefix trust. Native ICMPv6 errors and NDP are allowed;
RA/DHCPv6 receive no service exception. Host RA suppression is in the
NetworkManager profile (set by n14; its helper was removed in c74).

Task o74 (2026-11-13) narrows that lab trust for the five
`externalTrafficPolicy: Cluster` NodePorts, set `cluster_nodeports` = 30001
registry, 30222 Forgejo SSH, 30800 Anki, 30143/30025 Bridge. Their receiver
NetworkPolicies admit each node's flannel address 10.42.N.0, and a node that
does not host the pod masquerades a NodePort connection to exactly that
address, so any trusted lab host reached them through a non-hosting node
whatever the policy said. Chain `cluster_nodeport` now admits only the
r-nodes 10.20.0.120-122 (all five) and the rocky build VM 10.20.0.123 (30001
only, matching `registry-nodeport`); every other enp0s5 source, IPv6
included, is counted and dropped there. The frontends' relayd and the node-
local registry mirror are untouched: they use wg0 and localhost, which never
enter this table. The Local NodePorts 30090/30091/32200 and Traefik's
31637/30154 keep the client source, so their own policies decide and they are
not in the set. Keep the set equal to the Cluster NodePorts of `kubectl get
svc -A` and to `receiver_ports` of `f3s_nodeport_guard`. Other mesh peers that
reached a node over wg0 (measured from f0) kept the cross-node SNAT path
until task u74, below.
Rolled out on 2026-11-13 to r2, r1, r0, one node at a time under the live
rollout lock, through `f3s-firewall check/apply/confirm` (candidate =
confirmed SHA256 `72ce4fc0d0a4bf0b8a00b369f5d4f4780e70cb3e7349240dee4c02e5cc2a186f`);
`f3s_nodeport_guard` stayed byte-identical. Afterwards f0 timed out on all
five ports on every node, rocky reached 30001 on every node (and nothing
else), the frontends over wg0 and every node to itself and its peers kept
their access, and the `from_lan` drop counter stayed 0.

Task u74 (2026-11-13) closes the same bypass for wg0 mesh peers. New wg0
TCP to a local address on a `cluster_nodeports` port jumps into chain
`wg0_cluster_nodeport`, which admits exactly the wg0 ipBlocks of the receiver
policies: the r-nodes 192.168.2.120-122 and the laptop/phone .200/.201 on all
five ports, the frontends .110/.111 (relayd) only on 30001/30222/30800, the
ports relayd forwards; `protonbridge-nodeport` does not admit them. Every
other mesh peer (f-hosts, Pis, rocky routed through a gateway) and all IPv6
(the Services are IPv4-only, no policy admits IPv6) is counted and dropped.
Established flows and every other wg0 port (etcd, API, kubelet, NFS,
monitoring) are not inspected. `check-offline.py` derives the wg0
expectations from the four policies in `f3s/network-policies/namespaces`
(map `NODEPORT_POLICIES`), so a policy change fails the check until the
guard follows it; the o74 file fails it, as it admits every mesh peer. Before the change a 5-minute SYN capture on wg0 of all
three nodes saw no source other than .110/.111 on these ports.

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

Flannel's interface comes from
`/etc/rancher/k3s/config.yaml.d/60-network-migration.yaml`
(`flannel-iface: enp0s5`) on r0-r2. n14's network-migration helper wrote it;
the helper went in task c74 and the file has no gonf owner now (kept on
purpose, rename or adopt later; IQ `openbsdrouter-plan/CLEANUP.md` item 1).
No alternate selector is in the routine k3s template. After a change that
touches it, verify the VXLAN local address, FDB endpoints, quorum,
cross-node pod TCP/DNS, Services, ingress and NFS on each node: Node Ready
alone does not prove the transport.

`python3 check-offline.py` runs packet checks against the real candidate bytes
only in disposable user/network namespaces. It does not contact lab hosts.
