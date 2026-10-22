# Prometheus exposure controls (task 6n3)

The reviewed runtime configuration was deployed with explicit approval on
2026-10-22 from commit `5327733eee5249d57016491b96b4c4d4748d8910`.
Only the two Prometheus commits were published; other prepared changes stayed
unpublished. Live evidence is recorded below. Future publication, reconciliation
and destructive maintenance still require approval.

## Consumers and chosen boundary

Read-only inspection on 2026-10-22 found Prometheus 3.15.0 with remote-write,
admin and OTLP flags enabled. Both `/api/v1/write` and
`/api/v1/otlp/v1/metrics` self-metric request counters had zero increases over
the last 10 days of available data. No active Deployment, StatefulSet or
DaemonSet provided a remote-write/OTLP metrics sender.

- Gogios on blowfish and fishfinger uses GET `/api/v1/alerts`. Its three
  `rN.wg0:30090` failover addresses stay unchanged; each r-node gets a local
  proxy via a DaemonSet. This preserves Watchdog and firing-alert checks.
- Laptop alert commands default to `r0.wg0.wan.buetow.org`, resolving to
  192.168.2.120. Route over WireGuard from earth (192.168.2.200), even on the
  LAN. The old short `r0` default could choose a LAN source and is removed.
  GET queries, ranges, series/label discovery, exemplars, metadata, rules and
  targets also work through the proxy. UI access and POST queries use
  `just port-forward-prometheus` on localhost. Other LAN/WG sources are denied.
- Epimetheus is an ad-hoc historic remote-write consumer. Keep the receiver
  and 31-day out-of-order window; imports use the existing localhost-only
  Kubernetes port forward to `/api/v1/write`, with Kubernetes authentication
  and port-forward RBAC. No production sender URL needs changing.
- Alloy, Tempo and tracing-demo ArgoCD Applications are disabled. The demo
  exports **traces** to Alloy:4317; the saved Alloy config forwards traces to
  Tempo:4317. Neither sends metrics to Prometheus's OTLP endpoint. Disable
  Prometheus OTLP. Re-enabling a metric sender later requires its own scoped
  receiver access review; do not broadly allow the services namespace.
- Grafana is disabled. Its saved datasource points to the direct ClusterIP.
  Before re-enabling it, route its queries through the read proxy and add a
  same-namespace Grafana pod selector to the proxy policy; avoid granting it
  the direct receiver port.

`prometheus-nodeport` keeps service port 9090 and node port 30090 but selects
the proxy's 8080 port. `externalTrafficPolicy: Local` preserves client IPs
for its NetworkPolicy. An explicit nginx GET/HEAD route allowlist rejects
admin, write, OTLP, lifecycle and unknown paths, including GET mutations.
It also excludes config and flags. Access logs are off and HTTP-context
error logs go to `/dev/null`, since nginx upstream failures otherwise include
the full query URI. Main-context startup diagnostics stay on stderr. Use
HTTP status/readiness to detect upstream failures; request-context error
details are deliberately discarded to avoid retaining private label values.

The Prometheus ingress policy admits only proxy and Prometheus pods in the
same namespace on TCP 9090. The latter preserves its ServiceMonitor
self-scrape; config-reloader uses localhost. No egress policy is added, so
scraping, DNS, API discovery, Alertmanager delivery and storage are unchanged.
No PVC, PV, NFS mount, alert rule, scrape target, host service or firewall is
modified. Existing NFS-mount-monitor alerts remain enabled.

## Trusted nodes and policy coordination

Read-only `iptables-save -t filter` on r0 showed 80 `KUBE-NWPLCY` and 138
`KUBE-ROUTER` rule references: the k3s policy controller is programming the
existing cicd policies. API objects alone were not used as enforcement proof.
Still verify packet behavior on each node after approved deployment.

Kubernetes always permits traffic between a pod and its local node.
Host-network sources and node-originated traffic are trusted exceptions;
these policies do **not** isolate a compromised node or host-network pod.
An authorized port-forward also bypasses pod ingress isolation. Keep node
access and Kubernetes pod/exec/port-forward permissions restricted.

Task 7n3 owns general NetworkPolicies. These two policies are the necessary
Prometheus boundary only. Policies are additive: a later monitoring-wide
ingress allow can reopen 9090. Preserve this isolation when implementing
7n3, and account for Prometheus egress to scrape targets if 7n3 isolates egress.

References: [Kubernetes NetworkPolicy node exceptions](https://kubernetes.io/docs/concepts/services-networking/network-policies/),
[source preservation with Local services](https://kubernetes.io/docs/tutorials/services/source-ip/),
[Prometheus Operator API](https://prometheus-operator.dev/docs/api-reference/api/),
[Prometheus 3.15 disabled-admin response implementation](https://github.com/prometheus/prometheus/blob/v3.15.0/web/api/v1/api.go).

## Local validation

Requires Helm, Python/PyYAML and rootless Podman. The test starts only a local
stub upstream and local nginx, using the production config and pinned image;
it does not contact the cluster. The chart archive includes its dependencies.

```sh
helm pull https://github.com/prometheus-community/helm-charts/releases/download/kube-prometheus-stack-92.10.1/kube-prometheus-stack-92.10.1.tgz --destination /tmp
podman pull docker.io/library/nginx:1.30.6-alpine@sha256:c59d1f8b1e0e42da75be973424cd72e23f7f8adfb54e982f4d1cd0216a5a5576
HELM_CHART=/tmp/kube-prometheus-stack-92.10.1.tgz python3 -m unittest discover -s f3s/prometheus/tests -v
just --justfile f3s/prometheus/Justfile --dry-run firing-alerts
```

The tests render both authoritative and example Helm values, check disabled
admin/OTLP and retained historic-write/storage settings, evaluate the scoped
source rules, and exercise real nginx GET queries plus method/path denial.
A synthetic private-label query with an upstream failure must return 502
without its sentinel or request URI appearing in stdout/stderr; a separate
invalid-startup test confirms startup diagnostics are still retained.
Source-policy evaluation is an offline model, not a live enforcement test.

## Deployment and live acceptance procedure

Schedule a Prometheus rollout: changing its receiver flags causes an Operator
rollout/WAL replay. Do not combine it with NFS/CARP maintenance or restarts.

1. Publish only after approval. Argo sync waves stage the read-proxy
   ConfigMap (-2) and DaemonSet (-1), then the policies and Helm resources
   (0), then the NodePort selector/Local policy (1). Wait for a ready
   endpoint on **all three** r-nodes before completing acceptance; Argo's
   DaemonSet health must pass before it switches the NodePort. For a manual
   deployment, preserve the same ordering. On future proxy config edits,
   roll the DaemonSet
   after approval: a subPath ConfigMap mount does not hot-reload nginx.
2. Reconcile the complete Application, including native receiver flags and
   both policies; avoid partial commits that leave the raw NodePort open.
   A brief scrape/alert-evaluation gap during WAL replay is expected; keep
   the changes together and verify both frontend check cycles afterward.
3. Read the Prometheus resource **and the running pod arguments**. Confirm
   remote-write enabled; admin and OTLP disabled; no additional enabling flags
   or `otlp-write-receiver` feature remain. Wait for Prometheus and proxy
   readiness on every node. Storage objects and mounts must be unchanged.
4. From both frontends and earth's WireGuard IP, GET `/api/v1/alerts` through
   port 30090 on r0/r1/r2. Exercise `just firing-alerts`, `all-alerts` and
   `firing-count` from earth, and a GET PromQL `up` query. Confirm Watchdog
   and all previously healthy targets, alert rules and NFS-monitor metrics.
   Verify Gogios's next two cycles retain a healthy Prometheus/Watchdog check.
5. Only **after step 3 confirms admin disabled**, test POST to
   `/api/v1/admin/tsdb/clean_tombstones` through the proxy (405) and the
   localhost port forward (a non-success response with `admin APIs disabled`).
   Verify the disabled error body, rather than requiring 404 from direct
   Prometheus; status mapping can differ by version. No admin/delete or
   receiver POST probes were run during prep.
6. After flag verification, proxy POST `/api/v1/write`,
   `/api/v1/otlp/v1/metrics` and `/-/reload` must return 405, and GET variants
   404. A separately approved historic import may use the localhost forward;
   do not inject test series merely to validate denial. No cleanup deletion.
7. From an ordinary application pod (not hostNetwork, not a node), direct
   Prometheus Service/Pod TCP 9090 must time out; its proxy port must also be
   denied. Repeat against each r-node NodePort from an unapproved LAN/WG
   source. Authorized source queries must still succeed. Inspect policy
   counters rather than assuming an HTTP failure proves a policy drop.

If acceptance fails, stop and inspect before undoing isolation. For an
availability workaround, operators can use the authenticated port forward;
do not restore raw Prometheus NodePort exposure or re-enable admin globally.

## 2026-10-22 rollout evidence

- Published only `e907f31` and `5327733` by normal fast-forward from
  `82664386c906a94d0a6ba1cd88dc650443384557` to GitHub and then Forgejo.
  Forgejo's repository API reported `mirror: false`: it is a regular repo,
  so publishing to GitHub alone did not advance Argo's watched source.
  The exact same approved commit was then explicitly authorized for Forgejo;
  no force push, unrelated commits or Argo source changes were used.
- Applied only the approved Prometheus Application manifest and refreshed that
  Application. Argo reached Synced/Healthy/Succeeded at chart 92.10.1 and
  Git revision `5327733eee5249d57016491b96b4c4d4748d8910`. Its sync waited
  for the read-proxy DaemonSet before switching the NodePort. All three proxy
  pods and service endpoints were ready, with zero proxy restarts.
- The live Prometheus resource retained a stale `otlp-write-receiver` feature
  after initial reconciliation. A scoped JSON replacement aligned that list
  with the reviewed `[exemplar-storage]` value. Final running arguments and
  GET `/api/v1/status/flags` confirmed admin=false, OTLP=false,
  remote-write=true and only exemplar-storage; no enabling additionalArgs.
  Prometheus completed its approved rollout and was ready 2/2. Startup
  detected out-of-sequence mmap chunks; Prometheus's own recovery discarded
  the corrupt mmap files and replayed WAL. No manual TSDB deletion or storage
  maintenance was performed.
- Both frontends successfully queried alerts and a real PromQL `up` query
  through every r-node's WG NodePort. Earth did the same from the approved
  `192.168.2.200` source, with firing Watchdog observed on every node.
  Its existing full-tunnel wg0 profile stayed down: an explicitly approved
  runtime-only, root-private fixture reused its identity with routes restricted
  to `192.168.2.0/24`. All three Justfile alert commands passed. The fixture
  interface, routes and secret-bearing temporary files were removed; persisted
  config bytes and the existing default route were unchanged.
- Only after the disabled flags were verified, proxy GET admin/write/OTLP/
  reload routes returned 404 and POST returned 405 on all three nodes.
  A direct localhost port-forward admin probe returned HTTP 500 with
  `admin APIs disabled`. No remote-write/OTLP payload was sent to Prometheus,
  no data series were injected/deleted, and no cleanup ran. The authenticated
  direct port-forward remained available for historic imports.
- Three ordinary, non-hostNetwork probe pods, one on each r-node, each failed
  all eight access attempts: direct Prometheus Service and Pod, all three
  proxy Pod IPs, and all three NodePorts. Destination REJECT counters increased
  on proxy pods by r0=9/r1=6/r2=8 and on the raw Prometheus pod by 8. Counters
  were captured before deleting the probes because policy chains are rebuilt
  as pod membership changes. All probe pods were deleted afterward.
- Earth LAN source `192.168.1.6` and f0 WG source `192.168.2.130` each failed
  access to all three NodePorts. The proxy REJECT counter rose by exactly two
  on every r-node across these six requests. A pi0 source attempt was excluded
  from evidence because its route to r-node WG addresses used its LAN gateway.
- Post-rollout target health matched the pre-rollout baseline: 42/43 up;
  only the existing `192.168.2.133:9100` f3 exporter was down. The r0/r1/r2
  NFS-mount-monitor failure metrics were present and all zero. Watchdog stayed
  firing; 41 rule groups / 270 rules were all healthy. The only changed target
  identities were the restarted Prometheus pod's own 8080/9090 endpoints.
  `GoprecordsFleetUploadStale` returned to pending after the restart while its
  `for` window accumulated again; the rule was retained and healthy. Existing
  TargetDown and KubeJobFailed alerts remained firing. No NFS/CARP operation,
  unrelated host restart, SSH authentication
  change, recovery boot or frontend recipe deployment occurred.
- Normal active Gogios cycles completed at 11:21:59 and 11:26:54 UTC, with
  Prometheus alerts and Watchdog both OK (check epochs `1792668119` and
  `1792668414`). The passive frontend's reports at 11:25:11 and 11:30:10 UTC
  copied those respective check epochs with both checks OK. Neither cycle
  was forced. This completed live acceptance on both frontends; the temporary
  localhost port-forward was then closed.
