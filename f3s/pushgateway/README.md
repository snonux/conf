# Pushgateway Helm Chart

Prometheus Pushgateway deployment for the f3s Kubernetes cluster.

## Overview

Pushgateway is an intermediary service that allows ephemeral and batch jobs to expose metrics to Prometheus. It receives metrics pushed to it via HTTP and exposes them for Prometheus to scrape.

## Installation

```bash
cd helm-chart
helm upgrade --install pushgateway . -n monitoring --create-namespace
```

## Configuration

Edit `values.yaml` to customize:

```yaml
replicaCount: 1              # Number of replicas
image:
  repository: prom/pushgateway
  tag: v1.10.0               # Pushgateway version
service:
  type: ClusterIP            # Service type
  port: 9091                 # Port number
resources:                   # Resource limits
  requests:
    memory: "64Mi"
    cpu: "100m"
```

## Usage

### Push Metrics

Push metrics to the Pushgateway:

```bash
# From inside the cluster
curl -X POST http://pushgateway.monitoring.svc.cluster.local:9091/metrics/job/some_job \
  --data-binary @metrics.txt

# From outside (with port-forward)
kubectl port-forward -n monitoring svc/pushgateway 9091:9091
curl -X POST http://localhost:9091/metrics/job/some_job \
  --data-binary @metrics.txt
```

### View Pushgateway UI

```bash
kubectl port-forward -n monitoring svc/pushgateway 9091:9091
# Open http://localhost:9091
```

### Delete Metrics

```bash
# Delete all metrics for a job
curl -X DELETE http://localhost:9091/metrics/job/some_job

# Delete metrics for a specific instance
curl -X DELETE http://localhost:9091/metrics/job/some_job/instance/some_instance
```

## Prometheus Configuration

Ensure Prometheus is configured to scrape the Pushgateway:

```yaml
- job_name: 'pushgateway'
  honor_labels: true
  static_configs:
    - targets:
      - 'pushgateway.monitoring.svc.cluster.local:9091'
```

**Important**: Use `honor_labels: true` to preserve job and instance labels from pushed metrics.

## Helm Commands

```bash
# Install
helm install pushgateway . -n monitoring

# Upgrade
helm upgrade pushgateway . -n monitoring

# Uninstall
helm uninstall pushgateway -n monitoring

# View values
helm get values pushgateway -n monitoring

# View status
helm status pushgateway -n monitoring
```

## Verification

```bash
# Check deployment
kubectl get pods -n monitoring -l app=pushgateway

# Check service
kubectl get svc -n monitoring pushgateway

# View logs
kubectl logs -n monitoring -l app=pushgateway

# Test metrics endpoint
kubectl port-forward -n monitoring svc/pushgateway 9091:9091
curl http://localhost:9091/metrics
```

## Use Cases

Pushgateway is designed for:

- **Batch jobs**: Jobs that run periodically and exit
- **Short-lived processes**: Processes that don't live long enough to be scraped
- **Service-level metrics**: Aggregated metrics from multiple instances
- **Firewall/NAT scenarios**: When Prometheus can't reach the target

**Not recommended for**:

- Long-running applications (use `/metrics` endpoint instead)
- High-cardinality metrics
- Real-time monitoring (introduces staleness)

## Metrics Format

Push metrics in Prometheus text format:

```
# HELP metric_name Description of the metric
# TYPE metric_name counter
metric_name{label1="value1",label2="value2"} 42
```

## Architecture

```
┌─────────────────┐
│  Applications   │
│  Batch Jobs     │──Push──┐
│  Scripts        │        │
└─────────────────┘        │
                           ▼
                 ┌──────────────────┐
                 │  Pushgateway     │◄──Scrape──┐
                 │  :9091           │           │
                 └──────────────────┘           │
                                       ┌────────────────┐
                                       │  Prometheus    │
                                       └────────────────┘
```

## Notes

- Pushgateway does NOT implement any authentication
- Consider network policies if exposing externally
- Metrics persist until explicitly deleted or Pushgateway restarts
- Use persistence if you need metrics to survive restarts
