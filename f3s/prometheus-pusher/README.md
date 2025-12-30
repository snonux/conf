# Prometheus Pusher

A standalone Go binary that pushes metrics to Prometheus via Pushgateway.

## Quick Start

```bash
# 1. Deploy Pushgateway (one-time - see /home/paul/git/conf/f3s/pushgateway/)
cd /home/paul/git/conf/f3s/pushgateway/helm-chart
helm upgrade --install pushgateway . -n monitoring

# 2. Run the binary
cd /home/paul/git/conf/f3s/prometheus-pusher
./run.sh
```

That's it! The binary will push metrics every 15 seconds. Press Ctrl+C to stop.

## Overview

This project consists of:
1. **Pushgateway** - A Kubernetes service that receives pushed metrics
2. **prometheus-pusher** - A standalone Go binary that generates and pushes example metrics

## Metric Types Demonstrated

The application pushes the following types of metrics:

### Counter (`app_requests_total`)
- Monotonically increasing value
- Example: Total number of requests processed
- Use case: Counting events, total requests, errors, etc.

### Gauge (`app_active_connections`, `app_temperature_celsius`)
- Value that can increase or decrease
- Examples: Active connections, temperature, memory usage
- Use case: Current state measurements

### Histogram (`app_request_duration_seconds`)
- Samples observations and counts them in configurable buckets
- Example: Request duration distribution
- Use case: Latency measurements, response times

### Counter with Labels (`app_jobs_processed_total`)
- Counter with dimensional labels
- Labels: `job_type` (email, report, backup), `status` (success, failed)
- Use case: Categorized counting

## Project Structure

```
prometheus-pusher/
├── main.go                 # Go source code
├── go.mod / go.sum        # Go dependencies
├── prometheus-pusher      # Compiled binary (standalone executable)
├── run.sh                 # Helper script to run the binary
├── example-metrics.txt    # Example of metrics format
├── USAGE.md              # Detailed usage guide
└── README.md             # This file

Note: Pushgateway Helm chart is located at /home/paul/git/conf/f3s/pushgateway/
```

## What It Does

The `prometheus-pusher` binary:
- **Generates** realistic example metrics simulating a production application
- **Pushes** metrics to Pushgateway every 15 seconds using HTTP POST
- **Demonstrates** all major Prometheus metric types with practical examples

The metrics flow: `Go Binary → Pushgateway → Prometheus → Grafana`

## Example Metrics Format

The pusher sends metrics in Prometheus format to the Pushgateway. Here's what the data looks like:

```
# HELP app_requests_total Total number of requests processed
# TYPE app_requests_total counter
app_requests_total{instance="example-app",job="example_metrics_pusher"} 42

# HELP app_active_connections Number of currently active connections
# TYPE app_active_connections gauge
app_active_connections{instance="example-app",job="example_metrics_pusher"} 67

# HELP app_temperature_celsius Current temperature in Celsius
# TYPE app_temperature_celsius gauge
app_temperature_celsius{instance="example-app",job="example_metrics_pusher"} 23.5

# HELP app_request_duration_seconds Histogram of request duration in seconds
# TYPE app_request_duration_seconds histogram
app_request_duration_seconds_bucket{instance="example-app",job="example_metrics_pusher",le="0.005"} 2
app_request_duration_seconds_bucket{instance="example-app",job="example_metrics_pusher",le="0.01"} 3
app_request_duration_seconds_bucket{instance="example-app",job="example_metrics_pusher",le="+Inf"} 10
app_request_duration_seconds_sum{instance="example-app",job="example_metrics_pusher"} 8.5
app_request_duration_seconds_count{instance="example-app",job="example_metrics_pusher"} 10

# HELP app_jobs_processed_total Total number of jobs processed by type
# TYPE app_jobs_processed_total counter
app_jobs_processed_total{instance="example-app",job="example_metrics_pusher",job_type="email",status="success"} 15
app_jobs_processed_total{instance="example-app",job="example_metrics_pusher",job_type="email",status="failed"} 2
app_jobs_processed_total{instance="example-app",job="example_metrics_pusher",job_type="report",status="success"} 8
app_jobs_processed_total{instance="example-app",job="example_metrics_pusher",job_type="backup",status="success"} 12
```

## Querying Metrics in Prometheus

Once configured, you can query these metrics in Prometheus:

```promql
# View request rate
rate(app_requests_total[5m])

# View current active connections
app_active_connections

# View 95th percentile request duration
histogram_quantile(0.95, rate(app_request_duration_seconds_bucket[5m]))

# View failed jobs by type
app_jobs_processed_total{status="failed"}

# View job success rate
rate(app_jobs_processed_total{status="success"}[5m]) / rate(app_jobs_processed_total[5m])
```

## Configuration

The pusher is configured to:
- Push metrics every 15 seconds
- Use job name: `example_metrics_pusher`
- Use instance label: `example-app`
- Connect to Pushgateway at: `http://pushgateway.monitoring.svc.cluster.local:9091`

## How It Works

1. The Go application generates random example metrics simulating a real application
2. Metrics are pushed to the Pushgateway via HTTP POST
3. Prometheus scrapes the Pushgateway periodically
4. Metrics become available in Prometheus for querying and alerting
5. Grafana can visualize these metrics

## Best Practices

- Use Pushgateway for batch jobs, short-lived processes, or service-level metrics
- For long-running applications, prefer exposing a `/metrics` endpoint for Prometheus to scrape
- Include meaningful labels but avoid high-cardinality labels (e.g., user IDs, timestamps)
- Use appropriate metric types:
  - Counter for cumulative values
  - Gauge for point-in-time values
  - Histogram/Summary for distributions
