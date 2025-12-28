# Grafana Tempo - Distributed Tracing

Grafana Tempo deployment for the f3s Kubernetes cluster in monolithic mode.

## Overview

- **Deployment Mode**: Monolithic (all components in one process)
- **Storage Backend**: Filesystem (local storage on hostPath)
- **Storage Size**: 10Gi
- **Retention**: 7 days (168h)
- **Namespace**: `monitoring`

## Components

- **Tempo**: Distributed tracing backend
- **OTLP Receivers**: Accepts traces via gRPC (4317) and HTTP (4318)
- **Query Frontend**: Query interface on port 3200
- **Grafana Datasource**: Auto-discovered via ConfigMap label

## Architecture

```
Applications → Alloy (OTLP collector) → Tempo → Grafana
```

## Installation

```bash
just install
```

This will:
1. Add Grafana Helm repo and update
2. Create PersistentVolume and PersistentVolumeClaim
3. Install Tempo via Helm
4. Create Grafana datasource ConfigMap

## Configuration

### values.yaml

- Monolithic mode configuration
- OTLP receivers on ports 4317 (gRPC) and 4318 (HTTP)
- Local filesystem storage at `/var/tempo/traces`
- Resource limits: 2Gi memory, 1 CPU

### persistent-volumes.yaml

- PV: `tempo-data-pv` at `/data/nfs/k3svolumes/tempo/data`
- PVC: `tempo-data-pvc` (10Gi, ReadWriteOnce)

### datasource-configmap.yaml

- Auto-discovered by Grafana sidecar
- Enables traces-to-logs correlation with Loki
- Enables traces-to-metrics correlation with Prometheus
- Enables service graph visualization

## Grafana Integration

The datasource is automatically discovered by Grafana through the ConfigMap with label `grafana_datasource: "1"`.

To access traces in Grafana:
1. Navigate to Explore
2. Select "Tempo" datasource
3. Use Search or TraceQL queries

### Example TraceQL Queries

```
# Find all traces from demo app
{ resource.service.namespace = "tracing-demo" }

# Find slow requests (>200ms)
{ duration > 200ms }

# Find errors
{ status = error }

# Find traces from specific service
{ resource.service.name = "frontend" }
```

## Verification

Check that Tempo is running:
```bash
just status
```

Check Tempo readiness and OTLP ports:
```bash
just check
```

View logs:
```bash
just logs
```

## Sending Traces

Applications should send traces to Alloy's OTLP receivers:
- gRPC: `alloy.monitoring.svc.cluster.local:4317`
- HTTP: `alloy.monitoring.svc.cluster.local:4318`

Alloy forwards traces to Tempo at `tempo.monitoring.svc.cluster.local:4317`.

## Maintenance

### Upgrade

```bash
just upgrade
```

### Uninstall

```bash
just uninstall
```

### Check Storage Usage

```bash
kubectl exec -n monitoring $(kubectl get pod -n monitoring -l app.kubernetes.io/name=tempo -o jsonpath='{.items[0].metadata.name}') -- df -h /var/tempo
```

## Troubleshooting

### Tempo pod not starting

Check events:
```bash
kubectl describe pod -n monitoring -l app.kubernetes.io/name=tempo
```

Check PVC binding:
```bash
kubectl get pvc -n monitoring tempo-data-pvc
```

### No traces appearing

1. Verify Alloy is forwarding traces:
```bash
kubectl logs -n monitoring -l app.kubernetes.io/name=alloy | grep -i tempo
```

2. Check Tempo logs:
```bash
just logs
```

3. Verify OTLP receivers are listening:
```bash
just check
```

### Grafana datasource not appearing

1. Check ConfigMap exists:
```bash
kubectl get cm -n monitoring tempo-grafana-datasource --show-labels
```

2. Check Grafana sidecar logs:
```bash
kubectl logs -n monitoring $(kubectl get pod -n monitoring -l app.kubernetes.io/name=grafana -o jsonpath='{.items[0].metadata.name}') -c grafana-sc-datasources
```

3. Restart Grafana pod if needed:
```bash
kubectl delete pod -n monitoring -l app.kubernetes.io/name=grafana
```

## References

- [Grafana Tempo Documentation](https://grafana.com/docs/tempo/latest/)
- [Tempo Helm Chart](https://github.com/grafana/helm-charts/tree/main/charts/tempo)
- [OpenTelemetry Protocol (OTLP)](https://opentelemetry.io/docs/specs/otlp/)
- [TraceQL Query Language](https://grafana.com/docs/tempo/latest/traceql/)
