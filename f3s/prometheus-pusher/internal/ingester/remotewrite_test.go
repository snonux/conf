package ingester

import (
	"testing"
	"time"

	"prometheus-pusher/internal/metrics"

	"github.com/prometheus/prometheus/prompb"
)

func TestNewRemoteWriteIngester(t *testing.T) {
	ingester := NewRemoteWriteIngester()
	if ingester.client == nil {
		t.Error("RemoteWriteIngester.client should not be nil")
	}
}

func TestConvertSamplesToTimeSeries(t *testing.T) {
	now := time.Now()
	samples := []metrics.Sample{
		{
			MetricName: "test_metric1",
			Labels:     map[string]string{"env": "prod", "host": "server1"},
			Value:      42.5,
			Timestamp:  now,
		},
		{
			MetricName: "test_metric2",
			Labels:     map[string]string{"env": "test"},
			Value:      100.0,
			Timestamp:  now.Add(-1 * time.Hour),
		},
	}

	timeSeries := convertSamplesToTimeSeries(samples)

	if len(timeSeries) != 2 {
		t.Errorf("Expected 2 time series, got %d", len(timeSeries))
	}

	// Check first time series
	ts1 := timeSeries[0]
	if len(ts1.Labels) != 3 { // __name__ + 2 custom labels
		t.Errorf("Expected 3 labels, got %d", len(ts1.Labels))
	}

	hasName := false
	for _, label := range ts1.Labels {
		if label.Name == "__name__" && label.Value == "test_metric1" {
			hasName = true
		}
	}
	if !hasName {
		t.Error("Missing or incorrect __name__ label")
	}

	if len(ts1.Samples) != 1 {
		t.Errorf("Expected 1 sample, got %d", len(ts1.Samples))
	}
	if ts1.Samples[0].Value != 42.5 {
		t.Errorf("Expected value 42.5, got %f", ts1.Samples[0].Value)
	}
}

func TestGenerateHistoricTimeSeries(t *testing.T) {
	timestamp := time.Now().Add(-24 * time.Hour)

	timeSeries := generateHistoricTimeSeries(timestamp)

	if len(timeSeries) == 0 {
		t.Error("Expected time series to be generated")
	}

	// Should contain various metric types
	metricNames := make(map[string]bool)
	for _, ts := range timeSeries {
		for _, label := range ts.Labels {
			if label.Name == "__name__" {
				metricNames[label.Value] = true
			}
		}
	}

	expectedMetrics := []string{
		"prometheus_pusher_test_requests_total",
		"prometheus_pusher_test_active_connections",
		"prometheus_pusher_test_temperature_celsius",
		"prometheus_pusher_test_jobs_processed_total",
	}

	for _, expected := range expectedMetrics {
		if !metricNames[expected] {
			t.Errorf("Expected metric %s not found", expected)
		}
	}
}

func TestCreateCounterSeries(t *testing.T) {
	baseLabels := []prompb.Label{
		{Name: "instance", Value: "test-instance"},
		{Name: "job", Value: "test-job"},
	}

	ts := createCounterSeries("test_counter", baseLabels, 123.45, 1234567890000)

	if len(ts.Labels) != 3 { // __name__ + 2 base labels
		t.Errorf("Expected 3 labels, got %d", len(ts.Labels))
	}

	if len(ts.Samples) != 1 {
		t.Errorf("Expected 1 sample, got %d", len(ts.Samples))
	}

	if ts.Samples[0].Value != 123.45 {
		t.Errorf("Expected value 123.45, got %f", ts.Samples[0].Value)
	}

	if ts.Samples[0].Timestamp != 1234567890000 {
		t.Errorf("Expected timestamp 1234567890000, got %d", ts.Samples[0].Timestamp)
	}
}

func TestCreateGaugeSeries(t *testing.T) {
	baseLabels := []prompb.Label{
		{Name: "instance", Value: "test-instance"},
	}

	ts := createGaugeSeries("test_gauge", baseLabels, 67.89, 9876543210000)

	if len(ts.Samples) != 1 {
		t.Errorf("Expected 1 sample, got %d", len(ts.Samples))
	}

	if ts.Samples[0].Value != 67.89 {
		t.Errorf("Expected value 67.89, got %f", ts.Samples[0].Value)
	}
}

func TestGenerateHistogramSeries(t *testing.T) {
	baseLabels := []prompb.Label{
		{Name: "instance", Value: "test-instance"},
	}
	timestamp := int64(1234567890000)

	series := generateHistogramSeries(baseLabels, timestamp)

	if len(series) == 0 {
		t.Error("Expected histogram series to be generated")
	}

	// Should contain buckets, +Inf, sum, and count
	metricTypes := make(map[string]int)
	for _, ts := range series {
		for _, label := range ts.Labels {
			if label.Name == "__name__" {
				metricTypes[label.Value]++
			}
		}
	}

	if metricTypes["prometheus_pusher_test_request_duration_seconds_bucket"] == 0 {
		t.Error("Expected histogram buckets")
	}
	if metricTypes["prometheus_pusher_test_request_duration_seconds_sum"] != 1 {
		t.Error("Expected histogram sum")
	}
	if metricTypes["prometheus_pusher_test_request_duration_seconds_count"] != 1 {
		t.Error("Expected histogram count")
	}
}

func TestGenerateLabeledCounterSeries(t *testing.T) {
	baseLabels := []prompb.Label{
		{Name: "instance", Value: "test-instance"},
	}
	timestamp := int64(1234567890000)

	series := generateLabeledCounterSeries(baseLabels, timestamp)

	if len(series) == 0 {
		t.Error("Expected labeled counter series to be generated")
	}

	// Should have combinations of job types and statuses
	// 3 job types * 2 statuses = 6 series
	if len(series) != 6 {
		t.Errorf("Expected 6 labeled counter series, got %d", len(series))
	}

	// Verify label structure
	for _, ts := range series {
		hasJobType := false
		hasStatus := false
		for _, label := range ts.Labels {
			if label.Name == "job_type" {
				hasJobType = true
			}
			if label.Name == "status" {
				hasStatus = true
			}
		}
		if !hasJobType {
			t.Error("Expected job_type label")
		}
		if !hasStatus {
			t.Error("Expected status label")
		}
	}
}
