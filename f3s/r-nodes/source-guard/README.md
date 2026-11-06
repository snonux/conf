# Physical-source and pod-source migration candidates (o14)

The initial o14 preparation was dormant. During the approved r-node migration
on 2026-11-06, canonical `rnodes_firewall_stage` and
`rnodes_node_port_guard_stage` were switched to these migration files.
Activation still requires each existing loader's guarded apply and fresh SSH
confirmation; staging alone never loads rules. Native acceptance is recorded
in the individual 224/324/424 reports. Packet checks load rules only in
disposable network namespaces.
`rnodes_firewall_migration_stage` installs `/etc/f3s-firewall/migration.nft`;
`rnodes_node_port_guard_migration_stage` installs
`/etc/f3s-nodeport-guard/migration.nft`. Both tasks are Operational and validate
the dormant files with native `nft -c`. These dormant-file tasks leave `candidate.nft` files and
confirmed boot policies remain unchanged. Neither task installs or activates
a loader, changes systemd state, or requests a restart.

The host policy owns only `inet f3s_guard`, at prerouting -150 before DNAT.
It preserves homelab-local service trust for 10.20.0.0/24 and native
fd02:ca0b:f1cb:20::/64, with no new Wi-Fi/camera source trust. New lab sources
can reach retained old local IPv4 aliases during mixed-address staging.
Temporary old IPv4 trust is restricted to inventoried infrastructure peers;
the controller's current 192.168.1.236 address gets SSH only. Confirm the
controller address again before activation. The old physical ULA /64 remains
a temporary compatibility permission until all physical IPv6 moves are accepted;
it grants no new global-prefix trust. Native ICMPv6 errors and NDP are allowed;
RA/DHCPv6 receive no service exception. Host RA suppression remains n14's job.

Per-peer wg2 permissions match the router policy: laptop 10.99.0.2 TCP22/80/443,
phone 10.99.0.3 TCP443, to lab addresses only. Recovery aliases
10.98.0.120–122 accept only laptop TCP22. Recovery filtering precedes lab and
old-source trust; it is not a general service subnet. Existing established
traffic may finish. Additional direct wg2 services require a scoped matching
router/host/pod review; no direct SMTP/IMAP/DTail/NodePort exception is implied.
Public wg0 transport UDP56709 stays available. No wg0 identity, IPv6, peers,
AllowedIPs, node/advertise-address, pod or Service CIDR changes are made.

The independent `inet f3s_nodeport_guard` retains its -151 pod-source check,
receiver port list, pod CIDR and fib-local catch. Its migration candidate adds
all three new lab and recovery IPv4 destinations alongside old physical and
wg0 addresses. This prevents a pod targeting another node's future alias from
escaping the receiver-port restriction before DNAT/Flannel source NAT. Never
combine the tables or flush kube-proxy/CNI tables. They have separate loaders,
confirmation state and rollback timers.

Later activation is a separate guarded operation, one node at a time. First
verify the existing `f3s-firewall` and `f3s-nodeport-guard` tools and confirmed
boot policies, native rollback expiry/boot proof, fresh old-IP SSH, intended
listener/return routes and actual source inventory. Archive each current
candidate. After review explicitly copy each dormant migration file to its own
loader's `candidate.nft`; run that loader's `check`, then `apply` with the
actual SSH connection tuple. Follow each loader's existing independent fresh
connection/confirm/rollback procedure. Do not load these files with bare
`nft -f` on a production host. Update the repository's canonical stage source
only at that approved policy transition so a routine stage cannot overwrite a
selected migration candidate. Native expiry, reboot and fresh reconnect
acceptance remain unperformed by this preparation.

During shared-wire staging n14 uses the reachable 192.168.1.2 gateway for the
laptop /32 return route. Do not activate the igc1 recovery route while still
on shared igc0. Recovery aliases/return routing become usable only through
the later human cable and fresh igc1 recovery gates. The host candidate is
safe to prepare before either address exists; preparation proves no path.

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
