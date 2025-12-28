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
