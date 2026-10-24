# Rocky source firewall — sm3

2026-10-24: **r0 canary deployed and confirmed** from published `a54265e`
(base `7e8e97f`), preserving the deployed co3 NodePort guard. r1/r2 and both
Pis remain pending separate approval. No rpcbind stop, network restart,
mount change or SSH authentication change was performed. The rollout
procedure below requires explicit approval for each further host. The ISP
gateway
192.168.1.1 is ISP-managed; there is no permission to administer it. OpenWrt
192.168.1.101 is an AP and cannot supply the missing perimeter firewall.
FreeBSD PF is separate. Never fail over CARP, reboot into recovery mode,
restart k3s/networking or remount NFS to test this change.

## Observed packet paths and scope

Read-only inspection on 2026-10-22 found:

| Hosts | Physical interface / LAN IPv4 | Other paths / current firewall |
|---|---|---|
| r0/r1/r2 | enp0s5; 192.168.1.120–122/24 | wg0 192.168.2.120–122; cni0/flannel.1 10.42.0.0/16; firewalld off |
| pi2 | eth0; 192.168.1.127/24 | firewalld public zone; Docker Pi-hole uses host networking |
| pi3 | expected eth0/.128, **firewall profile not inspected** | SSH was unavailable on October 22; dn3 later connected. Native firewall preflight and separate approval are still required; never power-cycle for this task |

All inspected hosts have global IPv6 in 2a01:5a8:304:bc3::/64, ULA
fd22:c702:acb7::/64, and IPv6 default route via fe80::1. IPv4 default route is
192.168.1.1. R-node WireGuard actually listens on **UDP 56709**, not 51820.
Flannel listens on UDP 8472, with pod subnet routes through flannel.1 and
peer/control-plane routes through wg0. NFS is NFSv4.2 via loopback
127.0.0.1:2323/stunnel; no NFS or stunnel configuration changes are needed.

Live r-node IPv4 rules route PREROUTING → KUBE-SERVICES → KUBE-NODEPORTS →
DNAT, e.g. NodePort 30090 becomes pod 10.42.1.34:9090 and enters FORWARD.
CNI also DNATs host ports 80/443 to a pod. INPUT-only filtering cannot cover
those paths. IPv6 KUBE-NODEPORTS currently has no endpoints, but must remain
protected if IPv6 services are added. Pi2 has no Docker published-port DNAT;
its Pi-hole listeners reach the host stack because `network_mode: host`.

The dedicated **inet f3s_guard** table hooks prerouting at priority -150:
after conntrack (-200), before destination NAT (-100). The existing co3
`inet f3s_nodeport_guard` runs at -151, with separate files, state, rollback
timer and boot service. Preserve it throughout staging, activation and rollback;
sm3 never owns or restores that table. Only packets arriving
on the physical interface whose original destination is a local address
enter the guard. `fib daddr type local` deliberately omits `.iif`: this also
catches a host address assigned to another interface but reached over LAN.
KUBE/CNI/Docker/firewalld tables and forwarding policies remain untouched.
The native nft(8) “Chains” priority table and “FIB expressions” document these
semantics; the namespace packet test exercises the actual DNAT/FORWARD path.

| Physical-LAN ingress to a local destination | Guard action |
|---|---|
| IPv4 source 192.168.1.0/24 or IPv6 source fd22:c702:acb7::/64 | Return to the existing firewall rules |
| Established/related replies and existing sessions | Preserve until they close |
| ICMP/ICMPv6; link-local DHCPv6 server 547 → client 546 | Preserve |
| R-node UDP 56709 | Preserve encrypted WireGuard transport |
| All other new traffic, including SSH, kube services, metrics, rpcbind, DNS, ingress/NodePorts | Drop before DNAT |

wg0 and pod interfaces do not enter this chain. R-node peer flannel traffic
uses the trusted mesh/LAN. Host-initiated traffic receives its established
replies. “Return” is not an override of another table's deny: pi2 firewalld
remains active and continues deciding which LAN services it permits. Its
existing cockpit rule remains present, with public reachability blocked by
the source guard; removal from the LAN is a separate service-policy change.

**LAN global-IPv6 limitation:** a client on the LAN can select its own global
source when querying a Pi's global IPv6 address. It will be denied too. Do
not claim all LAN IPv6 DNS is preserved. The inspected controller currently
uses IPv4 DNS 192.168.1.101; tracked client guidance uses .127/.128 and the
AP's LAN advertisement was not administratively changed. Before approving a
Pi rollout, inspect affected clients' resolver/source selection and use
192.168.1.127/.128 or the Pis' ULA addresses. Link-local service access is
also outside the trusted service ranges. If clients must retain global
endpoints, stop and review an explicit source policy; do not accept the
whole globally routed internet or silently change ISP/AP configuration.

## Approval-only rollout

Approve **r0 canary staging/activation** first, then r1, r2 and pi2 separately
after acceptance. Pi3 stays excluded until a new read-only inspection confirms
its interface, routes, listeners, Docker mode and native parser, followed by
separate approval. The old task 1g3 SSH-outage observation is historical;
a later successful login does not validate the firewall profile. Do not use
the parallel cluster aggregates for rollout.

1. Record current addresses/routes/listeners, `nft list ruleset`,
   `iptables-save` and `ip6tables-save`, service enablement and baseline
   cluster/pod/DNS/NFS health. Keep recordings private to the operator. Check
   WG listen-port is still 56709 and default interfaces match this profile.
   Check co3 is still present and boot-enabled, and record its policy with
   packet/byte counters normalized before and after each sm3 operation.
   Baseline NFS reads must succeed without changing mounts. Open two
   independent SSH connections using existing authentication; disable
   multiplexing (`ssh -S none -o ControlMaster=no -p 22 ...`). Retain the
   inventory's normal sudo privilege path, including for root SSH. Establish
   both sessions and complete any existing PAM delay before arming the timer;
   never change SSH authentication or logind to speed up this rollout.

2. With approval, stage **one host**. These Operational tasks install only
   candidate files/helper/unit and run daemon-reload; they load no table and
   enable no boot service. They are absent from ordinary aggregates.

   ```sh
   ./gonf.sh push -preview -privilege=sudo -- -p22 -oBatchMode=yes root@r0.lan.buetow.org rnodes_firewall_stage
   ./gonf.sh push -privilege=sudo -- -p22 -oBatchMode=yes root@r0.lan.buetow.org rnodes_firewall_stage
   # Later, separately approved, using existing Pi login/escalation:
   ./gonf.sh push -privilege=sudo -- -p22 -oBatchMode=yes paul@pi2.lan.buetow.org pihole_firewall_stage
   ```

3. In the first root session on the r-node, use the normal sudo path:

   ```sh
   sudo -n /usr/local/sbin/f3s-firewall check
   sudo -n /usr/local/sbin/f3s-firewall apply "$SSH_CONNECTION"
   systemctl status f3s-firewall-rollback.timer --no-pager
   nft list table inet f3s_guard
   ```

   On pi2 use the same sudo helper commands; pass `"$SSH_CONNECTION"` as an
   argument expanded in the existing SSH shell. Never run apply in parallel
   on several hosts. The helper refuses fewer than two established SSH
   connections, snapshots **only its own table** and prior boot policy,
   arms a PID-1-managed five-minute rollback, then atomically replaces its
   own table. No ruleset-wide flush, firewalld reload or iptables restore is
   used. A checksum is recorded only after successful nft application;
   changed files alone cannot suppress a retry after failure.

4. Before the timer expires, verify a **new** SSH connection using LAN IPv4
   and ULA, laptop kubectl over LAN and wg0, node Ready/etcd leader health,
   pod-to-pod and pod DNS, LAN NodePort 30001/30090 and ingress, monitoring,
   and a bounded read of an existing NFS file. Pi2 additionally requires
   UDP/TCP DNS, wildcard response and admin UI from LAN IPv4/ULA and affected
   real clients. Do not restart anything or use CARP failover/remount tests.
   From blowfish, new `nc -6 -z -w 2 ADDRESS PORT` connections must fail to
   r-node 22/111/6443/9100/10250/10257/30001/30090 (also 80/443 hostPorts)
   and Pi2 22/53/80/443/9090/2222. Confirm the source-guard drop counter
   increases where a prior listening port is blocked: a failed connection
   to an already absent listener alone does not demonstrate filtering.
   Verify co3 still denies ordinary pod sources to its five protected
   NodePorts while the intended LAN/mesh clients continue to work.
   Existing public connections are deliberately preserved, so use fresh
   connection attempts. Test UDP DNS separately with `dig +time=2 +tries=1`
   and TCP DNS with `+tcp`; public-source attempts must receive no service
   response. Do not scan Pi3 as a substitute for recovery.

5. After acceptance, **from a new SSH connection** on that host:

   ```sh
   sudo -n /usr/local/sbin/f3s-firewall confirm "$SSH_CONNECTION"
   systemctl is-enabled f3s-firewall.service
   ```

   Confirmation refuses the applying connection, requires an active
   rollback timer and a successfully applied checksum, atomically publishes
   the candidate as confirmed, enables the boot service, and cancels the
   timer. A lock serializes apply/confirm/rollback/boot. The service orders
   before NetworkManager/firewalld/Docker/k3s and network-pre.target. It has
   no ExecStop that could silently remove the guard. It is enabled without
   restarting it: current rules are verified with nft, not service activity.
   Future staging changes only the candidate; boot always loads confirmed
   rules. A reboot during an unconfirmed apply loads the **previous**
   confirmed policy. The r0 canary exercised real PID-1 timer arming and
   cancellation. Reboot persistence and timer-triggered rollback have not
   been exercised on a production host; those remain maintenance checks.

The laptop currently has no active f3s `.200` tunnel, and phone `.201` packet
tests are unavailable. Mesh probes through the frontends and isolated packet
tests are useful checks, but do not prove either device's WireGuard path. Keep
device tests outstanding until the clients are connected; do not claim they
passed or activate their tunnels/change laptop routes without authorization.

## Rollback and interrupted preparation

Do nothing after a failed health check and the independent timer restores
the previous own table and previous confirmed-policy/enablement state within
five minutes, even if SSH/the controller disappears. Or in a surviving
session run `sudo -n /usr/local/sbin/f3s-firewall rollback` through the
existing privilege path. The first deployment restores table absence if none existed before.
Never restore an entire saved ruleset over live kube-proxy/firewalld rules.

Confirmation keeps the immediately preceding snapshot in the root-private
/var/lib/f3s-firewall/previous directory. For a later **separately approved**
reversal, keep two independent SSH sessions and run
`sudo /usr/local/sbin/f3s-firewall revert "$SSH_CONNECTION"`. This restores
the preceding own table, confirmed boot policy and service enablement, even
after the initial rollback timer has been canceled. No network/mount restart.

Preparation builds a complete private `prepared.*` directory under
/var/lib/f3s-firewall and renames it to `pending` before arming the timer.
Failure/interruption before publication can leave `prepared.*` debris but
cannot have loaded a candidate. After inspecting its contents and the live
table, the operator may remove that abandoned preparation directory; do not
delete an active pending snapshot. If interrupted after publishing `pending`
but before arming the timer, no candidate has been loaded: explicitly run
`f3s-firewall rollback` to restore/clear the complete snapshot before retry.
A timer-arm failure itself removes pending and never loads rules. If
rollback fails, pending remains for inspection/retry; keep the second SSH
session, correct the reported native nft/systemd error and rerun rollback.
If a reboot loses the transient timer, boot uses old confirmed policy;
explicit rollback clears pending before any retry. No recovery-mode boot.

## Separate rpcbind cleanup

Only after a successful firewall canary, explicitly approve the one-node
task `rnodes_firewall_rpcbind_off`. It verifies /data/nfs/k3svolumes is nfs4
and rejects any NFSv3 mount, then stops/masks rpcbind.service and its
activation socket. Ordinary `rnodes_nfs_client_units` now manages only
nfs-client.target and will not restart/unmask rpcbind. This baseline change
stops nothing by itself. Native r0 nfs-client.target has no rpcbind Requires.
Check existing NFS reads and workloads again; do not remount or fail over.
Record prior unit states so an approved rollback can unmask and restore
exactly those service/socket enablement/activity states.

## r0 canary acceptance — 2026-10-24

The owner approved publication and **r0 only** after reviewing the prepared
14-file change. GitHub and Forgejo `master` both received `a54265e`; no
canonical pending changes or other repositories were published with it.

- Established two independent trusted root SSH transports and warmed the
  normal sudo path. Strict staging preview passed, then the three installed
  assets matched the reviewed hashes and root ownership/modes. Staging
  loaded no table and enabled no boot service.
- Applied the candidate with a real PID-1 five-minute rollback timer. All
  guarded checks passed, then confirmed from the second transport after
  16.991 seconds. The timer was cancelled, pending state cleared and
  confirmed policy persisted as mode 0600 with its boot service enabled.
  Candidate/confirmed SHA-256:
  `794a9c28c4bbfa174eaa7f0edb77e958568e490f701ffc08eede45641ea435e7`.
- From blowfish's public IPv6 `vio0` route, six host services were reachable
  before activation: TCP 22/111/6443/9100/10250/10257. After activation and
  again after confirmation, none of ten probes connected (those six plus
  80/443/30001/30090). The new guard's drop counter increased during the
  guarded checks; absent listeners were not used as the sole deny proof.
- Ordinary r0 pod probes still denied all 30 combinations of co3's five
  protected NodePorts across the three LAN and three WG node addresses.
  The co3 policy was unchanged with counters normalized and remains
  boot-enabled. Pod-to-host API, API ClusterIP, UDP/TCP Service DNS and
  direct cross-node CoreDNS pod queries all passed.
- Fresh LAN IPv4 SSH with normal sudo and fresh trusted ULA SSH passed.
  LAN registry access, both frontend WG clients' registry/Prometheus/Forgejo
  SSH/Anki paths, normal LAN Player ingress with certificate validation and
  the unchanged unauthenticated HTTP 401 response,
  API readiness/etcd leader health, local registry HTTP and a bounded read
  of the existing public NFS package index all passed before and after.
- Preserved measured baseline restrictions: the laptop's direct LAN
  Prometheus and Forgejo SSH NodePort access was already denied; both
  intended frontend WG clients remained allowed. Direct laptop-to-r0 HTTPS and HTTPS NodePort
  probes also failed before activation; their cause was not established.
  Normal LAN ingress returned its expected HTTP 401. Do not describe those
  direct failed pre-change paths as successful tests or as a new firewall regression.
- k3s PID/start time and both rpcbind unit states remained unchanged.
  Cluster checks retained 3 Ready nodes, 54 production pods with unchanged
  UIDs/readiness/restart counts, 40 Healthy/Synced Argo apps, 57 unchanged
  NetworkPolicies and 43 healthy Prometheus targets. Seven r0-specific
  targets subsequently scraped successfully after activation. Both normal
  Gogios reports refreshed after activation with no critical, unknown or
  stale checks; their existing Job warning was unchanged.
- Repeated packet/client checks after confirmation, then deleted the one
  temporary unprivileged probe using its exact UID as a delete precondition.
  No test NetworkPolicy or workload deployment was needed.

No reboot, real timer expiry/rollback, CARP failover, remount, SSH-policy
change or rpcbind cleanup was performed. Laptop/phone f3s tunnel packets
remain untested as described above. Full sm3 is **not complete**: r1/r2,
Pi2/Pi3, separately approved rpcbind cleanup and device checks remain.
Private receipts are under
`~/.local/state/conf-worktrees/sm3-integration-20261024/live/`.

## Verification performed during preparation

- October 22: native read-only `nft -c -f -` passed on r0/r1/r2/pi2; pi3
  was excluded. Those historical checks are not fresh fleet acceptance.
- October 24: r0 native `nft -c -f -` passed for the candidate rendered
  from the actual Gonf plan. Its deployed co3 table was unchanged with
  counters normalized, and its boot service remains enabled. The sm3 table
  and staging files remain absent; WG still listens on UDP 56709 and NFS
  remains nfs4. Existing Gonf 0.25.2 supports plan 31/strict-preview 1, so
  no controller runtime bootstrap was needed.
- October 24: the actual strict r0 `rnodes_firewall_stage` preview passed:
  10 plan operations, zero changes, six would-change actions limited to
  the new candidate/helper/unit/directory, restorecon and daemon-reload.
  No live files were staged and no rule or service state changed.
- `f3s/tests/linux-source-firewall.sh` runs native nft in isolated user/network
  namespaces: prior reachable public host and DNAT/FORWARD NodePort paths
  become blocked; LAN/ULA services, UDP DNS, LAN NodePorts, WG transport,
  host-initiated replies and pod-to-host paths survive. Repeated atomic
  replacement, failed-transaction preservation and own-table rollback leave
  the unrelated simulated kube-proxy table intact. Pi profile has no WG
  exception. These are isolated packet tests, not deployed-host acceptance.
- `f3s/tests/linux-firewall-lifecycle.sh` executes the actual helper with
  native nft in isolated user/network/mount namespaces. Fake systemd and SSH
  metadata exercise load failure, rollback dispatch, timer-arm failure,
  same-session confirmation refusal, new-session persistence, and boot
  during pending apply. The October 24 integration additionally loads co3
  from its actual candidate and asserts that its table is unchanged after
  every helper operation, including failure and rollback paths.
  This simulates PID-1 timer behavior; it does not
  claim a real host timer was armed or fired.
- October 24 integration: shell syntax/ShellCheck and full Go generate,
  build, test, vet and errcheck passed. No conf Go unit tests were added.
  All runtime helper/policy/unit/profile files and the original packet
  test are byte-identical to reviewed `4cd0137` (round 1 passed with no
  findings). The current integration retains both task registrations and
  adds the co3 lifecycle assertions above. Further fleet acceptance remains;
  the separately approved r0 canary is recorded above.
