# Grafana Loki

Log aggregation system for your k3s cluster.

Includes Grafana Alloy as a DaemonSet to collect logs from all containers in all namespaces.

## Prerequisites

Create the data directory on your host:

```bash
sudo mkdir -p /data/nfs/k3svolumes/loki/data
sudo chown 10001:10001 /data/nfs/k3svolumes/loki/data
```

## Install

```bash
just install
```

## Configure Grafana

Add Loki as a data source in Grafana:
- Type: Loki
- URL: `http://loki.monitoring.svc.cluster.local:3100`
