# Prometheus Pusher

A versatile Go tool for pushing metrics to Prometheus with support for both realtime and historic data ingestion.

## Overview

**prometheus-pusher** is a standalone binary that:
- **Generates** realistic example metrics simulating production applications
- **Pushes** metrics via Pushgateway (realtime) or Remote Write API (historic)
- **Automatically detects** timestamp age and chooses the optimal ingestion method
- **Supports** multiple data formats (CSV, JSON) and all Prometheus metric types
- **Provides** Grafana dashboard for visualizing test metrics

## Quick Start

### 1. Deploy Pushgateway (one-time setup)

```bash
cd /home/paul/git/conf/f3s/pushgateway/helm-chart
helm upgrade --install pushgateway . -n monitoring --create-namespace
```

### 2. Run in Realtime Mode

```bash
# Port-forward Pushgateway
kubectl port-forward -n monitoring svc/pushgateway 9091:9091 &

# Push test metrics continuously
cd /home/paul/git/conf/f3s/prometheus-pusher
./prometheus-pusher -mode=realtime -continuous
```

The binary pushes metrics every 15 seconds. Press Ctrl+C to stop.

### 3. View Metrics

```bash
# Pushgateway UI
open http://localhost:9091

# Prometheus UI
kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090 &
open http://localhost:9090
```

## Operating Modes

### 🔄 Realtime Mode (Default)
Push current metrics to Pushgateway with "now" timestamp.

```bash
./prometheus-pusher -mode=realtime -continuous
```

**Options:**
- `-pushgateway` - Pushgateway URL (default: http://localhost:9091)
- `-job` - Job name (default: example_metrics_pusher)
- `-continuous` - Keep pushing every 15 seconds

### ⏰ Historic Mode
Push a single datapoint from the past using Remote Write API.

```bash
# Port-forward Prometheus
kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090 &

# Push data from 24 hours ago
./prometheus-pusher -mode=historic -hours-ago=24
```

**Options:**
- `-prometheus` - Prometheus URL (default: http://localhost:9090/api/v1/write)
- `-hours-ago` - Hours in the past (default: 24)

### 📦 Backfill Mode
Import a range of historic data points.

```bash
# Backfill last 48 hours with 1-hour intervals
./prometheus-pusher -mode=backfill -start-hours=48 -end-hours=0 -interval=1

# Backfill last week with 6-hour intervals
./prometheus-pusher -mode=backfill -start-hours=168 -end-hours=0 -interval=6
```

**Options:**
- `-start-hours` - Start time in hours ago
- `-end-hours` - End time in hours ago (0 = now)
- `-interval` - Interval between points in hours

### 🤖 Auto Mode (Recommended!)
Automatically detect timestamp age and route to the correct ingestion method.

```bash
# Generate test data
./generate-test-data.sh

# Import mixed current and historic data
./prometheus-pusher -mode=auto -file=test-all-ages.csv
```

**Detection Logic:**
- Data < 5 minutes old → Pushgateway (realtime)
- Data ≥ 5 minutes old → Remote Write (historic)

**Options:**
- `-file` - Input file path
- `-format` - Data format: csv or json (default: csv)
- `-pushgateway` - Pushgateway URL
- `-prometheus` - Prometheus Remote Write URL

## Data Formats

### CSV Format

```csv
# Format: metric_name,labels,value,timestamp_ms
# Labels: key1=value1;key2=value2
prometheus_pusher_test_requests_total,instance=web1;env=prod,100,1767125148000
prometheus_pusher_test_temperature_celsius,instance=web2,22.5,1767038748000

# Timestamp is optional (uses "now" if omitted)
prometheus_pusher_test_active_connections,instance=web3,42,
```

### JSON Format

```json
[
  {
    "metric": "prometheus_pusher_test_requests_total",
    "labels": {"instance": "web1", "env": "prod"},
    "value": 100,
    "timestamp_ms": 1767125148000
  },
  {
    "metric": "prometheus_pusher_test_temperature_celsius",
    "labels": {"instance": "web2"},
    "value": 22.5,
    "timestamp_ms": 1767038748000
  }
]
```

## Test Metrics

All generated metrics use the `prometheus_pusher_test_` prefix to clearly identify them as test data.

### Counter: `prometheus_pusher_test_requests_total`
- **Type:** Counter (monotonically increasing)
- **Description:** Total number of requests processed
- **Use case:** Counting total events, requests, errors

### Gauge: `prometheus_pusher_test_active_connections`
- **Type:** Gauge (can increase or decrease)
- **Description:** Current number of active connections (0-100)
- **Use case:** Current state measurements, capacity

### Gauge: `prometheus_pusher_test_temperature_celsius`
- **Type:** Gauge
- **Description:** Current temperature in Celsius (0-50°C)
- **Use case:** Environmental monitoring

### Histogram: `prometheus_pusher_test_request_duration_seconds`
- **Type:** Histogram (distribution)
- **Description:** Request duration distribution
- **Buckets:** 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10 seconds
- **Use case:** Latency measurements, SLO tracking

### Labeled Counter: `prometheus_pusher_test_jobs_processed_total`
- **Type:** Counter with labels
- **Description:** Jobs processed by type and status
- **Labels:**
  - `job_type`: email, report, backup
  - `status`: success, failed
- **Use case:** Categorized counting, multi-dimensional metrics

## Grafana Dashboard

A comprehensive dashboard is available showcasing all test metrics.

### Dashboard Features

- **8 Panels:**
  1. Request Rate (line graph)
  2. Total Requests (stat panel)
  3. Active Connections (gauge with thresholds)
  4. Temperature (gauge with thresholds)
  5. Request Duration Histogram (p50, p90, p99)
  6. Average Request Duration (stat)
  7. Jobs Processed by Type (bar gauge)
  8. Jobs Status Breakdown (table)

- **Auto-refresh:** Every 10 seconds
- **Time range:** Last 15 minutes (customizable)
- **Dark theme optimized**

### Deploy Dashboard

#### Option 1: Helm/Kubernetes ConfigMap (Recommended)

```bash
# Deploy via Kubernetes ConfigMap
kubectl apply -f ../prometheus/prometheus-pusher-dashboard.yaml
```

The dashboard will be automatically discovered by Grafana.

#### Option 2: Manual Import

```bash
# Port-forward Grafana
kubectl port-forward -n monitoring svc/prometheus-grafana 3000:80

# Open Grafana
open http://localhost:3000

# Go to Dashboards → Import → Upload grafana-dashboard.json
```

#### Option 3: Automated Script

```bash
# Deploy via API
./deploy-dashboard.sh

# Or with custom credentials
GRAFANA_URL="http://localhost:3000" \
GRAFANA_USER="admin" \
GRAFANA_PASSWORD="yourpassword" \
./deploy-dashboard.sh
```

## Example Queries

### Basic Queries

```promql
# View total requests
prometheus_pusher_test_requests_total

# View request rate over last 5 minutes
rate(prometheus_pusher_test_requests_total[5m])

# View current active connections
prometheus_pusher_test_active_connections

# View current temperature
prometheus_pusher_test_temperature_celsius
```

### Histogram Queries

```promql
# 95th percentile request duration
histogram_quantile(0.95, rate(prometheus_pusher_test_request_duration_seconds_bucket[5m]))

# 50th percentile (median)
histogram_quantile(0.50, rate(prometheus_pusher_test_request_duration_seconds_bucket[5m]))

# Average request duration
rate(prometheus_pusher_test_request_duration_seconds_sum[5m]) /
rate(prometheus_pusher_test_request_duration_seconds_count[5m])
```

### Labeled Counter Queries

```promql
# Failed jobs by type
prometheus_pusher_test_jobs_processed_total{status="failed"}

# Job success rate
rate(prometheus_pusher_test_jobs_processed_total{status="success"}[5m]) /
rate(prometheus_pusher_test_jobs_processed_total[5m])

# Total jobs by type
sum by (job_type) (prometheus_pusher_test_jobs_processed_total)
```

### Curl Examples

```bash
# Port-forward Prometheus
kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090 &

# Query total requests
curl -s "http://localhost:9090/api/v1/query?query=prometheus_pusher_test_requests_total" | jq .

# Query temperature
curl -s "http://localhost:9090/api/v1/query?query=prometheus_pusher_test_temperature_celsius" | jq .

# Query request rate
curl -s "http://localhost:9090/api/v1/query?query=rate(prometheus_pusher_test_requests_total[5m])" | jq .

# Query histogram p95
curl -s "http://localhost:9090/api/v1/query?query=histogram_quantile(0.95,rate(prometheus_pusher_test_request_duration_seconds_bucket[5m]))" | jq .
```

## Time Range Limitations

### ✅ Supported Time Ranges

| Time Range | Status | Method |
|------------|--------|--------|
| Current (< 5 min) | ✅ Works | Pushgateway |
| 1 hour old | ✅ Works | Remote Write |
| 1 day old | ✅ Works | Remote Write |
| 1 week old | ✅ Works | Remote Write |
| 1 month old | ✅ Works | Remote Write |

### ⚠️ Potential Issues

- **Future timestamps:** Rejected (> 5 minutes in future)
- **Very old data (6+ months):** May be rejected depending on Prometheus retention
- **Years old:** Likely rejected - use `promtool tsdb create-blocks-from` instead
- **Out-of-order samples:** Can't insert older data into existing time series (use different labels)

### Prometheus Configuration

Check your retention settings:

```bash
# View retention
kubectl get prometheus -n monitoring prometheus-kube-prometheus-prometheus \
  -o jsonpath='{.spec.retention}'

# Default is typically 15 days
```

For very old data:
- Increase retention in Prometheus config
- Enable out-of-order ingestion (experimental)
- Use `promtool` for direct TSDB block creation

## Project Structure

```
prometheus-pusher/
├── cmd/
│   └── prometheus-pusher/
│       └── main.go              # Main entry point
├── internal/
│   ├── config/                  # Configuration
│   ├── metrics/                 # Metric generators
│   ├── parser/                  # CSV/JSON parsers
│   └── ingester/                # Pushgateway & Remote Write ingesters
├── prometheus-pusher            # Compiled binary
├── grafana-dashboard.json       # Grafana dashboard definition
├── deploy-dashboard.sh          # Dashboard deployment script
├── generate-test-data.sh        # Test data generator
├── run.sh                       # Helper script
└── README.md                    # This file
```

## Setup Requirements

### 1. Enable Prometheus Remote Write Receiver

For historic data ingestion, Prometheus needs the remote write receiver enabled:

```yaml
# In prometheus/persistence-values.yaml
prometheus:
  prometheusSpec:
    enableFeatures:
      - remote-write-receiver
```

### 2. Update Prometheus Scrape Config

Ensure Pushgateway is in scrape targets:

```yaml
# additional-scrape-configs.yaml
- job_name: 'pushgateway'
  honor_labels: true
  static_configs:
    - targets:
      - 'pushgateway.monitoring.svc.cluster.local:9091'
```

Apply the configuration:

```bash
kubectl create secret generic additional-scrape-configs \
  --from-file=/home/paul/git/conf/f3s/prometheus/additional-scrape-configs.yaml \
  --dry-run=client -o yaml -n monitoring | kubectl apply -f -
```

## Building from Source

```bash
# Build binary
go build -o prometheus-pusher cmd/prometheus-pusher/main.go

# Run tests
go test ./... -v

# Check test coverage
go test ./... -cover
```

## Troubleshooting

### Binary can't connect to Pushgateway

```bash
# Check port-forward is running
ps aux | grep "port-forward.*9091"

# Restart port-forward
kubectl port-forward -n monitoring svc/pushgateway 9091:9091
```

### Metrics not appearing in Prometheus

```bash
# Check Pushgateway has metrics
curl http://localhost:9091/metrics | grep "prometheus_pusher_test"

# Check Prometheus scrape targets
# Open http://localhost:9090/targets - look for "pushgateway" job

# Check Prometheus logs
kubectl logs -n monitoring -l app.kubernetes.io/name=prometheus
```

### "Remote write receiver not enabled" error

```bash
# Verify feature is enabled
kubectl logs -n monitoring prometheus-prometheus-kube-prometheus-prometheus-0 | grep "remote-write-receiver"

# Should see: msg="Experimental features enabled" features=[remote-write-receiver]
```

### "Out of order sample" error

This occurs when trying to insert data older than existing data for the same time series.

**Solutions:**
- Use different job labels for historic data (e.g., `job="historic_data"`)
- Enable out-of-order ingestion in Prometheus (experimental)
- Ensure backfill goes from oldest to newest

### Dashboard not appearing in Grafana

```bash
# Check ConfigMap exists
kubectl get configmap -n monitoring | grep prometheus-pusher

# Check labels
kubectl get configmap prometheus-pusher-dashboard -n monitoring -o yaml | grep "grafana_dashboard"

# Restart Grafana to force reload
kubectl rollout restart deployment/prometheus-grafana -n monitoring
```

## Architecture

```
┌─────────────────┐
│  Go Binary      │
│ (prometheus-    │──Push realtime──┐
│  pusher)        │                 │
└─────────────────┘                 ▼
         │                  ┌──────────────────┐
         │                  │  Pushgateway     │◄──Scrape──┐
         │                  │  (Port 9091)     │           │
         │                  └──────────────────┘           │
         │                                                 │
         └──Push historic──────────────────┐              │
                                            ▼              │
                                  ┌─────────────────┐     │
                                  │   Prometheus    │◄────┘
                                  │   (Port 9090)   │
                                  │ Remote Write API│
                                  └─────────────────┘
                                           │
                                           │ Datasource
                                           ▼
                                  ┌─────────────────┐
                                  │    Grafana      │
                                  │   (Port 3000)   │
                                  │   Dashboards    │
                                  └─────────────────┘
```

## Best Practices

### When to Use Pushgateway vs. Remote Write

**Use Pushgateway (realtime mode):**
- Short-lived batch jobs
- Service-level metrics
- Jobs behind firewalls
- Current/recent data (< 5 minutes old)

**Use Remote Write (historic mode):**
- Historic data import
- Backfilling gaps
- Data migration
- Data older than 5 minutes

**Use Auto Mode:**
- Mixed current and historic data
- Importing from files
- Unknown timestamp ages
- General-purpose ingestion

### Metric Design

- **Use appropriate metric types:**
  - Counter for cumulative values (requests, errors)
  - Gauge for point-in-time values (temperature, connections)
  - Histogram for distributions (latency, sizes)

- **Label cardinality:**
  - Include meaningful labels
  - Avoid high-cardinality labels (user IDs, timestamps)
  - Keep label combinations reasonable (< 1000 per metric)

- **Naming conventions:**
  - Use descriptive names
  - Include units in gauge names (\_celsius, \_bytes)
  - Use \_total suffix for counters

## Cleanup

```bash
# Stop port-forwards
pkill -f "port-forward.*9091"
pkill -f "port-forward.*9090"
pkill -f "port-forward.*3000"

# Delete test metrics from Pushgateway
curl -X DELETE http://localhost:9091/metrics/job/example_metrics_pusher

# Uninstall Pushgateway (if needed)
helm uninstall pushgateway -n monitoring
```

## Additional Resources

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Pushgateway Documentation](https://github.com/prometheus/pushgateway)
- [Prometheus Remote Write Spec](https://prometheus.io/docs/concepts/remote_write_spec/)
- [Grafana Documentation](https://grafana.com/docs/)

## Version

Current version: 0.0.0

## License

See LICENSE file for details.
