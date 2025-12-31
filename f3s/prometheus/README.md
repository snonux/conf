# Prometheus Stack Configuration

## Deploying

```bash
just install  # First time
just upgrade  # Updates
```

**IMPORTANT**: After upgrading, Grafana will automatically restart to load new configurations.

## Datasources

All Grafana datasources are provisioned via a single unified ConfigMap:
- `grafana-datasources-all.yaml` - Contains Prometheus, Alertmanager, Loki, and Tempo

This ConfigMap is directly mounted to `/etc/grafana/provisioning/datasources/` in the Grafana pod, ensuring datasources are automatically loaded on startup.

**Provisioned Datasources:**
- ✅ **Prometheus** (uid=prometheus) - Default datasource for metrics
- ✅ **Alertmanager** (uid=alertmanager) - Alert management
- ✅ **Loki** (uid=loki) - Log aggregation
- ✅ **Tempo** (uid=tempo) - Distributed tracing with traces-to-logs and traces-to-metrics correlation

**Note:** The sidecar-based provisioning is disabled in favor of direct ConfigMap mounting (following the pattern from /home/paul/git/x-rag/infra/k8s/monitoring/). See `problem.md` for the complete debugging journey and resolution.

## Historic Data Ingestion

Prometheus is configured to accept historic data with custom timestamps via the Remote Write API. This enables backfilling test data for ad-hoc troubleshooting and development purposes.

### Configuration

The following features are enabled in `persistence-values.yaml`:

```yaml
prometheus:
  prometheusSpec:
    # Enable Remote Write receiver endpoint and Admin API
    additionalArgs:
      - name: web.enable-remote-write-receiver
        value: ""
      - name: web.enable-admin-api
        value: ""

    # Enable out-of-order ingestion for backfilling
    enableFeatures:
      - exemplar-storage
      - otlp-write-receiver

    # Allow backfilling up to 31 days in the past (provides 1-day buffer for 30-day datasets)
    tsdb:
      outOfOrderTimeWindow: 744h  # 31 days
```

### What This Enables

- **Remote Write API**: HTTP endpoint at `/api/v1/write` for ingesting metrics with custom timestamps
- **Admin API**: HTTP endpoints at `/api/v1/admin/tsdb/*` for data deletion and management
- **Out-of-Order Ingestion**: Allows writing data points older than existing data for the same time series
- **31-Day Window**: Can backfill data up to 31 days in the past (configured via `outOfOrderTimeWindow`, provides 1-day buffer for 30-day datasets)

### Use Cases

This configuration is designed for:
- **Testing**: Populating Grafana dashboards with synthetic historic data
- **Development**: Simulating various time-series scenarios
- **Troubleshooting**: Backfilling gaps in metric collection

Example: The [Epimetheus](https://github.com/pbuetow/epimetheus) tool uses this to push test metrics with historic timestamps.

### Data Deletion

The Admin API enables selective deletion of time series data for cleanup after testing:

**Delete specific metrics:**
```bash
curl -X POST 'http://localhost:9090/api/v1/admin/tsdb/delete_series?match[]=metric_name'
```

**Clean up tombstones** (free disk space):
```bash
curl -X POST 'http://localhost:9090/api/v1/admin/tsdb/clean_tombstones'
```

**Using the cleanup script:**

The [Epimetheus repository](https://github.com/pbuetow/epimetheus) includes `cleanup-benchmark-data.sh` which automates deletion of all benchmark metrics:
```bash
# Requires port-forward to Prometheus
kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090

# Run cleanup
./cleanup-benchmark-data.sh
```

The script deletes all `epimetheus_benchmark_*` metrics and cleans up tombstones automatically.

### Performance Considerations

**Important**: This is NOT a production-ready configuration. Enabling these features has trade-offs:

- **Increased Memory Usage**: Out-of-order ingestion requires additional memory for buffering and sorting time series
- **Higher TSDB Overhead**: Prometheus TSDB needs to handle non-sequential writes, increasing disk I/O
- **Query Performance**: Queries may be slower due to fragmented data blocks
- **Storage Amplification**: Out-of-order samples can trigger additional compactions, increasing storage usage

**Recommendation**: For production environments:
- Keep `outOfOrderTimeWindow` as small as possible (or disabled)
- Monitor Prometheus memory and disk usage closely
- Use Remote Write only when necessary
- Consider using dedicated testing/development Prometheus instances

**Note**: This setup is optimized for ad-hoc troubleshooting and development workflows, not for production monitoring at scale.
