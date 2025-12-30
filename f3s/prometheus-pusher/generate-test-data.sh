#!/bin/bash

# Generate test data with actual timestamps for different time ranges

NOW=$(date +%s)000  # Current time in milliseconds
ONE_HOUR_AGO=$((NOW - 3600000))
ONE_DAY_AGO=$((NOW - 86400000))
ONE_WEEK_AGO=$((NOW - 604800000))
ONE_MONTH_AGO=$((NOW - 2592000000))

cat > test-all-ages.csv << EOF
# Prometheus metrics in CSV format demonstrating all time ranges
# Format: metric_name,labels,value,timestamp_ms

# CURRENT data (< 5min old - will use Pushgateway/Realtime)
app_requests_total,instance=current;env=prod,100,$NOW
app_temperature_celsius,instance=current;zone=us-east,22.5,$NOW
app_active_connections,instance=current;env=prod,50,$NOW

# 1 HOUR OLD data (will use Remote Write/Historic)
app_requests_total,instance=1h_ago;env=prod,95,$ONE_HOUR_AGO
app_active_connections,instance=1h_ago;env=prod,45,$ONE_HOUR_AGO
app_temperature_celsius,instance=1h_ago;zone=us-east,21.8,$ONE_HOUR_AGO

# 1 DAY OLD data (will use Remote Write/Historic)
app_requests_total,instance=1d_ago;env=prod,150,$ONE_DAY_AGO
app_temperature_celsius,instance=1d_ago;zone=eu-west,18.3,$ONE_DAY_AGO
app_active_connections,instance=1d_ago;env=prod,60,$ONE_DAY_AGO

# 1 WEEK OLD data (will use Remote Write/Historic)
app_requests_total,instance=1w_ago;env=prod,200,$ONE_WEEK_AGO
app_jobs_processed_total,instance=1w_ago;env=prod;job_type=email;status=success,75,$ONE_WEEK_AGO
app_temperature_celsius,instance=1w_ago;zone=asia,25.2,$ONE_WEEK_AGO

# 1 MONTH OLD data (will use Remote Write/Historic)
app_requests_total,instance=1m_ago;env=prod,180,$ONE_MONTH_AGO
app_active_connections,instance=1m_ago;env=prod,30,$ONE_MONTH_AGO
app_temperature_celsius,instance=1m_ago;zone=africa,28.7,$ONE_MONTH_AGO
EOF

echo "Generated test-all-ages.csv with the following timestamps:"
echo "  Current:  $NOW ($(date -d @$((NOW/1000)) '+%Y-%m-%d %H:%M:%S'))"
echo "  1h ago:   $ONE_HOUR_AGO ($(date -d @$((ONE_HOUR_AGO/1000)) '+%Y-%m-%d %H:%M:%S'))"
echo "  1d ago:   $ONE_DAY_AGO ($(date -d @$((ONE_DAY_AGO/1000)) '+%Y-%m-%d %H:%M:%S'))"
echo "  1w ago:   $ONE_WEEK_AGO ($(date -d @$((ONE_WEEK_AGO/1000)) '+%Y-%m-%d %H:%M:%S'))"
echo "  1m ago:   $ONE_MONTH_AGO ($(date -d @$((ONE_MONTH_AGO/1000)) '+%Y-%m-%d %H:%M:%S'))"
