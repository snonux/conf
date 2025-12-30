#!/bin/bash
# Deploy Prometheus Pusher Test Metrics dashboard to Grafana

set -e

GRAFANA_URL="${GRAFANA_URL:-http://localhost:3000}"
GRAFANA_USER="${GRAFANA_USER:-admin}"
GRAFANA_PASSWORD="${GRAFANA_PASSWORD:-admin}"

echo "=== Deploying Prometheus Pusher Test Metrics Dashboard ==="
echo "Grafana URL: $GRAFANA_URL"
echo ""

# Check if Grafana is accessible
if ! curl -sf "${GRAFANA_URL}/api/health" > /dev/null; then
    echo "Error: Cannot reach Grafana at $GRAFANA_URL"
    echo "Make sure Grafana is running and accessible"
    echo ""
    echo "If running in Kubernetes, port-forward first:"
    echo "  kubectl port-forward -n monitoring svc/prometheus-grafana 3000:80"
    exit 1
fi

echo "✅ Grafana is accessible"
echo ""

# Import dashboard
echo "Importing dashboard..."
RESPONSE=$(curl -sf -X POST \
    -H "Content-Type: application/json" \
    -u "${GRAFANA_USER}:${GRAFANA_PASSWORD}" \
    -d @grafana-dashboard.json \
    "${GRAFANA_URL}/api/dashboards/db")

if [ $? -eq 0 ]; then
    DASHBOARD_UID=$(echo "$RESPONSE" | grep -o '"uid":"[^"]*"' | cut -d'"' -f4)
    DASHBOARD_URL="${GRAFANA_URL}/d/${DASHBOARD_UID}/prometheus-pusher-test-metrics"

    echo "✅ Dashboard imported successfully!"
    echo ""
    echo "Dashboard URL: $DASHBOARD_URL"
    echo ""
    echo "The dashboard shows:"
    echo "  - Request rate and total requests"
    echo "  - Active connections gauge"
    echo "  - Temperature gauge"
    echo "  - Request duration percentiles (p50, p90, p99)"
    echo "  - Average request duration"
    echo "  - Jobs processed by type"
    echo "  - Jobs status breakdown table"
else
    echo "❌ Failed to import dashboard"
    echo "Response: $RESPONSE"
    exit 1
fi
