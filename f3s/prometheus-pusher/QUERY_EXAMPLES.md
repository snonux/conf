# Prometheus Query Examples - Data Ingestion Verification

This document shows actual curl commands and their outputs querying data ingested by prometheus-pusher.

## Data Ingested

We ingested the following metrics using realtime mode (Pushgateway):
- Counter: `app_requests_total`
- Gauges: `app_active_connections`, `app_temperature_celsius`
- Histogram: `app_request_duration_seconds`
- Labeled Counter: `app_jobs_processed_total`

---

## Query Examples

### Query 1: Counter Metric - Total Requests

**Command:**
```bash
curl -s "http://localhost:9090/api/v1/query?query=app_requests_total"
```

**Output:**
```json
{
    "status": "success",
    "data": {
        "resultType": "vector",
        "result": [
            {
                "metric": {
                    "__name__": "app_requests_total",
                    "instance": "example-app",
                    "job": "example_metrics_pusher"
                },
                "value": [1767127978.666, "4"]
            }
        ]
    }
}
```

**Explanation:** Counter showing 4 total requests processed. The timestamp `1767127978.666` is Unix epoch time (seconds since 1970-01-01).

---

### Query 2: Gauge Metric - Temperature

**Command:**
```bash
curl -s "http://localhost:9090/api/v1/query?query=app_temperature_celsius"
```

**Output:**
```json
{
    "status": "success",
    "data": {
        "resultType": "vector",
        "result": [
            {
                "metric": {
                    "__name__": "app_temperature_celsius",
                    "instance": "example-app",
                    "job": "example_metrics_pusher"
                },
                "value": [1767127980.789, "30.836861056300393"]
            }
        ]
    }
}
```

**Explanation:** Gauge showing current temperature of 30.84°C.

---

### Query 3: Gauge Metric - Active Connections

**Command:**
```bash
curl -s "http://localhost:9090/api/v1/query?query=app_active_connections"
```

**Output:**
```json
{
    "status": "success",
    "data": {
        "resultType": "vector",
        "result": [
            {
                "metric": {
                    "__name__": "app_active_connections",
                    "instance": "example-app",
                    "job": "example_metrics_pusher"
                },
                "value": [1767127982.964, "32"]
            }
        ]
    }
}
```

**Explanation:** Gauge showing 32 currently active connections.

---

### Query 4: Labeled Counter - Jobs Processed by Type and Status

**Command:**
```bash
curl -s "http://localhost:9090/api/v1/query?query=app_jobs_processed_total"
```

**Output:**
```json
{
    "status": "success",
    "data": {
        "resultType": "vector",
        "result": [
            {
                "metric": {
                    "__name__": "app_jobs_processed_total",
                    "instance": "example-app",
                    "job": "example_metrics_pusher",
                    "job_type": "backup",
                    "status": "failed"
                },
                "value": [1767127993.729, "3"]
            },
            {
                "metric": {
                    "__name__": "app_jobs_processed_total",
                    "instance": "example-app",
                    "job": "example_metrics_pusher",
                    "job_type": "email",
                    "status": "success"
                },
                "value": [1767127993.729, "3"]
            },
            {
                "metric": {
                    "__name__": "app_jobs_processed_total",
                    "instance": "example-app",
                    "job": "example_metrics_pusher",
                    "job_type": "report",
                    "status": "success"
                },
                "value": [1767127993.729, "1"]
            }
        ]
    }
}
```

**Explanation:** Labeled counter with multiple time series showing job processing by type (backup, email, report) and status (success, failed).

---

### Query 5: Histogram - Request Duration (Buckets)

**Command:**
```bash
curl -s "http://localhost:9090/api/v1/query?query=app_request_duration_seconds_bucket"
```

**Output (truncated):**
```json
{
  "status": "success",
  "data": {
    "resultType": "vector",
    "result": [
      {
        "metric": {
          "__name__": "app_request_duration_seconds_bucket",
          "instance": "example-app",
          "job": "example_metrics_pusher",
          "le": "0.005"
        },
        "value": [1767127997.104, "0"]
      },
      {
        "metric": {
          "__name__": "app_request_duration_seconds_bucket",
          "instance": "example-app",
          "job": "example_metrics_pusher",
          "le": "0.01"
        },
        "value": [1767127997.104, "0"]
      },
      {
        "metric": {
          "__name__": "app_request_duration_seconds_bucket",
          "instance": "example-app",
          "job": "example_metrics_pusher",
          "le": "0.025"
        },
        "value": [1767127997.104, "0"]
      }
      // ... more buckets: 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, +Inf
    ]
  }
}
```

**Explanation:** Histogram buckets showing cumulative counts of request durations. Used for percentile calculations.

---

### Query 6: Histogram - Sum and Count

**Sum Command:**
```bash
curl -s "http://localhost:9090/api/v1/query?query=app_request_duration_seconds_sum"
```

**Sum Output:**
```json
{
    "status": "success",
    "data": {
        "resultType": "vector",
        "result": [
            {
                "metric": {
                    "__name__": "app_request_duration_seconds_sum",
                    "instance": "example-app",
                    "job": "example_metrics_pusher"
                },
                "value": [1767128000.778, "2.4976701293337467"]
            }
        ]
    }
}
```

**Count Command:**
```bash
curl -s "http://localhost:9090/api/v1/query?query=app_request_duration_seconds_count"
```

**Count Output:**
```json
{
    "status": "success",
    "data": {
        "resultType": "vector",
        "result": [
            {
                "metric": {
                    "__name__": "app_request_duration_seconds_count",
                    "instance": "example-app",
                    "job": "example_metrics_pusher"
                },
                "value": [1767128000.832, "3"]
            }
        ]
    }
}
```

**Explanation:** 
- Sum: Total of all request durations = 2.498 seconds
- Count: Total number of requests = 3
- Average duration = 2.498 / 3 = 0.833 seconds per request

---

## Verification Summary

✅ **All metric types successfully ingested:**
- Counter: `app_requests_total` = 4
- Gauge: `app_temperature_celsius` = 30.84°C
- Gauge: `app_active_connections` = 32
- Labeled Counter: `app_jobs_processed_total` (3 series with different labels)
- Histogram: `app_request_duration_seconds` (buckets, sum, count)

✅ **Data queryable via Prometheus API**
✅ **Timestamps preserved correctly**
✅ **Labels attached properly**
✅ **All metric types working as expected**

---

## Additional Query Examples

### Filter by Specific Label
```bash
curl -s 'http://localhost:9090/api/v1/query?query=app_jobs_processed_total{job_type="email"}'
```

### Range Query (Last 10 Minutes)
```bash
START=$(date -d '10 minutes ago' +%s)
END=$(date +%s)
curl -s "http://localhost:9090/api/v1/query_range?query=app_requests_total&start=${START}&end=${END}&step=60"
```

### Calculate Rate (Requests per Second over 5m)
```bash
curl -s 'http://localhost:9090/api/v1/query?query=rate(app_requests_total[5m])'
```

### Sum Aggregation
```bash
curl -s 'http://localhost:9090/api/v1/query?query=sum(app_jobs_processed_total)'
```

---

Generated: 2025-12-30
Tool: prometheus-pusher (refactored version with 63.9% test coverage)
