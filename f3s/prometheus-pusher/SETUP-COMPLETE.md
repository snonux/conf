# Historic Data Ingestion - Setup Complete

## ✅ What Was Done

### 1. Extended prometheus-pusher for Historic Data

The application now supports three modes:

**Mode 1: Realtime** (Original behavior)
- Pushes current metrics to Pushgateway
- Prometheus scrapes with current timestamp
- Use for ongoing monitoring

**Mode 2: Historic** (NEW)
- Push single datapoint with custom timestamp
- Specify hours ago (e.g., 24 = yesterday)
- Uses Prometheus Remote Write API

**Mode 3: Backfill** (NEW)
- Backfill range of historic data
- Specify start, end, and interval
- Batch ingestion for large datasets

### 2. Code Structure

```
prometheus-pusher/
├── main.go              # Main entry point with mode selection
├── realtime.go          # Original Pushgateway functionality
├── historic.go          # NEW: Remote Write with timestamps
├── prometheus-pusher-historic  # NEW: Binary with all modes
└── HISTORIC.md          # Complete documentation
```

### 3. Technical Implementation

**Remote Write Protocol**:
- Format: Protobuf (prompb.WriteRequest)
- Encoding: Snappy compression
- Headers: X-Prometheus-Remote-Write-Version: 0.1.0
- Endpoint: /api/v1/write

**Key Insight**: Pushgateway doesn't support timestamps, but Remote Write does!

### 4. Prometheus Configuration Update

Updated `/home/paul/git/conf/f3s/prometheus/persistence-values.yaml`:

```yaml
prometheus:
  prometheusSpec:
    additionalArgs:
      - name: web.enable-remote-write-receiver
        value: "true"
```

This enables Prometheus to accept historic data via Remote Write API.

## ⚠️ Pending: Cluster Issue

The Kubernetes cluster became unreachable during the final step. Once the cluster is back:

### Complete the Setup

```bash
# 1. Apply the Prometheus configuration
cd /home/paul/git/conf/f3s/prometheus
helm upgrade prometheus prometheus-community/kube-prometheus-stack \
  -n monitoring \
  -f persistence-values.yaml

# 2. Wait for Prometheus to restart
kubectl rollout status statefulset/prometheus-prometheus-kube-prometheus-prometheus \
  -n monitoring --timeout=120s

# 3. Verify remote write receiver is enabled
kubectl logs -n monitoring prometheus-prometheus-kube-prometheus-prometheus-0 \
  | grep "enable-remote-write-receiver"

# Should see: level=INFO msg="Starting Prometheus" ... web.enable-remote-write-receiver=true
```

## 🧪 Testing Historic Data Ingestion

Once the cluster is back and configured:

### Test 1: Single Historic Datapoint

```bash
cd /home/paul/git/conf/f3s/prometheus-pusher

# Port-forward Prometheus
kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090 &

# Push data from 24 hours ago
./prometheus-pusher-historic \
  -mode=historic \
  -hours-ago=24 \
  -prometheus=http://localhost:9090/api/v1/write

# Expected output:
# Successfully pushed historic data for 24 hours ago
```

### Test 2: Query Historic Data

```bash
# Query the historic data
curl -s 'http://localhost:9090/api/v1/query?query=app_requests_total{job="historic_data"}' \
  | python3 -m json.tool

# Should see data with timestamp from 24 hours ago
```

### Test 3: Backfill Multiple Datapoints

```bash
# Backfill last 48 hours with 2-hour intervals
./prometheus-pusher-historic \
  -mode=backfill \
  -start-hours=48 \
  -end-hours=0 \
  -interval=2 \
  -prometheus=http://localhost:9090/api/v1/write

# Expected output:
# Starting backfill from 48 hours ago to 0 hours ago (interval: 2 hours)
# Successfully pushed historic data for 48 hours ago...
# ...
# Backfill complete: 25 successful, 0 errors
```

### Test 4: Visualize in Prometheus UI

```bash
# Port-forward if not already done
kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090

# Open http://localhost:9090
# Query: {job="historic_data"}
# Switch to Graph view to see historic data timeline
```

## 📊 Example Queries

Once historic data is ingested:

```promql
# All historic data
{job="historic_data"}

# Compare realtime vs historic
app_requests_total{job="example_metrics_pusher"}  # current
app_requests_total{job="historic_data"}           # historic

# View specific metric from past
app_temperature_celsius{job="historic_data"}

# Rate calculation over historic data
rate(app_requests_total{job="historic_data"}[5m])

# Histogram percentiles from historic data
histogram_quantile(0.95,
  rate(app_request_duration_seconds_bucket{job="historic_data"}[5m]))
```

## 🎯 Use Cases

Now you can:

1. **Backfill missing data** during outages
2. **Import historic data** from other systems
3. **Test with specific timestamps** for debugging
4. **Migrate data** from legacy monitoring systems
5. **Generate sample data** for demonstrations

## ⚙️ Command Reference

### Realtime Mode
```bash
# Single push (original behavior)
./prometheus-pusher-historic -mode=realtime

# Continuous pushing every 15s
./prometheus-pusher-historic -mode=realtime -continuous

# Custom Pushgateway URL
./prometheus-pusher-historic \
  -mode=realtime \
  -pushgateway=http://custom-pushgateway:9091 \
  -job=my_app
```

### Historic Mode
```bash
# Yesterday's data
./prometheus-pusher-historic -mode=historic -hours-ago=24

# 3 hours ago
./prometheus-pusher-historic -mode=historic -hours-ago=3

# Last week
./prometheus-pusher-historic -mode=historic -hours-ago=168

# Custom Prometheus URL
./prometheus-pusher-historic \
  -mode=historic \
  -hours-ago=24 \
  -prometheus=http://custom-prometheus:9090/api/v1/write
```

### Backfill Mode
```bash
# Last 24 hours, hourly
./prometheus-pusher-historic -mode=backfill -start-hours=24 -end-hours=0 -interval=1

# Last week, every 6 hours
./prometheus-pusher-historic -mode=backfill -start-hours=168 -end-hours=0 -interval=6

# Specific range (48h ago to 24h ago, every 2h)
./prometheus-pusher-historic -mode=backfill -start-hours=48 -end-hours=24 -interval=2
```

## 📚 Documentation

- **HISTORIC.md**: Complete guide to historic data ingestion
- **USAGE.md**: Original realtime mode documentation
- **README.md**: Project overview
- **SUMMARY.md**: Technical architecture

## 🔧 Troubleshooting

### "remote write receiver not enabled"
```
Error: remote write failed with status 404:
  remote write receiver needs to be enabled
```

**Solution**: Complete the "Pending: Cluster Issue" steps above to enable the feature.

### "out of order sample"
```
Error: sample timestamp out of order
```

**Causes**:
1. Trying to insert data older than existing data for that series
2. Backfilling in wrong order (newest to oldest)

**Solutions**:
1. Use different job label (already done: `job="historic_data"`)
2. Backfill from oldest to newest (already implemented)
3. Delete existing series first if needed

### "sample too old"
```
Error: sample is too old
```

**Limitation**: Prometheus has limits on how old data can be (typically a few days).

**Solution**: For very old data (weeks/months), consider using `promtool tsdb create-blocks-from` to write TSDB blocks directly.

## 🎉 Summary

You now have a complete solution for:
- ✅ Realtime metrics (Pushgateway)
- ✅ Historic data ingestion (Remote Write)
- ✅ Batch backfilling (automated range)
- ✅ Flexible timestamp control
- ✅ All Prometheus metric types supported

All code is committed and pushed to git!

**Next Step**: Once cluster is back, run the setup commands above and start testing!
