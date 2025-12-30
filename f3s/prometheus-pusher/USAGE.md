# Prometheus Pusher - Usage Guide

## Quick Start

### 1. Deploy Pushgateway (One-time setup)

```bash
cd /home/paul/git/conf/f3s/pushgateway/helm-chart
helm upgrade --install pushgateway . -n monitoring --create-namespace
```

### 2. Update Prometheus Configuration (One-time setup)

The Prometheus scrape configuration has already been updated in `/home/paul/git/conf/f3s/prometheus/additional-scrape-configs.yaml` to include:

```yaml
- job_name: 'pushgateway'
  honor_labels: true
  static_configs:
    - targets:
      - 'pushgateway.monitoring.svc.cluster.local:9091'
```

Apply it:
```bash
kubectl create secret generic additional-scrape-configs \
  --from-file=/home/paul/git/conf/f3s/prometheus/additional-scrape-configs.yaml \
  --dry-run=client -o yaml -n monitoring | kubectl apply -f -
```

### 3. Run the Standalone Binary

First, port-forward the Pushgateway:
```bash
kubectl port-forward -n monitoring svc/pushgateway 9091:9091
```

In another terminal, run the binary:
```bash
cd /home/paul/git/conf/f3s/prometheus-pusher
./prometheus-pusher
```

The binary will:
- Push metrics immediately on startup
- Continue pushing metrics every 15 seconds
- Generate random example data to simulate a real application

## Viewing Metrics

### View Pushgateway UI
```bash
kubectl port-forward -n monitoring svc/pushgateway 9091:9091
# Open http://localhost:9091
```

### Query Prometheus
```bash
kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090
# Open http://localhost:9090
```

Example queries:
```promql
# View total requests
app_requests_total

# View request rate over last 5 minutes
rate(app_requests_total[5m])

# View current active connections
app_active_connections

# View current temperature
app_temperature_celsius

# View 95th percentile request duration
histogram_quantile(0.95, rate(app_request_duration_seconds_bucket[5m]))

# View failed jobs by type
app_jobs_processed_total{status="failed"}

# View job success rate
rate(app_jobs_processed_total{status="success"}[5m]) / rate(app_jobs_processed_total[5m])
```

## Metric Types Explained

### Counter: `app_requests_total`
- **Type**: Counter
- **Description**: Total number of requests processed
- **Value behavior**: Only increases (monotonically increasing)
- **Use case**: Counting total events, requests, errors

### Gauge: `app_active_connections`, `app_temperature_celsius`
- **Type**: Gauge
- **Description**: Current value that can go up or down
- **Value behavior**: Can increase or decrease
- **Use cases**:
  - Active connections
  - Current temperature
  - Memory usage
  - Queue length

### Histogram: `app_request_duration_seconds`
- **Type**: Histogram
- **Description**: Distribution of request durations
- **Value behavior**: Samples observations into buckets
- **Use cases**:
  - Request latency
  - Response times
  - Data sizes
- **Buckets**: .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10 seconds

### Counter with Labels: `app_jobs_processed_total`
- **Type**: Counter with labels
- **Description**: Jobs processed by type and status
- **Labels**:
  - `job_type`: email, report, backup
  - `status`: success, failed
- **Use cases**: Categorized counting, multi-dimensional metrics

## Prometheus Format Example

The metrics are sent in Prometheus text format:

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

# HELP app_jobs_processed_total Total number of jobs processed by type
# TYPE app_jobs_processed_total counter
app_jobs_processed_total{instance="example-app",job="example_metrics_pusher",job_type="email",status="success"} 15
app_jobs_processed_total{instance="example-app",job="example_metrics_pusher",job_type="email",status="failed"} 2
```

## Customizing the Binary

Edit `main.go` to:
1. Change the Pushgateway URL
2. Modify the push interval (currently 15 seconds)
3. Add your own metrics
4. Change label values

Then rebuild:
```bash
go build -o prometheus-pusher main.go
```

## Architecture

```
┌─────────────────┐
│  Go Binary      │
│  (prometheus-   │──Push metrics──┐
│   pusher)       │                │
└─────────────────┘                │
                                   ▼
                         ┌──────────────────┐
                         │  Pushgateway     │◄──Scrape──┐
                         │  (Port 9091)     │           │
                         └──────────────────┘           │
                                                        │
                                              ┌─────────────────┐
                                              │   Prometheus    │
                                              │   (Port 9090)   │
                                              └─────────────────┘
```

## When to Use Pushgateway vs. Scraping

**Use Pushgateway (what we're doing) for:**
- Batch jobs
- Short-lived processes
- Service-level metrics
- Jobs behind firewalls

**Use Prometheus scraping (alternative approach) for:**
- Long-running applications
- Services with consistent endpoints
- Applications that can expose `/metrics` endpoint

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
curl http://localhost:9091/metrics | grep "app_"

# Check Prometheus scrape targets
# Open http://localhost:9090/targets
# Look for "pushgateway" job

# Check Prometheus logs
kubectl logs -n monitoring -l app.kubernetes.io/name=prometheus
```

### Reload Prometheus config manually
```bash
# The Prometheus Operator should auto-reload, but if needed:
kubectl delete pod -n monitoring -l app.kubernetes.io/name=prometheus
```

## Clean Up

```bash
# Stop port-forwards
pkill -f "port-forward.*9091"
pkill -f "port-forward.*9090"

# Remove deployment (if you want to uninstall)
helm uninstall pushgateway-only -n monitoring
```
