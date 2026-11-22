# Grafana Cloud fit and cost for the f3s homelab (task 064)

Audit date: 2026-11-22. The numbers come from the live Prometheus
(`monitoring/prometheus-kube-prometheus-prometheus`), read through a
port-forward, plus published Grafana Cloud pricing. This only maps the setup
and estimates cost. Nothing was changed.

## What runs today and what Grafana Cloud would replace

| Today (f3s) | State | Grafana Cloud match |
|---|---|---|
| kube-prometheus-stack 93.1.0 (Prometheus 3.16, Alertmanager 0.34) | live, 10d retention, 10Gi PV | Cloud Metrics (Mimir), Cloud Alerting |
| Grafana | disabled (SQLite on NFS broke; old admin password leaked in history) | Hosted Grafana: removes the SQLite/NFS problem entirely |
| Loki + Alloy | disabled | Cloud Logs; Alloy would stay as the in-cluster collector |
| Tempo | disabled, no data | Cloud Traces |
| Pushgateway (Shelly TLS probes, etc.) | live | keep it local; Alloy scrapes it and remote-writes |
| Gogios (frontend checks) | live | Synthetic Monitoring could cover the HTTP checks, but Gogios runs outside the cluster on purpose, so keep it |
| Node exporters on r0-r2, f0-f3, pi0/pi1 | live | unchanged, scraped by Alloy |

## Current size

- **455,081 active head series** and **~15,300 samples/s**, across 43 targets.
- 23 jobs scrape every 30s and one every 10s.
- Series by job:

| Job | Series |
|---|---|
| kubelet | 152,918 |
| kube-controller-manager | 145,210 |
| apiserver | 116,169 |
| kube-state-metrics | 10,644 |
| node-exporter (9 hosts) | 8,440 |
| garage | 5,916 |
| kube-etcd | 5,893 |
| argocd-server | 2,378 |
| prometheus self | 2,191 |
| traefik | 1,278 |
| alertmanager | 767 |

Most of that is waste. k3s runs apiserver, controller-manager and kubelet
in one process, and every one of these endpoints serves the same metrics.
For example, the top `kube-controller-manager` metric names are
`etcd_request_duration_seconds_bucket` and
`apiserver_request_duration_seconds_bucket`, with identical counts in the
`apiserver` job. So about 400k series are histogram buckets collected three
times over. A follow-up task tracks fixing this locally.

## Published pricing (2026)

- **Free:** 10k active metric series, 50 GB logs, 50 GB traces, 3 users,
  14-day retention.
- **Pro:** $19/month. This includes the free allowances, then
  ~$6.50 per extra 1k active series. Logs are billed per GB processed plus
  written (sources quote ~$0.05 + $0.40/GB) and extra users cost $8 each.
- Verify on grafana.com/pricing before buying, because third-party
  summaries disagree on how log retention is billed.

## Cost scenarios (metrics dominate; logs and traces fit the free 50 GB)

| Scenario | Series shipped | Monthly cost |
|---|---|---|
| A. Remote-write everything as-is | ~455k | 19 + 445 × 6.50 ≈ **$2,900** |
| B. Drop duplicate k3s histograms, ship the rest | ~45-60k | ≈ **$250-350** |
| C. Curated set: node-exporter, kube-state-metrics (allowlisted), garage, traefik, pushgateway, alerts' inputs | ~25-30k | ≈ **$150-200** |
| D. Free tier: node-exporter, the Shelly/pushgateway alerts and a trimmed kube-state-metrics, via Alloy `keep` relabelling | ≤10k | **$0** |

Log volume is unknown because Loki is off. A homelab with about 40 pods
normally stays far below 50 GB/month, especially if Alloy drops debug lines,
so logs would most likely cost $0 under D as well.

## Recommendation

1. **A personal plan only makes sense as D (free tier)**, or at most C.
   Anything close to the current cardinality costs thousands per month.
2. The best use is **hosted Grafana plus off-site alerting**. That replaces
   the disabled local Grafana, whose SQLite on NFS failed and whose admin
   password is leaked. It also gives an alert path that still works when f3s
   is down, which covers part of the Gogios blind spot. Keep the local
   Prometheus as the full-fidelity store with 10d retention.
3. Do the local deduplication first either way. It shrinks the local TSDB
   too, and it makes C/D straightforward.
4. Tradeoffs: host metrics and logs leave the house (pick the EU region), the
   free tier keeps only 14 days, and the setup depends on Grafana Labs not
   changing the free allowances.

Sources: grafana.com/pricing (verify), last9.io/blog/grafana-cloud-pricing,
agentdeals.dev/vendor/grafana-cloud, cubeapm.com/blog/grafana-cloud-pricing-and-review.
