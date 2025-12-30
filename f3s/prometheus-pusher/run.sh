#!/bin/bash

# Simple script to run the prometheus pusher
# Automatically sets up port-forwarding and runs the binary

set -e

echo "Starting Prometheus Pusher..."
echo ""
echo "Step 1: Setting up port-forward to Pushgateway..."
kubectl port-forward -n monitoring svc/pushgateway 9091:9091 > /tmp/pushgateway-port-forward.log 2>&1 &
PF_PID=$!

# Wait for port-forward to be ready
sleep 2

echo "Step 2: Running prometheus-pusher binary..."
echo "Press Ctrl+C to stop"
echo ""

# Run the binary and capture its exit status
./prometheus-pusher
EXIT_CODE=$?

# Clean up port-forward
echo ""
echo "Cleaning up port-forward..."
kill $PF_PID 2>/dev/null || true

echo "Done!"
exit $EXIT_CODE
