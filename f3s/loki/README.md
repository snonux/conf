# Loki

Log aggregation plus Grafana Alloy as a DaemonSet collecting all container
logs. Namespace `monitoring`. The ArgoCD app `argocd-apps/monitoring/loki.yaml`
is currently disabled (`.disabled`); `alloy.yaml` is active.

Volume: `/data/nfs/k3svolumes/loki/data`, owned 10001:10001.

Grafana datasource: type Loki, `http://loki.monitoring.svc.cluster.local:3100`.

```sh
just status | logs-loki | logs-alloy | port-forward-loki [3100]
just sync | argocd-status | restart-loki | restart-alloy | restart
```
