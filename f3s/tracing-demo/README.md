# Tracing Demo Application

Three-tier Python Flask application demonstrating distributed tracing with OpenTelemetry and Grafana Tempo.

## Overview

This demo application shows how distributed tracing works across multiple microservices:

- **Frontend**: Receives HTTP requests, forwards to middleware
- **Middleware**: Transforms data, calls backend
- **Backend**: Returns data (simulates database queries)

Each service is instrumented with OpenTelemetry and sends traces to Grafana Tempo via Alloy.

## Architecture

```
User → Frontend (Flask:5000) → Middleware (Flask:5001) → Backend (Flask:5002)
           ↓                          ↓                        ↓
                    Alloy (OTLP:4317) → Tempo → Grafana
```

## Components

### Frontend Service
- Port: 5000
- Endpoints:
  - `GET /` - Service info and health
  - `GET /health` - Kubernetes health probe
  - `GET|POST /api/process` - Main processing endpoint
- Calls: Middleware service

### Middleware Service
- Port: 5001
- Endpoints:
  - `GET /` - Service info and health
  - `GET /health` - Kubernetes health probe
  - `POST /api/transform` - Data transformation endpoint
- Calls: Backend service

### Backend Service
- Port: 5002
- Endpoints:
  - `GET /` - Service info and health
  - `GET /health` - Kubernetes health probe
  - `GET /api/data` - Data retrieval endpoint (simulates DB query)
- Calls: None (leaf service)

## OpenTelemetry Instrumentation

All services use:
- **Auto-instrumentation**: Flask and Requests libraries automatically create spans
- **Manual spans**: Custom spans for business logic with attributes
- **OTLP export**: Traces sent to Alloy via gRPC on port 4317
- **Resource attributes**: Service name, namespace, version identify each service

## Build and Deploy

### Prerequisites

1. Tempo must be deployed and running in `monitoring` namespace
2. Alloy must be configured with OTLP receivers
3. Docker installed for building images
4. Access to k3s cluster (SSH to r0)

### Quick Start

```bash
# Build Docker images
just build

# Import images to k3s
just import

# Deploy with Helm
just install

# Check status
just status
```

### Rebuild and Update

```bash
# Rebuild images, import, and upgrade deployment
just rebuild
```

## Testing

### Basic Test

```bash
# Test health endpoint
curl http://tracing-demo.f3s.buetow.org/

# Test API endpoint (generates a trace)
curl http://tracing-demo.f3s.buetow.org/api/process
```

### Load Test

Generate 50 requests to create multiple traces:

```bash
just load-test
```

### View Logs

```bash
# View logs from all services
just logs

# Follow frontend logs
just logs-frontend

# Follow middleware logs
just logs-middleware

# Follow backend logs
just logs-backend
```

## Viewing Traces in Grafana

1. Navigate to Grafana: https://grafana.f3s.buetow.org
2. Go to Explore → Select "Tempo" datasource
3. Use TraceQL queries:

```
# All traces from demo app
{ resource.service.namespace = "tracing-demo" }

# Slow requests (>200ms)
{ duration > 200ms }

# Traces from specific service
{ resource.service.name = "frontend" }

# Errors
{ status = error }
```

4. View Service Graph to see connections between services

## Trace Features Demonstrated

### Distributed Context Propagation
Traces automatically span all three services, showing:
- Frontend span (root)
- Middleware span (child of frontend)
- Backend span (child of middleware)

### Custom Attributes
Each service adds custom attributes:
- `service.name` - Service identifier
- `service.namespace` - Application namespace
- Custom business logic attributes

### Trace Correlation
- **Traces-to-Logs**: Click on a span to see related logs in Loki
- **Traces-to-Metrics**: View Prometheus metrics for services in the trace
- **Service Graph**: Visualize service dependencies

## Development

### Local Testing with Port Forwarding

```bash
# Forward frontend
just port-forward-frontend
curl http://localhost:5000/

# Forward middleware
just port-forward-middleware
curl http://localhost:5001/

# Forward backend
just port-forward-backend
curl http://localhost:5002/
```

### Modifying the Application

1. Edit Python code in `docker/*/app.py`
2. Rebuild: `just build`
3. Import: `just import`
4. Upgrade: `just upgrade`

Or use the combined command: `just rebuild`

## Troubleshooting

### No traces appearing in Grafana

1. Check pods are running:
```bash
kubectl get pods -n services | grep tracing-demo
```

2. Check Alloy is receiving traces:
```bash
kubectl logs -n monitoring -l app.kubernetes.io/name=alloy | grep -i otlp
```

3. Check Tempo is storing traces:
```bash
kubectl logs -n monitoring -l app.kubernetes.io/name=tempo | grep -i trace
```

4. Verify OTLP endpoint is accessible:
```bash
kubectl exec -n services $(kubectl get pod -n services -l app=tracing-demo-frontend -o jsonpath='{.items[0].metadata.name}') -- wget -qO- http://alloy.monitoring.svc.cluster.local:4317
```

### Pods not starting

Check events and logs:
```bash
kubectl describe pod -n services -l app=tracing-demo-frontend
kubectl logs -n services -l app=tracing-demo-frontend
```

### Images not found

Verify images are imported to k3s:
```bash
ssh r0 'k3s crictl images | grep tracing-demo'
```

If missing, run:
```bash
just import
```

## Cleanup

Remove the demo application:

```bash
just delete
```

## References

- [OpenTelemetry Python Documentation](https://opentelemetry.io/docs/languages/python/)
- [Flask Instrumentation](https://opentelemetry-python-contrib.readthedocs.io/en/latest/instrumentation/flask/flask.html)
- [Grafana Tempo Documentation](https://grafana.com/docs/tempo/latest/)
- [TraceQL Query Language](https://grafana.com/docs/tempo/latest/traceql/)
