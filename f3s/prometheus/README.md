# Prometheus stack

kube-prometheus-stack 93.1.0, release `prometheus`, namespace `monitoring`,
deployed by ArgoCD from `argocd-apps/monitoring/prometheus.yaml`. That
Application has two sources: the upstream chart with inline
`valuesObject`, and `f3s/prometheus/manifests/` (rules, dashboards,
scrape-config secret, Prometheus PV, NodePort). ArgoCD applies only
`manifests/`; the YAML files at this directory's top level aren't synced.

Verified 2026-11-08: Prometheus 3.16.0, Alertmanager 0.34.2 and operator /
config-reloader 0.95.1. All 43 scrape targets remain UP with the same job
coverage. The 92 → 93 upgrade applied the ten monitoring CRDs first;
ArgoCD then reconciled the chart and admission hooks. Existing network
policies, Service specs, storage and disabled components were preserved.
The old Helm release record is stale; manage this stack through its ArgoCD
Application.

The chart source uses ArgoCD's native OCI support at
`oci://ghcr.io/prometheus-community/charts/kube-prometheus-stack`, with
`path: .` and tag `93.1.0`. The publisher's OCI archive was verified
byte-for-byte against the HTTP chart (SHA-256
`9d727ffda3995720dd66a46c94ee6f18eecf1c8fe560ec47824f9335b05145d2`).
This replaced intermittent HTTP 504s from GitHub's chart release-asset URL.
ArgoCD compared the OCI manifest digest successfully, without restarting
monitoring pods; no repository credential is needed for this public source.

Before the upgrade, private resource/CRD backups and an r0 etcd snapshot
were saved, and f0's atomic filesystem snapshot
`zdata/enc/nfsdata@j34-before-prometheus-93-20261108` was held with tag
`j34-prometheus-upgrade`. It is crash-consistent, not a quiesced TSDB
backup. Recovery must restore only the affected application's directories
with its writers stopped; never roll back the shared NFS dataset.

Current values worth knowing:

- Grafana is disabled (`grafana.enabled: false`): SQLite on NFS didn't
  survive restarts, and Loki/Tempo are off too. The Grafana settings stay in
  the values for when it comes back (UID 911, datasources from ConfigMap
  `grafana-datasources-all` mounted at `/etc/grafana/provisioning/datasources`,
  sidecar off). Its NFS PV/PVC (`grafana-data-pv`/`-pvc`) and the PostSync
  Grafana restart hook were removed (audit 2026-09-25 #16); the old data
  stays in `/data/nfs/k3svolumes/grafana/data`. Re-enabling needs a new PVC
  (preferably local-path, not SQLite on NFS) for `existingClaim`. Do not
  reuse the old `grafana.db`, or reset the admin password before exposing
  it: its admin password is the one leaked in public history (`a8341c5`).
- etcd and controller-manager scraped at 192.168.2.120-122 (ports 2381,
  10257). kube-proxy and kube-scheduler are off (embedded in k3s).
- node-exporter reads the textfile collector at
  `/var/lib/node_exporter/textfile_collector` (NFS mount monitor metrics).
- Extra scrape jobs from secret `additional-scrape-configs`: node-exporter
  (FreeBSD/OpenBSD hosts), garage, pushgateway, radicale-drop,
  kubernetes-services-no-radicale.
- Storage: 10Gi PV selected by labels `type: local`, `app: prometheus`.
- NodePort 30090 (`prometheus-nodeport`) selects a read-only nginx proxy
  DaemonSet on r0/r1/r2. Only Gogios's frontend WireGuard sources and earth's
  WireGuard source may connect. Direct Prometheus stays on ClusterIP;
  its ingress policy allows the proxy and its own metric scrape.
- Alertmanager routes: `component=argocd`, `component=nfs` (30m repeat),
  `component=trivy`; Watchdog to `null`. All receivers are UI-only (no
  mail/webhook).

## Operate

Task **jn3** prepares [upgrade maintenance monitoring](../docs/upgrade-maintenance.md)
for k3s release staleness and FreeBSD pending-kernel reboots. Its scripts,
crons and alert rules await review and approved deployment.

```sh
just status
just alerts                  # port-forward Prometheus to :9090
just alertmanager            # port-forward Alertmanager to :9093
just firing-alerts [node]    # read proxy over WG; default r0.wg0.wan.buetow.org
just firing-count [node]
just all-alerts [node]
just logs-prometheus
just sync; just argocd-status
just restart-prometheus; just restart-alertmanager
```

## Backfilling (test data)

Epimetheus historic imports still use the remote-write receiver
(`/api/v1/write`) and `tsdb.outOfOrderTimeWindow: 744h` (31 days).
Keep `exemplar-storage`. The admin API and unused OTLP receiver are disabled.
The proxy never forwards write/admin/lifecycle routes, even if an operator
temporarily enables one in Prometheus. Use your RBAC-authorized Kubernetes
context and a localhost-only port forward for imports:

Clean up after tests:

```sh
kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090
# In another terminal, point Epimetheus historic mode at:
# http://127.0.0.1:9090/api/v1/write
```

Ad-hoc destructive cleanup requires separate approval and a temporary,
reviewed `enableAdminAPI: true` change in the ArgoCD values (never another
`additionalArgs` flag). Keep both policies and the proxy in place, use the
localhost port forward, then revert to `false` and verify the disabled API
after reconciliation. Do not run Epimetheus's `cleanup-benchmark-data.sh`
against production without that approval: it deletes every
`epimetheus_benchmark_*` series.

See [exposure controls and deployment acceptance](exposure.md) for the client
inventory, trusted-node limits, local checks, and staged rollout.

The Grafana datasource debugging record is in
[`docs/archive/f3s/prometheus/problem.md`](../../docs/archive/f3s/prometheus/problem.md).
