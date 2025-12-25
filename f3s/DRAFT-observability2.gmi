# f3s: Kubernetes with FreeBSD - Part 9: Enabling etcd Metrics

## Introduction

This post covers enabling etcd metrics monitoring for the k3s cluster. The etcd dashboard in Grafana initially showed no data because k3s uses an embedded etcd that doesn't expose metrics by default.

=> ./2025-12-07-f3s-kubernetes-with-freebsd-part-8.html Part 8: Observability

## Enabling etcd metrics in k3s

On each control-plane node (r0, r1, r2), create /etc/rancher/k3s/config.yaml:

```
etcd-expose-metrics: true
```

Then restart k3s on each node:

```
systemctl restart k3s
```

After restarting, etcd metrics are available on port 2381:

```
curl http://127.0.0.1:2381/metrics | grep etcd
```

## Configuring Prometheus to scrape etcd

In persistence-values.yaml, enable kubeEtcd with the node IP addresses:

```
kubeEtcd:
  enabled: true
  endpoints:
    - 192.168.1.120
    - 192.168.1.121
    - 192.168.1.122
  service:
    enabled: true
    port: 2381
    targetPort: 2381
```

Apply the changes:

```
just upgrade
```

## Verifying etcd metrics

After the changes, all etcd targets are being scraped:

```
kubectl exec -n monitoring prometheus-prometheus-kube-prometheus-prometheus-0 \
  -c prometheus -- wget -qO- 'http://localhost:9090/api/v1/query?query=etcd_server_has_leader' | \
  jq -r '.data.result[] | "\(.metric.instance): \(.value[1])"'
```

Output:

```
192.168.1.120:2381: 1
192.168.1.121:2381: 1
192.168.1.122:2381: 1
```

The etcd dashboard in Grafana now displays metrics including Raft proposals, leader elections, and peer round trip times.

## Complete persistence-values.yaml

The complete updated persistence-values.yaml:

```
kubeEtcd:
  enabled: true
  endpoints:
    - 192.168.1.120
    - 192.168.1.121
    - 192.168.1.122
  service:
    enabled: true
    port: 2381
    targetPort: 2381

prometheus:
  prometheusSpec:
    additionalScrapeConfigsSecret:
      enabled: true
      name: additional-scrape-configs
      key: additional-scrape-configs.yaml
    storageSpec:
      volumeClaimTemplate:
        spec:
          storageClassName: ""
          accessModes: ["ReadWriteOnce"]
          resources:
            requests:
              storage: 10Gi
          selector:
            matchLabels:
              type: local
              app: prometheus

grafana:
  persistence:
    enabled: true
    type: pvc
    existingClaim: "grafana-data-pvc"

  initChownData:
    enabled: false

  podSecurityContext:
    fsGroup: 911
    runAsUser: 911
    runAsGroup: 911
```

## Summary

Enabled etcd metrics monitoring for the k3s embedded etcd by:

* Adding etcd-expose-metrics: true to /etc/rancher/k3s/config.yaml on each control-plane node
* Configuring Prometheus to scrape etcd on port 2381

The etcd dashboard now provides visibility into cluster health, leader elections, and Raft consensus metrics.

=> https://codeberg.org/snonux/conf/src/branch/master/f3s/prometheus prometheus configuration on Codeberg
