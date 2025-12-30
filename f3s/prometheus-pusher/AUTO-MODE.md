# Auto Mode - Automatic Timestamp Detection

## Overview

The **AUTO mode** is a smart ingestion mode that:
1. **Reads metrics** with timestamps from a file or stdin
2. **Automatically detects** how old each metric is
3. **Chooses the right ingestion method**:
   - Realtime data (< 5 minutes old) → Pushgateway
   - Historic data (> 5 minutes old) → Remote Write API
4. **Logs what it's doing** so you can see which method is used

**No manual timestamp calculation needed!** Just provide data with timestamps.

## Why Use Auto Mode?

### Problem
Previously, you had to:
- Manually calculate how old your data is
- Choose between `--mode=realtime` or `--mode=historic`
- Specify `-hours-ago` for each datapoint

### Solution
Now you can:
- Provide data with timestamps in any format (CSV or JSON)
- The tool automatically detects age and chooses ingestion method
- Batch import mixed data (some current, some old)

## Usage

### From File

```bash
# CSV format
./prometheus-pusher-auto -mode=auto -file=metrics.csv -format=csv

# JSON format
./prometheus-pusher-auto -mode=auto -file=metrics.json -format=json
```

### From Stdin

```bash
# Pipe CSV data
cat metrics.csv | ./prometheus-pusher-auto -mode=auto -format=csv

# Interactive input
./prometheus-pusher-auto -mode=auto -format=csv
# (then paste data and press Ctrl+D)
```

## Input Formats

### CSV Format

```
# Format: metric_name,labels,value,timestamp_ms
# Labels: key1=value1;key2=value2

app_requests_total,instance=web1;env=prod,100,1767125148000
app_temperature_celsius,instance=web2;zone=us,22.5,1767038748000
```

**Fields**:
1. `metric_name`: Prometheus metric name
2. `labels`: Semicolon-separated label pairs (optional)
3. `value`: Metric value (float)
4. `timestamp_ms`: Unix timestamp in milliseconds (optional, defaults to now)

**Example**:
```csv
# Current data (no timestamp = uses now)
app_requests_total,instance=web1,100,

# 1 hour ago
app_requests_total,instance=web2,95,1767121548000

# 1 day ago
app_requests_total,instance=web3,150,1767038748000
```

### JSON Format

```json
[
  {
    "metric": "app_requests_total",
    "labels": {"instance": "web1", "env": "prod"},
    "value": 100,
    "timestamp_ms": 1767125148000
  },
  {
    "metric": "app_temperature_celsius",
    "labels": {"instance": "web2", "zone": "us"},
    "value": 22.5,
    "timestamp_ms": 1767038748000
  }
]
```

**Fields**:
- `metric`: Metric name (required)
- `labels`: Object with label key-value pairs (optional)
- `value`: Metric value (required)
- `timestamp_ms`: Unix timestamp in milliseconds (optional)

## Generating Test Data

Use the provided script to generate test data for all time ranges:

```bash
./generate-test-data.sh
```

This creates `test-all-ages.csv` with:
- Current data (< 5 min old)
- 1 hour old data
- 1 day old data
- 1 week old data
- 1 month old data

## Example: Import All Time Ranges

```bash
# 1. Generate test data
./generate-test-data.sh

# 2. Port-forward Prometheus (for historic data)
kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090 &

# 3. Port-forward Pushgateway (for current data)
kubectl port-forward -n monitoring svc/pushgateway 9091:9091 &

# 4. Auto-import all data
./prometheus-pusher-auto \
  -mode=auto \
  -file=test-all-ages.csv \
  -format=csv \
  -pushgateway=http://localhost:9091 \
  -prometheus=http://localhost:9090/api/v1/write
```

**Expected Output**:
```
🤖 AUTO mode: Automatically detecting timestamp age and choosing ingestion method

📁 Reading metrics from: test-all-ages.csv (format: csv)
📊 Auto-ingest summary:
  Total samples: 15
  Realtime samples (< 5min old): 3
  Historic samples (> 5min old): 12

🔄 Ingesting 3 REALTIME samples via Pushgateway...
  Note: Pushgateway ingestion uses current timestamp
✅ Successfully ingested 3 realtime samples

⏰ Ingesting 12 HISTORIC samples via Remote Write...
  [1/12] app_requests_total (age: 1.0 hours)
  [2/12] app_active_connections (age: 1.0 hours)
  [3/12] app_temperature_celsius (age: 1.0 hours)
  [4/12] app_requests_total (age: 1.0 days)
  [5/12] app_temperature_celsius (age: 1.0 days)
  [6/12] app_active_connections (age: 1.0 days)
  [7/12] app_requests_total (age: 7.0 days)
  [8/12] app_jobs_processed_total (age: 7.0 days)
  [9/12] app_temperature_celsius (age: 7.0 days)
  [10/12] app_requests_total (age: 30.0 days)
  [11/12] app_active_connections (age: 30.0 days)
  [12/12] app_temperature_celsius (age: 30.0 days)
✅ Successfully ingested 12 historic samples

🎉 Auto-ingest complete!
```

## Detection Logic

The tool uses a **5-minute threshold**:

| Data Age | Ingestion Method | Reason |
|----------|------------------|---------|
| < 5 minutes | Pushgateway (realtime) | Recent enough to use "now" timestamp |
| ≥ 5 minutes | Remote Write (historic) | Too old, needs preserved timestamp |

**Why 5 minutes?**
- Allows for clock skew and processing delays
- Prometheus scrapes Pushgateway every 15-30s
- Gives buffer for network delays

## Query Imported Data

After import, query in Prometheus:

```promql
# View current data (from Pushgateway)
{instance="current"}

# View 1 hour old data
{instance="1h_ago"}

# View 1 day old data
{instance="1d_ago"}

# View 1 week old data
{instance="1w_ago"}

# View 1 month old data
{instance="1m_ago"}

# All imported data
{env="prod"}
```

## Flags

```
-mode=auto          Enable auto mode
-file=<path>        Input file (CSV or JSON)
-format=<fmt>       Format: csv or json (default: csv)
-pushgateway=<url>  Pushgateway URL (default: http://localhost:9091)
-prometheus=<url>   Prometheus remote write URL (default: http://localhost:9090/api/v1/write)
-job=<name>         Job name for metrics (default: example_metrics_pusher)
```

## Supported Time Ranges

✅ **Current data** (< 5 min): Works perfectly
✅ **1 hour old**: Works via Remote Write
✅ **1 day old**: Works via Remote Write
✅ **1 week old**: Works via Remote Write
✅ **1 month old**: Works via Remote Write
⚠️ **Very old data** (months/years): May hit Prometheus limits

For very old data (> few months), consider:
- Using `promtool tsdb create-blocks-from` instead
- Increasing Prometheus retention settings
- Using long-term storage solutions

## Benefits

1. **No timestamp math** - Tool calculates age automatically
2. **Mixed data** - Import both current and historic data in one go
3. **Visual feedback** - See exactly which ingestion method is used
4. **Batch import** - Process large CSV/JSON files easily
5. **Error handling** - Clear messages if ingestion fails

## Comparison with Other Modes

| Mode | Use Case | Timestamp Handling |
|------|----------|-------------------|
| `realtime` | Live monitoring | Always uses "now" |
| `historic` | Single old datapoint | Manually specify `-hours-ago` |
| `backfill` | Range of datapoints | Manually specify range |
| `auto` | **Any mix of data** | **Automatic detection** |

## Advanced Example: Import from Multiple Sources

```bash
# Generate various test data
./generate-test-data.sh

# Import yesterday's backup
./prometheus-pusher-auto -mode=auto -file=backup_yesterday.csv

# Import last week's logs
./prometheus-pusher-auto -mode=auto -file=logs_lastweek.json -format=json

# Import current metrics
./prometheus-pusher-auto -mode=auto -file=current_metrics.csv
```

All data is automatically routed to the correct ingestion method!

## Troubleshooting

### "No valid samples found"
- Check CSV/JSON format
- Ensure timestamps are in milliseconds
- Check for syntax errors in labels

### "Remote write receiver not enabled"
- Ensure Prometheus has `--web.enable-remote-write-receiver` flag
- Check prometheus/persistence-values.yaml configuration

### "Pushgateway connection refused"
- Verify port-forward: `kubectl port-forward -n monitoring svc/pushgateway 9091:9091`
- Check Pushgateway is running: `kubectl get pods -n monitoring | grep pushgateway`

## Summary

Auto mode makes importing data effortless:
- 📥 Read data from file or stdin
- 🔍 Automatically detect timestamp age
- 🎯 Choose optimal ingestion method
- 📊 Clear logging of what's happening
- ✅ Support for all time ranges (current → 1 month old)

No more manual timestamp calculations - just provide your data!
