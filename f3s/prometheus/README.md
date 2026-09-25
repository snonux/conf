# Prometheus stack

kube-prometheus-stack 55.5.0, release `prometheus`, namespace `monitoring`,
deployed by ArgoCD from `argocd-apps/monitoring/prometheus.yaml`. That
Application has two sources: the upstream chart with inline
`valuesObject`, and `f3s/prometheus/manifests/` (rules, dashboards,
scrape-config secret, Prometheus PV, NodePort). ArgoCD applies only
`manifests/`; the YAML files at this directory's top level aren't synced.

Current values worth knowing:

- Grafana is disabled (`grafana.enabled: false`): SQLite on NFS didn't
  survive restarts, and Loki/Tempo are off too. The Grafana settings stay in
  the values for when it comes back (UID 911, datasources from ConfigMap
  `grafana-datasources-all` mounted at `/etc/grafana/provisioning/datasources`,
  sidecar off). Its NFS PV/PVC (`grafana-data-pv`/`-pvc`) and the PostSync
  Grafana restart hook were removed (audit 2026-09-25 #16); the old data
  stays in `/data/nfs/k3svolumes/grafana/data`. Re-enabling needs a new PVC
  (preferably local-path, not SQLite on NFS) for `existingClaim`.
- etcd and controller-manager scraped at 192.168.2.120-122 (ports 2381,
  10257). kube-proxy and kube-scheduler are off (embedded in k3s).
- node-exporter reads the textfile collector at
  `/var/lib/node_exporter/textfile_collector` (NFS mount monitor metrics).
- Extra scrape jobs from secret `additional-scrape-configs`: node-exporter
  (FreeBSD/OpenBSD hosts), garage, pushgateway, radicale-drop,
  kubernetes-services-no-radicale.
- Storage: 10Gi PV selected by labels `type: local`, `app: prometheus`.
- NodePort 30090 (`prometheus-nodeport`).
- Alertmanager routes: `component=argocd`, `component=nfs` (30m repeat),
  `component=trivy`; Watchdog to `null`. All receivers are UI-only (no
  mail/webhook).

## Operate

```sh
just status
just alerts                  # port-forward Prometheus to :9090
just alertmanager            # port-forward Alertmanager to :9093
just firing-alerts [node]    # via NodePort 30090, default node r0
just firing-count [node]
just all-alerts [node]
just logs-prometheus
just sync; just argocd-status
just restart-prometheus; just restart-alertmanager
```

## Backfilling (test data)

Enabled on purpose for ad-hoc testing (e.g. Epimetheus): remote-write
receiver (`/api/v1/write`), admin API, features `exemplar-storage` and
`otlp-write-receiver`, `tsdb.outOfOrderTimeWindow: 744h` (31 days). Costs
memory, extra compactions and slower queries; not a production setting.

Clean up after tests:

```sh
kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090
curl -X POST 'http://localhost:9090/api/v1/admin/tsdb/delete_series?match[]=metric_name'
curl -X POST 'http://localhost:9090/api/v1/admin/tsdb/clean_tombstones'
```

Epimetheus's `cleanup-benchmark-data.sh` deletes all
`epimetheus_benchmark_*` series.

The Grafana datasource debugging record is in
[`docs/archive/f3s/prometheus/problem.md`](../../docs/archive/f3s/prometheus/problem.md).
