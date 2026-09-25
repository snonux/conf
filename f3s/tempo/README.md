# Tempo

Grafana Tempo, monolithic mode, namespace `monitoring`. The ArgoCD app
`argocd-apps/monitoring/tempo.yaml` is currently disabled (`.disabled`), and
so are Grafana and Alloy, so nothing is collecting or showing traces right now.
The PV/PVC and the old local-path `storage-tempo-0` were deleted from the
cluster (audit 2026-09-25 #16); `/data/nfs/k3svolumes/tempo/data` does not
exist either, so create it and `kubectl apply -f persistent-volumes.yaml`
before re-enabling.

| | |
|---|---|
| OTLP receivers | gRPC 4317, HTTP 4318 |
| Query | port 3200 |
| Storage | filesystem `/var/tempo/traces`; PV `tempo-data-pv` at `/data/nfs/k3svolumes/tempo/data`, PVC `tempo-data-pvc` 10Gi |
| Retention | 168h |
| Limits | 2Gi memory, 1 CPU (`values.yaml`) |
| Grafana datasource | `datasource-configmap.yaml` (`tempo-grafana-datasource`, label `grafana_datasource: "1"`), traces-to-logs (Loki), traces-to-metrics, service graph |

Flow: apps send OTLP to Alloy (`alloy.monitoring.svc.cluster.local:4317` /
`:4318`), Alloy forwards to `tempo.monitoring.svc.cluster.local:4317`.

```sh
just status | logs [lines] | check      # check: readiness + OTLP ports
just port-forward [3200] | port-forward-otlp-grpc [4317] | port-forward-otlp-http [4318]
just sync | argocd-status | restart
kubectl exec -n monitoring $(kubectl get pod -n monitoring -l app.kubernetes.io/name=tempo -o jsonpath='{.items[0].metadata.name}') -- df -h /var/tempo
kubectl logs -n monitoring -l app.kubernetes.io/name=alloy | grep -i tempo
```

TraceQL examples:

```
{ resource.service.namespace = "tracing-demo" }
{ resource.service.name = "frontend" }
{ duration > 200ms }
{ status = error }
```
