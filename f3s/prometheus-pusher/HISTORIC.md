# Historic Data Ingestion

This document explains how to ingest historic data into Prometheus using the prometheus-pusher tool.

## Problem

The standard Pushgateway approach has a limitation: it doesn't support custom timestamps. When you push metrics to Pushgateway, Prometheus scrapes them with the current timestamp. This means you cannot backfill historic data (e.g., data from yesterday or last week).

## Solution

Prometheus supports the **Remote Write API** which accepts timestamped samples. By enabling the `remote-write-receiver` feature flag, Prometheus can accept historic data with custom timestamps via HTTP POST.

### Limitations

- **Out-of-order samples**: By default, Prometheus rejects samples that are older than the most recent sample for that time series
- **Time window**: Prometheus typically accepts data within a certain time window (default: up to 1 hour in the past for new series)
- **Feature flag required**: The remote write receiver must be enabled with `--enable-feature=remote-write-receiver`

## Setup

### 1. Enable Remote Write Receiver

The Prometheus instance needs to be configured with the remote write receiver feature:

```yaml
# In prometheus/persistence-values.yaml
prometheus:
  prometheusSpec:
    enableFeatures:
      - remote-write-receiver
```

This has been configured and applied to the monitoring namespace Prometheus instance.

### 2. Verify Feature is Enabled

```bash
kubectl logs -n monitoring prometheus-prometheus-kube-prometheus-prometheus-0 | grep "remote-write-receiver"
```

You should see: `msg="Experimental features enabled" features=[remote-write-receiver]`

## Usage

The `prometheus-pusher` binary supports three modes:

### Mode 1: Realtime (Default)

Push current metrics to Pushgateway (same as before):

```bash
./prometheus-pusher -mode=realtime -continuous
```

Options:
- `-pushgateway`: Pushgateway URL (default: http://localhost:9091)
- `-job`: Job name (default: example_metrics_pusher)
- `-continuous`: Keep pushing every 15 seconds

### Mode 2: Historic (Single Datapoint)

Push a single datapoint from X hours ago:

```bash
# Port-forward Prometheus
kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090 &

# Push data from 24 hours ago
./prometheus-pusher -mode=historic -hours-ago=24

# Push data from 3 hours ago
./prometheus-pusher -mode=historic -hours-ago=3

# Push data from yesterday (48 hours ago)
./prometheus-pusher -mode=historic -hours-ago=48
```

Options:
- `-prometheus`: Prometheus remote write URL (default: http://localhost:9090/api/v1/write)
- `-hours-ago`: How many hours in the past (default: 24)

### Mode 3: Backfill (Multiple Datapoints)

Backfill a range of historic data:

```bash
# Backfill last 48 hours with 1-hour intervals
./prometheus-pusher -mode=backfill -start-hours=48 -end-hours=0 -interval=1

# Backfill last week with 6-hour intervals
./prometheus-pusher -mode=backfill -start-hours=168 -end-hours=0 -interval=6

# Backfill specific range (24h ago to 12h ago, every 2 hours)
./prometheus-pusher -mode=backfill -start-hours=24 -end-hours=12 -interval=2
```

Options:
- `-start-hours`: Start time in hours ago (e.g., 48 = 2 days ago)
- `-end-hours`: End time in hours ago (e.g., 0 = now)
- `-interval`: Interval between datapoints in hours

## Data Format

Historic data is sent using the Prometheus Remote Write protocol (Protobuf):

1. **Protocol**: HTTP POST with Protobuf payload
2. **Encoding**: Snappy compression
3. **Headers**:
   - Content-Type: application/x-protobuf
   - Content-Encoding: snappy
   - X-Prometheus-Remote-Write-Version: 0.1.0

4. **Payload**: TimeSeries with custom timestamps

Example time series:
```protobuf
TimeSeries {
  Labels: [
    {Name: "__name__", Value: "app_requests_total"},
    {Name: "instance", Value: "example-app"},
    {Name: "job", Value: "historic_data"}
  ],
  Samples: [
    {Value: 42, Timestamp: 1735516800000}  // milliseconds since epoch
  ]
}
```

## Example: Backfill Last 24 Hours

```bash
#!/bin/bash

# 1. Port-forward Prometheus
kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090 &
PF_PID=$!
sleep 2

# 2. Backfill data for every hour in the last 24 hours
cd /home/paul/git/conf/f3s/prometheus-pusher
./prometheus-pusher \
  -mode=backfill \
  -prometheus=http://localhost:9090/api/v1/write \
  -start-hours=24 \
  -end-hours=0 \
  -interval=1

# 3. Clean up
kill $PF_PID
```

## Querying Historic Data

Once backfilled, the historic data is queryable in Prometheus:

```promql
# View all historic data
{job="historic_data"}

# View specific metric from historic data
app_requests_total{job="historic_data"}

# View data from a specific time range
app_temperature_celsius{job="historic_data"}[24h]

# Compare realtime vs historic data
app_requests_total{job="example_metrics_pusher"}  # realtime
app_requests_total{job="historic_data"}           # historic
```

## Troubleshooting

### Error: "remote write receiver not enabled"

```
Error: remote write failed with status 404: remote write receiver not enabled
```

Solution: Ensure Prometheus has the `remote-write-receiver` feature enabled and has restarted.

### Error: "out of order sample"

```
Error: sample timestamp out of order
```

This occurs when trying to insert data older than existing data for the same time series. Solutions:
1. Use a different job label for historic data (already done: `job="historic_data"`)
2. Enable out-of-order ingestion in Prometheus (experimental)
3. Ensure backfill starts from oldest to newest

### Error: "sample too old"

```
Error: sample is too old
```

Prometheus has limits on how old data can be. By default:
- For existing series: can't be older than the oldest block
- For new series: typically accepts data up to 1 hour old

Solution: For very old data (weeks/months), use `promtool tsdb create-blocks-from` instead.

## Best Practices

1. **Use different job labels**: Historic data uses `job="historic_data"`, realtime uses `job="example_metrics_pusher"`
2. **Backfill in order**: Always backfill from oldest to newest to avoid out-of-order rejections
3. **Small batches**: Don't overwhelm Prometheus - the tool includes 100ms delays between datapoints
4. **Verify first**: Test with a single datapoint before running large backfills
5. **Monitor errors**: Check Prometheus logs if ingestion fails

## Limitations

- **Very old data**: For data older than a few days, consider using `promtool` for TSDB block creation
- **High cardinality**: Be careful with label combinations - they create separate time series
- **Performance**: Large backfills can impact Prometheus performance
- **Out-of-order**: By default, Prometheus rejects out-of-order samples

## Alternative: Using Promtool

For very large historic datasets, you can use `promtool` to create TSDB blocks:

```bash
# 1. Generate OpenMetrics format file
./prometheus-pusher -mode=export -output=metrics.txt

# 2. Create blocks from the file
promtool tsdb create-blocks-from openmetrics metrics.txt /path/to/prometheus/data
```

This method bypasses the API and writes directly to the TSDB, but requires filesystem access.
