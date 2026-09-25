# Loki

Log aggregation plus Grafana Alloy as a DaemonSet collecting all container
logs. Namespace `monitoring`. The ArgoCD app `argocd-apps/monitoring/loki.yaml`
is disabled (`.disabled`), and so is `alloy.yaml` (audit 2026-09-25 #16).

Volume: `/data/nfs/k3svolumes/loki/data`, owned 10001:10001. The PV/PVC were
deleted from the cluster (the data stays on NFS); `kubectl apply -f
persistent-volumes.yaml` before re-enabling.

Grafana datasource: type Loki, `http://loki.monitoring.svc.cluster.local:3100`.

```sh
just status | logs-loki | logs-alloy | port-forward-loki [3100]
just sync | argocd-status | restart-loki | restart-alloy | restart
```
