package ingester

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"time"

	"prometheus-pusher/internal/metrics"

	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
)

const (
	requestTimeout = 10 * time.Second
	backfillDelay  = 100 * time.Millisecond
)

// RemoteWriteIngester handles historic metric ingestion via Prometheus Remote Write API.
// This ingester preserves custom timestamps, making it suitable for importing historic data.
type RemoteWriteIngester struct {
	client *http.Client
}

// NewRemoteWriteIngester creates a new Remote Write ingester.
func NewRemoteWriteIngester() RemoteWriteIngester {
	return RemoteWriteIngester{
		client: &http.Client{Timeout: requestTimeout},
	}
}

// Ingest sends samples to Prometheus via Remote Write API.
func (i RemoteWriteIngester) Ingest(ctx context.Context, samples []metrics.Sample, url string) error {
	if len(samples) == 0 {
		return fmt.Errorf("no samples to ingest")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	timeSeries := convertSamplesToTimeSeries(samples)
	writeRequest := &prompb.WriteRequest{Timeseries: timeSeries}

	return i.sendWriteRequest(ctx, url, writeRequest)
}

// IngestHistoric generates and ingests historic metrics for a specific time in the past.
func (i RemoteWriteIngester) IngestHistoric(ctx context.Context, url string, hoursAgo int) error {
	timestamp := time.Now().Add(-time.Duration(hoursAgo) * time.Hour)
	timeSeries := generateHistoricTimeSeries(timestamp)
	writeRequest := &prompb.WriteRequest{Timeseries: timeSeries}

	if err := i.sendWriteRequest(ctx, url, writeRequest); err != nil {
		return err
	}

	log.Printf("Successfully pushed historic data for %d hours ago (timestamp: %s)",
		hoursAgo, timestamp.Format(time.RFC3339))
	return nil
}

// Backfill ingests historic metrics for a range of time points.
func (i RemoteWriteIngester) Backfill(ctx context.Context, url string, startHoursAgo, endHoursAgo, intervalHours int) error {
	log.Printf("Starting backfill from %d hours ago to %d hours ago (interval: %d hours)",
		startHoursAgo, endHoursAgo, intervalHours)

	successCount := 0
	errorCount := 0

	for hoursAgo := startHoursAgo; hoursAgo >= endHoursAgo; hoursAgo -= intervalHours {
		if err := i.IngestHistoric(ctx, url, hoursAgo); err != nil {
			log.Printf("Error pushing data for %d hours ago: %v", hoursAgo, err)
			errorCount++
		} else {
			successCount++
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backfillDelay):
		}
	}

	log.Printf("Backfill complete: %d successful, %d errors", successCount, errorCount)

	if errorCount > 0 {
		return fmt.Errorf("backfill completed with %d errors", errorCount)
	}

	return nil
}

// sendWriteRequest sends a write request to Prometheus.
func (i RemoteWriteIngester) sendWriteRequest(ctx context.Context, url string, writeRequest *prompb.WriteRequest) error {
	data, err := writeRequest.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal write request: %w", err)
	}

	compressed := snappy.Encode(nil, data)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(compressed))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")

	resp, err := i.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send remote write request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("remote write failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// convertSamplesToTimeSeries converts metrics.Sample to prompb.TimeSeries format.
func convertSamplesToTimeSeries(samples []metrics.Sample) []prompb.TimeSeries {
	timeSeries := make([]prompb.TimeSeries, 0, len(samples))

	for _, sample := range samples {
		labels := []prompb.Label{{Name: "__name__", Value: sample.MetricName}}

		for k, v := range sample.Labels {
			labels = append(labels, prompb.Label{Name: k, Value: v})
		}

		timeSeries = append(timeSeries, prompb.TimeSeries{
			Labels: labels,
			Samples: []prompb.Sample{{
				Value:     sample.Value,
				Timestamp: sample.Timestamp.UnixMilli(),
			}},
		})
	}

	return timeSeries
}

// generateHistoricTimeSeries generates example time series for a specific timestamp.
func generateHistoricTimeSeries(timestamp time.Time) []prompb.TimeSeries {
	timestampMs := timestamp.UnixMilli()
	var timeSeries []prompb.TimeSeries

	baseLabels := []prompb.Label{
		{Name: "instance", Value: "example-app"},
		{Name: "job", Value: "historic_data"},
	}

	timeSeries = append(timeSeries, createCounterSeries("prometheus_pusher_test_requests_total", baseLabels, float64(rand.Intn(100)+1), timestampMs))
	timeSeries = append(timeSeries, createGaugeSeries("prometheus_pusher_test_active_connections", baseLabels, float64(rand.Intn(100)), timestampMs))
	timeSeries = append(timeSeries, createGaugeSeries("prometheus_pusher_test_temperature_celsius", baseLabels, 15+rand.Float64()*20, timestampMs))

	timeSeries = append(timeSeries, generateHistogramSeries(baseLabels, timestampMs)...)
	timeSeries = append(timeSeries, generateLabeledCounterSeries(baseLabels, timestampMs)...)

	return timeSeries
}

// createCounterSeries creates a counter metric time series.
func createCounterSeries(name string, baseLabels []prompb.Label, value float64, timestamp int64) prompb.TimeSeries {
	labels := []prompb.Label{{Name: "__name__", Value: name}}
	labels = append(labels, baseLabels...)

	return prompb.TimeSeries{
		Labels:  labels,
		Samples: []prompb.Sample{{Value: value, Timestamp: timestamp}},
	}
}

// createGaugeSeries creates a gauge metric time series.
func createGaugeSeries(name string, baseLabels []prompb.Label, value float64, timestamp int64) prompb.TimeSeries {
	return createCounterSeries(name, baseLabels, value, timestamp)
}

// generateHistogramSeries generates histogram bucket time series.
func generateHistogramSeries(baseLabels []prompb.Label, timestamp int64) []prompb.TimeSeries {
	buckets := []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	var series []prompb.TimeSeries

	cumulativeCount := 0
	for _, bucket := range buckets {
		cumulativeCount += rand.Intn(5)
		labels := []prompb.Label{
			{Name: "__name__", Value: "prometheus_pusher_test_request_duration_seconds_bucket"},
			{Name: "le", Value: fmt.Sprintf("%g", bucket)},
		}
		labels = append(labels, baseLabels...)

		series = append(series, prompb.TimeSeries{
			Labels:  labels,
			Samples: []prompb.Sample{{Value: float64(cumulativeCount), Timestamp: timestamp}},
		})
	}

	infLabels := []prompb.Label{
		{Name: "__name__", Value: "prometheus_pusher_test_request_duration_seconds_bucket"},
		{Name: "le", Value: "+Inf"},
	}
	infLabels = append(infLabels, baseLabels...)
	series = append(series, prompb.TimeSeries{
		Labels:  infLabels,
		Samples: []prompb.Sample{{Value: float64(cumulativeCount), Timestamp: timestamp}},
	})

	series = append(series, createCounterSeries("prometheus_pusher_test_request_duration_seconds_sum", baseLabels, rand.Float64()*100, timestamp))
	series = append(series, createCounterSeries("prometheus_pusher_test_request_duration_seconds_count", baseLabels, float64(cumulativeCount), timestamp))

	return series
}

// generateLabeledCounterSeries generates labeled counter time series.
func generateLabeledCounterSeries(baseLabels []prompb.Label, timestamp int64) []prompb.TimeSeries {
	jobTypes := []string{"email", "report", "backup"}
	statuses := []string{"success", "failed"}
	var series []prompb.TimeSeries

	for _, jobType := range jobTypes {
		for _, status := range statuses {
			labels := []prompb.Label{
				{Name: "__name__", Value: "prometheus_pusher_test_jobs_processed_total"},
				{Name: "job_type", Value: jobType},
				{Name: "status", Value: status},
			}
			labels = append(labels, baseLabels...)

			series = append(series, prompb.TimeSeries{
				Labels:  labels,
				Samples: []prompb.Sample{{Value: float64(rand.Intn(20)), Timestamp: timestamp}},
			})
		}
	}

	return series
}
