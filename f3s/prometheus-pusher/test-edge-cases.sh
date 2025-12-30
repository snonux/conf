#!/bin/bash

# Test edge cases for Prometheus ingestion limits

set +e  # Don't exit on errors - we expect some to fail

NOW=$(date +%s)000

echo "=================================================="
echo "Testing Prometheus Ingestion Edge Cases"
echo "=================================================="
echo ""
echo "Current time: $(date -d @$((NOW/1000)) '+%Y-%m-%d %H:%M:%S')"
echo ""

# Generate test data for various edge cases
cat > test-edge-cases.csv << EOF
# Edge case tests for Prometheus ingestion

# RECENT/CURRENT - Should all work
app_edge_now,instance=now,100,$NOW
app_edge_1min_ago,instance=1min_ago,100,$((NOW - 60000))
app_edge_5min_ago,instance=5min_ago,100,$((NOW - 300000))

# FUTURE - Will likely be rejected
app_edge_1min_future,instance=1min_future,100,$((NOW + 60000))
app_edge_10min_future,instance=10min_future,100,$((NOW + 600000))
app_edge_1h_future,instance=1h_future,100,$((NOW + 3600000))

# PAST - Testing various ages
app_edge_1h_old,instance=1h_old,100,$((NOW - 3600000))
app_edge_1d_old,instance=1d_old,100,$((NOW - 86400000))
app_edge_1w_old,instance=1w_old,100,$((NOW - 604800000))
app_edge_1m_old,instance=1m_old,100,$((NOW - 2592000000))
app_edge_2m_old,instance=2m_old,100,$((NOW - 5184000000))
app_edge_3m_old,instance=3m_old,100,$((NOW - 7776000000))
app_edge_6m_old,instance=6m_old,100,$((NOW - 15552000000))
app_edge_1y_old,instance=1y_old,100,$((NOW - 31536000000))
app_edge_2y_old,instance=2y_old,100,$((NOW - 63072000000))
EOF

echo "Generated test data with following timestamps:"
echo "  Now:        $(date -d @$((NOW/1000)) '+%Y-%m-%d %H:%M:%S')"
echo "  1min future: $(date -d @$(((NOW + 60000)/1000)) '+%Y-%m-%d %H:%M:%S')"
echo "  1h future:  $(date -d @$(((NOW + 3600000)/1000)) '+%Y-%m-%d %H:%M:%S')"
echo "  1h ago:     $(date -d @$(((NOW - 3600000)/1000)) '+%Y-%m-%d %H:%M:%S')"
echo "  1d ago:     $(date -d @$(((NOW - 86400000)/1000)) '+%Y-%m-%d %H:%M:%S')"
echo "  1w ago:     $(date -d @$(((NOW - 604800000)/1000)) '+%Y-%m-%d %H:%M:%S')"
echo "  1m ago:     $(date -d @$(((NOW - 2592000000)/1000)) '+%Y-%m-%d %H:%M:%S')"
echo "  2m ago:     $(date -d @$(((NOW - 5184000000)/1000)) '+%Y-%m-%d %H:%M:%S')"
echo "  3m ago:     $(date -d @$(((NOW - 7776000000)/1000)) '+%Y-%m-%d %H:%M:%S')"
echo "  6m ago:     $(date -d @$(((NOW - 15552000000)/1000)) '+%Y-%m-%d %H:%M:%S')"
echo "  1y ago:     $(date -d @$(((NOW - 31536000000)/1000)) '+%Y-%m-%d %H:%M:%S')"
echo "  2y ago:     $(date -d @$(((NOW - 63072000000)/1000)) '+%Y-%m-%d %H:%M:%S')"
echo ""

echo "=================================================="
echo "IMPORTANT: Port-forward Prometheus before running this test:"
echo "  kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090 &"
echo "=================================================="
echo ""

read -p "Press Enter to start test (or Ctrl+C to cancel)..."

echo ""
echo "Running ingestion test..."
echo ""

./prometheus-pusher \
  -mode=auto \
  -file=test-edge-cases.csv \
  -prometheus=http://localhost:9090/api/v1/write \
  2>&1 | tee test-edge-cases.log

echo ""
echo "=================================================="
echo "Test Results Summary"
echo "=================================================="
echo ""

# Analyze results
if grep -q "Successfully ingested" test-edge-cases.log; then
    echo "✅ Some samples were successfully ingested"
    SUCCESS=$(grep -o "Successfully ingested [0-9]* historic samples" test-edge-cases.log | grep -o "[0-9]*")
    echo "   Success count: $SUCCESS samples"
else
    echo "❌ No samples were successfully ingested"
fi

echo ""

if grep -qi "error\|failed\|rejected" test-edge-cases.log; then
    echo "❌ Some samples were rejected:"
    grep -i "error\|failed\|rejected" test-edge-cases.log | head -10
else
    echo "✅ No errors detected"
fi

echo ""
echo "Full log saved to: test-edge-cases.log"
echo ""

# Check what data made it into Prometheus
echo "=================================================="
echo "Querying Prometheus for Successfully Imported Data"
echo "=================================================="
echo ""

sleep 2  # Give Prometheus time to process

for age in "now" "1min_ago" "5min_ago" "1min_future" "10min_future" "1h_future" \
           "1h_old" "1d_old" "1w_old" "1m_old" "2m_old" "3m_old" "6m_old" "1y_old" "2y_old"; do
    result=$(curl -s "http://localhost:9090/api/v1/query?query=app_edge_$age" | \
             python3 -c "import sys, json; d=json.load(sys.stdin); print('✅ Found' if d['data']['result'] else '❌ Not found')" 2>/dev/null || echo "⚠️  Query failed")
    printf "  %-15s : %s\n" "$age" "$result"
done

echo ""
echo "=================================================="
echo "Conclusion"
echo "=================================================="
echo ""
echo "Check the results above to see which time ranges work."
echo "Generally:"
echo "  ✅ Current to 1 month:  Should work"
echo "  ⚠️  2-3 months:         Depends on retention settings"
echo "  ❌ 6+ months:          Likely rejected"
echo "  ❌ Years old:          Definitely rejected"
echo "  ❌ Future timestamps:  Rejected (except maybe 1-2 min)"
echo ""
