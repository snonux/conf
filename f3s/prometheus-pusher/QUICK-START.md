# Quick Start - Single Binary, All Features

## One Binary: `prometheus-pusher`

All features in one tool! Choose your mode:

### 🔄 Realtime Mode (Default)
Push current metrics to Pushgateway

```bash
./prometheus-pusher -mode=realtime -continuous
```

### ⏰ Historic Mode
Push single datapoint from the past

```bash
./prometheus-pusher -mode=historic -hours-ago=24
```

### 📦 Backfill Mode
Import range of historic data

```bash
./prometheus-pusher -mode=backfill -start-hours=48 -end-hours=0 -interval=1
```

### 🤖 Auto Mode (Recommended!)
Automatically detect timestamp age and route correctly

```bash
./prometheus-pusher -mode=auto -file=data.csv
```

## Quick Examples

### Import Current, 1h, 1d, 1w, 1m Old Data (All at Once!)

```bash
# 1. Generate test data
./generate-test-data.sh

# 2. Port-forward services
kubectl port-forward -n monitoring svc/pushgateway 9091:9091 &
kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090 &

# 3. Auto-import everything
./prometheus-pusher -mode=auto -file=test-all-ages.csv

# Output:
# 📊 Auto-ingest summary:
#   Realtime samples (< 5min old): 3
#   Historic samples (> 5min old): 12
# 🔄 Ingesting REALTIME via Pushgateway...
# ⏰ Ingesting HISTORIC via Remote Write...
#   [1/12] app_requests_total (age: 1.0 hours)
#   [4/12] app_temperature_celsius (age: 1.0 days)
#   [7/12] app_requests_total (age: 7.0 days)
#   [10/12] app_requests_total (age: 30.0 days)
# 🎉 Auto-ingest complete!
```

## Data Format (CSV)

```csv
# metric_name,labels,value,timestamp_ms
app_requests_total,instance=web1;env=prod,100,1767125148000
app_temperature_celsius,instance=web2,22.5,1767038748000
```

## All Modes in One Command

```bash
# See all options
./prometheus-pusher -help

# Modes:
#   realtime  - Push current metrics to Pushgateway
#   historic  - Push single historic datapoint
#   backfill  - Backfill range of datapoints
#   auto      - Automatically detect and route
```

## Documentation

- `ANSWER.md` - Can it import all time ranges? YES!
- `AUTO-MODE.md` - Complete auto mode guide
- `HISTORIC.md` - Historic data ingestion details
- `README.md` - Project overview
- `USAGE.md` - Detailed usage guide

## Summary

✅ **One binary** - No confusion
✅ **Four modes** - All use cases covered
✅ **Auto detection** - No manual timestamp calculation
✅ **All time ranges** - Current to 1 month old
✅ **Clear logging** - See exactly what's happening

Just run:
```bash
./prometheus-pusher -mode=auto -file=your-data.csv
```

Done! 🎉
