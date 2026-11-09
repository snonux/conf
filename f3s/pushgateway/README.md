# Pushgateway

`prom/pushgateway:v1.11.5` (pinned by digest in `values.yaml`), ClusterIP
service `pushgateway:9091` in `monitoring` plus the Local NodePort
`pushgateway-lan` (30091, see Writers),
ArgoCD app `pushgateway` (chart in `helm-chart/`). No auth, no persistence:
metrics live until deleted or the pod restarts. Prometheus scrapes it
through the `pushgateway` job in `additional-scrape-configs` with
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

## Writers

The only production writer is the Shelly certificate probe on pi0 and pi1
(task s44, gonf `pis_netbsd_shelly_tls_*`, script
`gonf/netbsd/assets/shelly-tls-probe`). Each Pi PUTs its group
`job=shelly_tls`, `reporter=pi0|pi1` hourly to the Local NodePort
**`pushgateway-lan` 30091** on `10.20.0.120`–`.122`; only the node running
the pod answers, the probe tries all three. `externalTrafficPolicy: Local`
keeps the Pi's source, so NetworkPolicy `pushgateway-scrape` admits exactly
`10.20.0.125/32` and `10.20.0.126/32` on TCP 9091, next to the Prometheus
scrape. No node/CNI address is admitted, so ordinary pods cannot reach it
through the NodePort. The Pis share `10.20.0.0/24` with the r-nodes (no
router rule), and the r-node `f3s_guard` already trusts that subnet. The
Pushgateway has no auth: anything those two sources send is accepted,
including deletes of other groups. And because the scrape job uses
`honor_labels: true`, a writer controls the `job` and `instance` labels of
what it pushes: pi0/pi1 could inject series that look like any other job.
Only these two controller hosts are admitted for that reason.

The NetworkPolicy rule went live by `kubectl apply` (task s44). The manual
Argo Application `network-policies-monitoring` pins `1fc3dfa` since task
e54, a commit that contains the rule, and is Synced. Never pin it to an
older commit (such as `a1112b9`): a sync would remove the Pi rule and stop
every push.

The alerts are `f3s/prometheus/manifests/shelly-tls-alerts.yaml`. Without
persistence, every Pushgateway restart (each cluster power-on) starts empty
until the next hourly push; `ShellyTLSProbeAbsent` waits 3h for that.

```sh
# from pi0/pi1 only (the policy admits no other source)
curl -s http://10.20.0.122:30091/api/v1/metrics   # whichever node has the pod
```

```sh
just status | logs [lines] | port-forward [9091] | sync | argocd-status | restart
kubectl get pods -n monitoring -l app=pushgateway
```
