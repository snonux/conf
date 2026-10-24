# NodePort pod-source guard — co3

Prepared locally on 2026-10-24; **not published, installed or activated**.
Fresh expert review and explicit approval for staging, activation and temporary
cluster probes are still required. Task 7n3 is deployed; its policies and the
existing Prometheus boundary remain unchanged.

## Problem and current evidence

An ordinary r0 pod reached r1 NodePorts 30001, 30222, 30143, 30025 and 30800
during the approved 7n3 tests, despite direct Service/Pod receiver denial.
Kube-proxy's Cluster NodePort path can replace the source with a trusted node.
Read-only inspection now also confirms Flannel's source-node masquerading on
all three nodes: traffic from 10.42.0.0/16 to destinations outside that network
can lose its pod identity before it reaches another node. A receiving-node-only
pod-source filter would therefore miss part of the observed bypass.

All three current pod CIDRs are IPv4 subnets of 10.42.0.0/16. Node LAN addresses
are 192.168.1.120–122 and WG addresses 192.168.2.120–122; native interface
inventory found no uncovered non-loopback, non-pod IPv4 address. All five
NodePorts still use `externalTrafficPolicy: Cluster`.

| NodePort | Receiver | Pod port |
|---|---|---|
| 30001 | infra/docker-registry-service | 5000 |
| 30222 | services/forgejo-ssh | 2222 |
| 30800 | services/anki-sync-server-nodeport | 8080 |
| 30143 | services/protonbridge IMAP | 143 |
| 30025 | services/protonbridge SMTP | 25 |

## Candidate

`f3s/r-nodes/nodeport-guard/candidate.nft` owns only the inet table
`f3s_nodeport_guard`. Its prerouting filter runs at priority -151, before both
kube-proxy destination NAT and Flannel source NAT. For a pod source in
10.42.0.0/16 and one of the five TCP ports, it drops traffic to:

- Any of the six node LAN/WG addresses, including another node's address.
- Any locally routed address, including local CNI gateways.

The first condition catches cross-node traffic at its source. The second
covers local aliases; remote CNI destinations remain within the pod network and
reach the destination guard with their pod source intact. **Every r-node must
be activated before global closure can be claimed.** A successful r0 canary
alone does not protect pods originating on r1/r2.

This is the ordinary-pod bypass control required by co3. It does not establish
an external-client allowlist: existing non-pod client SNAT and node/hostNetwork
trust remain. Task sm3 owns the separate host/interface source firewall. Direct
application Service/Pod isolation remains kube-router's responsibility.

There is no Service routing change, receiver-policy expansion, HTTP-header trust
change, or catch-all pod-egress block. Traffic to a third-party host using one
of these port numbers remains usable. Host-originated localhost registry pulls,
LAN clients and WG sources .110/.111/.200/.201 remain outside the pod-source
rule. Real laptop/phone tunnels were not activated for these tests.

The node address set and pod CIDR are part of the reviewed candidate. Before
changing node addresses, adding a node or enabling another pod CIDR/IPv6 family,
update and revalidate the guard; do not assume the current static set covers it.

## Staging and activation ownership

The Operational task `rnodes_node_port_guard_stage` installs only:

- `/etc/f3s-nodeport-guard/candidate.nft`, root:GID0 0600, parent 0700.
- `/usr/local/sbin/f3s-nodeport-guard`, root:GID0 0755.
- `/etc/systemd/system/f3s-nodeport-guard.service`, root:GID0 0644.

Its only commands are restorecon and a change-gated systemd daemon-reload.
It does not load a policy, enable/start a service or join the ordinary rnodes
aggregate. Never run the whole aggregate for this task: SSH, NFS and unrelated
maintenance are outside its scope.

The activation helper adapts the previously reviewed sm3 guard flow, with
separate paths, table, locks, rollback timer and boot unit. It neither changes
nor activates sm3's `f3s_guard` table. A later sm3 rollout must preserve this
table; its priority -150 follows this guard's -151.

Activation requires two independent SSH transports on port22. Applying arms a
five-minute PID1 rollback timer before loading the candidate. Confirmation must
come from a different SSH_CONNECTION and persists the candidate for boot.
Stopping the boot service does not remove protection. There is no SSH reload,
host reboot or recovery boot in this procedure.

## Checks already run

- Native `nft -c` passed on r0, r1 and r2 with their actual nftables 1.0.9
  environment. An initial 25-second check timed out; the bounded retry through
  dedicated read-only SSH transports completed. No rules were loaded. The
  custom table was verified absent afterwards and all three transports closed.
- Native Linux user/network namespace packet tests reproduce source-node
  masquerading, Cluster NodePort destination NAT and receiver pod-source denial.
  Ninety unguarded source/address/port cases succeed. A destination-only guard
  still admits all five negative-control attempts, whose server-observed source
  is the masqueraded node gateway. With both guards active, 135 cases are denied.
  Across two simulated endpoint placements, 150 WG/LAN client cases and five
  host-local cases remain positive per placement. Direct receiver denials and
  unrelated/outgoing paths remain unchanged. Native counters increase on both
  simulated nodes. Atomic failure leaves protection intact; own-table removal
  restores the original control behavior. These are isolated kernel tests,
  **not deployed-cluster acceptance**.
- Actual-helper lifecycle tests use native nft in isolated user/network/mount
  namespaces. Systemd/SSH metadata are simulated. They cover one-session
  refusal, failed load, rollback dispatch, same-session confirmation refusal,
  new-session persistence, pending-boot behavior, timer-arm failure and explicit
  post-confirm revert. The other `f3s_guard` table remains unchanged. This does
  not claim a real host's PID1 timer has been exercised.
- Bash syntax and ShellCheck pass. Go generation/build/test/vet/errcheck pass;
  the generated r-node descriptions are included. An actual controller plan
  has ten operations, correct owners/modes, `nft -c`/Bash validation and only
  the two inert staging commands.

Run the local checks from this checkout:

```sh
f3s/tests/nodeport-guard.sh
f3s/tests/nodeport-guard-lifecycle.sh
shellcheck f3s/r-nodes/nodeport-guard/f3s-nodeport-guard f3s/tests/nodeport-guard*.sh
```

The tests refuse the real host user/network namespaces before mutating routes
or rules. Their servers and namespace processes are cleaned at exit.

## Proposed live sequence — approval required

1. Recheck node CIDRs/addresses, the five Services/endpoint placements, native
   source-NAT rules, the existing guard/table absence or ownership, and current
   app/metrics/Gogios baselines. Stop for an unexplained change. Preserve all
   7n3/6n3 policy specs and the Argo chart overlay. Check required LAN/WG clients,
   node-local registry pulls and the existing pinned-certificate Bridge tunnel.
2. Publish only the reviewed co3 delta on the current approved GitHub/Forgejo
   source. Build that checkout's controller, then use its exact binary and
   GONF_CONF_ROOT for strict single-host previews. Keep ignored secrets in their
   canonical controller directory; do not copy them into a worktree.
3. Roll out r0, then r1, then r2, with no parallel activation. A strict preview
   must show only the three files, parent directory, restorecon and daemon-reload.
   Stop if a runtime bootstrap or any unrelated change is needed. Only after
   this gate run the named staging task on that host.
4. Open two independent SSH connections (`ControlMaster=no`, `-S none`), keep
   both open, and record the prior own table/boot state in a root-private backup.
   From the first connection, run:

   ```sh
   /usr/local/sbin/f3s-nodeport-guard check
   /usr/local/sbin/f3s-nodeport-guard apply "$SSH_CONNECTION"
   ```

   Verify the real rollback timer is active before testing. For this stage,
   ordinary clients on this source node must fail all five ports through local
   and remote LAN/WG node addresses. Check native counters. Required gateway
   clients, registry pulls, mail tunnel, apps and metrics must retain baseline
   health. If the five-minute window is insufficient, let rollback restore the
   prior policy; never confirm untested protection to beat the timer.
5. From the second independent transport, after those gates:

   ```sh
   /usr/local/sbin/f3s-nodeport-guard confirm "$SSH_CONNECTION"
   ```

   Verify the real timer stopped, confirmed bytes/ownership and boot enablement.
   Then proceed to the next host. Keep the bounded ordinary clients unprivileged,
   tokenless, without host networking or trusted app labels; record their UIDs.
6. After all three hosts are confirmed, repeat the complete ordinary-client
   Service/Pod/NodePort matrix from each node. Repeat real allowed-client reads,
   raw/proxy Prometheus denials, native DNS, app readiness/restarts, targets/rules
   and normal monitoring cycles. Existing failed Jobs remain a recorded baseline.
7. Validate a real Kubernetes endpoint transition without moving an existing
   workload: use an unused reserved test NodePort30417, a UID-recorded temporary
   Service in test, and bounded canary pods on r0/r1. Its exact-label temporary
   allow policy is scoped to trusted node/CNI and gateway sources on8080.
   Temporarily add 30417 to the owned `receiver_ports` set on each host only after
   verifying it was absent. Observe one ready endpoint, add the second, then
   remove only the first owned pod. At each transition, ordinary pod connections
   remain denied and trusted gateway reads succeed through every node. Record
   EndpointSlices and counters. This step, including the temporary port/set
   changes and resource creation/deletion, is part of the approval request.
8. Remove only owned temporary resources by UID and the task-added 30417 set
   element. Require final kernel rules to match the reviewed five-port candidate
   apart from counters, confirmed files unchanged and no probe resources left.
   Do not change an existing application's endpoint, force a CronJob, write
   mail/data or restart Kubernetes, networking, mounts or hosts for acceptance.
9. Reapply only staging and require convergence; verify it does not reload or
   disable confirmed protection. Record actual observed limitations, update the
   policy runbook/f3s skill, and close co3 only after the live checks pass.

## Rollback

For an unconfirmed stage, either let the independent timer fire or run the
helper's `rollback`. For a confirmed stage, use `revert "$SSH_CONNECTION"`
with the required second transport kept open. Both restore only the saved own
table and prior confirmed-file/boot state. Never flush the whole ruleset or
remove kube-proxy, kube-router or sm3 tables. Verify baseline clients and metrics
after restoration; report rollback as baseline restored, not bypass fixed.

Sources: [Kubernetes NodePort source behavior](https://kubernetes.io/docs/tutorials/services/source-ip/),
[Netfilter hook ordering](https://wiki.nftables.org/wiki-nftables/index.php/Netfilter_hooks)
and [nft route-type matching](https://wiki.nftables.org/wiki-nftables/index.php/Matching_routing_information).
