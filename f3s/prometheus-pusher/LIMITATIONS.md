# Prometheus Ingestion Limitations

## Time Range Limits

### ✅ What Works (Tested)

| Time Range | Status | Method |
|------------|--------|--------|
| Current (now) | ✅ Works | Pushgateway |
| 1 hour old | ✅ Works | Remote Write |
| 1 day old | ✅ Works | Remote Write |
| 1 week old | ✅ Works | Remote Write |
| 1 month old | ✅ Works | Remote Write |

### ⚠️ Potential Issues

#### 1. **Future Data** (timestamps in the future)

**Limit**: Prometheus rejects samples too far in the future

```bash
# Default: ~5 minutes into the future is allowed
# Controlled by: --storage.tsdb.allow-out-of-order-time-window
```

**Example that might fail**:
```csv
# 1 hour in the future - WILL BE REJECTED
app_requests_total,instance=test,100,TIMESTAMP_1H_FUTURE
```

**Error**: `sample is too far in the future`

#### 2. **Very Old Data** (months/years old)

**Limits depend on**:
- Prometheus retention period
- TSDB block structure
- `--storage.tsdb.min-block-duration` setting

**Typical limits**:
- **Few months old**: Usually works
- **6+ months old**: May be rejected
- **Years old**: Likely rejected unless using promtool

**Example that might fail**:
```csv
# 6 months old - MIGHT BE REJECTED
app_requests_total,instance=test,100,TIMESTAMP_6M_AGO

# 1 year old - LIKELY REJECTED
app_requests_total,instance=test,100,TIMESTAMP_1Y_AGO
```

**Error**: `sample is too old`

#### 3. **Out-of-Order Samples** (for existing time series)

**Problem**: If a time series already has recent data, you can't insert older data

**Example**:
```bash
# Step 1: Push current data
echo "app_requests_total,instance=test,100,$NOW" | ./prometheus-pusher -mode=auto

# Step 2: Try to push older data for SAME time series - WILL BE REJECTED
echo "app_requests_total,instance=test,95,$ONE_HOUR_AGO" | ./prometheus-pusher -mode=auto
```

**Error**: `out of order sample`

**Workaround**: Use different labels (different time series)

#### 4. **Pushgateway Timestamp Limitations**

**Problem**: Pushgateway NEVER preserves timestamps

- All data uses "now" when Prometheus scrapes
- Cannot backfill old data via Pushgateway
- Only suitable for current/recent data

**Example**:
```csv
# Even with old timestamp, Pushgateway uses "now"
app_requests_total,instance=current,100,TIMESTAMP_1D_AGO
# ↑ This timestamp is IGNORED by Pushgateway
```

## Testing Edge Cases

Let me create a test for various edge cases:

```bash
#!/bin/bash
# test-limits.sh

NOW=$(date +%s)000

# Test cases
cat > test-limits.csv << EOF
# Edge case tests

# 1. CURRENT - should work
app_test_current,instance=test1,100,$NOW

# 2. 5 minutes in future - might work
app_test_future_5m,instance=test2,100,$((NOW + 300000))

# 3. 1 hour in future - will likely be rejected
app_test_future_1h,instance=test3,100,$((NOW + 3600000))

# 4. 2 months old - might work
app_test_2m_old,instance=test4,100,$((NOW - 5184000000))

# 5. 6 months old - might be rejected
app_test_6m_old,instance=test5,100,$((NOW - 15552000000))

# 6. 1 year old - likely rejected
app_test_1y_old,instance=test6,100,$((NOW - 31536000000))

# 7. 2 years old - very likely rejected
app_test_2y_old,instance=test7,100,$((NOW - 63072000000))
EOF

echo "Testing edge cases..."
./prometheus-pusher -mode=auto -file=test-limits.csv
```

## Prometheus Configuration Limits

### Default Settings

```yaml
# Prometheus default limits
--storage.tsdb.retention.time=15d       # Data older than 15 days is deleted
--storage.tsdb.min-block-duration=2h    # Minimum block size
--web.enable-remote-write-receiver      # Must be enabled for historic data
```

### What These Mean for Ingestion

1. **Retention Time** (`--storage.tsdb.retention.time`)
   - Default: 15 days
   - Can't ingest data older than retention period
   - Check your Prometheus config

2. **Min Block Duration** (`--storage.tsdb.min-block-duration`)
   - Affects how old data can be written
   - Default: 2 hours
   - Older data needs to align with block boundaries

3. **Out-of-Order Time Window**
   - Default: disabled
   - Can be enabled with `--enable-feature=out-of-order-ingestion`
   - Allows writing old data to existing series

## Checking Your Limits

```bash
# Check Prometheus retention
kubectl get prometheus -n monitoring prometheus-kube-prometheus-prometheus \
  -o jsonpath='{.spec.retention}'

# Check Prometheus logs for limits
kubectl logs -n monitoring prometheus-prometheus-kube-prometheus-prometheus-0 \
  | grep -i "retention\|block\|sample.*old\|sample.*future"
```

## Summary Table

| Time Range | Ingestion | Notes |
|------------|-----------|-------|
| 1 hour future | ❌ Rejected | "Too far in future" |
| 5 min future | ⚠️ Maybe | Depends on config |
| Current | ✅ Works | Both methods |
| 1 hour old | ✅ Works | Remote Write |
| 1 day old | ✅ Works | Remote Write |
| 1 week old | ✅ Works | Remote Write |
| 1 month old | ✅ Works | Remote Write |
| 2 months old | ⚠️ Maybe | Depends on retention |
| 6 months old | ⚠️ Maybe | Likely rejected |
| 1 year old | ❌ Rejected | Too old |
| 2+ years old | ❌ Rejected | Way too old |

## Solutions for Very Old Data

If you need to ingest data older than a few months:

### Option 1: Use promtool (Recommended for very old data)

```bash
# 1. Export data in OpenMetrics format
cat > old-metrics.txt << EOF
# HELP app_requests_total Total requests
# TYPE app_requests_total counter
app_requests_total{instance="old"} 100 TIMESTAMP_1Y_AGO
EOF

# 2. Create TSDB blocks directly
promtool tsdb create-blocks-from openmetrics old-metrics.txt /prometheus/data

# 3. Restart Prometheus to load new blocks
kubectl rollout restart statefulset/prometheus-prometheus-kube-prometheus-prometheus -n monitoring
```

### Option 2: Adjust Prometheus Retention

```yaml
# Increase retention to accept older data
prometheus:
  prometheusSpec:
    retention: 90d  # Keep data for 90 days
    retentionSize: 50GB
```

### Option 3: Enable Out-of-Order Ingestion

```yaml
# Allow out-of-order samples
prometheus:
  prometheusSpec:
    enableFeatures:
      - out-of-order-ingestion
    additionalArgs:
      - --storage.tsdb.out-of-order-time-window=30d
```

## Best Practices

1. ✅ **Current to 1 month**: Use prometheus-pusher auto mode
2. ⚠️ **1-3 months old**: Test first, may need config changes
3. ❌ **6+ months old**: Use promtool instead
4. ❌ **Years old**: Definitely use promtool

## Testing Your Limits

To find your exact limits:

```bash
# Generate test data for various ages
./generate-test-data.sh

# Try importing and watch for errors
./prometheus-pusher -mode=auto -file=test-limits.csv 2>&1 | tee import.log

# Check what failed
grep -i "error\|rejected\|failed" import.log
```

## Error Messages Guide

| Error Message | Meaning | Solution |
|---------------|---------|----------|
| `sample is too old` | Beyond retention | Use promtool or increase retention |
| `sample is too far in the future` | Timestamp in future | Check your clock/timestamps |
| `out of order sample` | Older than existing data | Use different labels or enable OOO |
| `remote write receiver not enabled` | Feature not enabled | Enable --web.enable-remote-write-receiver |
| `sample timestamp out of order` | Wrong order in batch | Sort by timestamp |

## Conclusion

**Practical Limits for prometheus-pusher**:
- ✅ **Safe range**: Current to 1 month old
- ⚠️ **Test first**: 1-3 months old
- ❌ **Use promtool**: 3+ months old

The 1 month limit we tested (and works!) is a safe, practical upper bound for most use cases.
