# Pushgateway

`prom/pushgateway:v1.10.0`, ClusterIP service `pushgateway:9091` in
`monitoring`, ArgoCD app `pushgateway` (chart in `helm-chart/`). No auth, no
persistence: metrics live until deleted or the pod restarts. Prometheus
scrapes it through the `pushgateway` job in `additional-scrape-configs` with
`honor_labels: true` (keeps pushed `job`/`instance`).

```sh
# in cluster
curl -X POST --data-binary @metrics.txt \
  http://pushgateway.monitoring.svc.cluster.local:9091/metrics/job/<job>
# from outside
kubectl port-forward -n monitoring svc/pushgateway 9091:9091
curl -X POST --data-binary @metrics.txt http://localhost:9091/metrics/job/<job>
curl -X DELETE http://localhost:9091/metrics/job/<job>
curl -X DELETE http://localhost:9091/metrics/job/<job>/instance/<instance>
```

Body is Prometheus text format. Use it for batch and short-lived jobs only.

```sh
just status | logs [lines] | port-forward [9091] | sync | argocd-status | restart
kubectl get pods -n monitoring -l app=pushgateway
```
