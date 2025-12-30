package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
)

// GenerateHistoricMetrics generates metric samples for a specific time in the past
// hoursAgo: how many hours in the past to generate data for
func GenerateHistoricMetrics(hoursAgo int) []prompb.TimeSeries {
	timestamp := time.Now().Add(-time.Duration(hoursAgo) * time.Hour).UnixMilli()

	var timeSeries []prompb.TimeSeries

	// Counter: app_requests_total
	timeSeries = append(timeSeries, prompb.TimeSeries{
		Labels: []prompb.Label{
			{Name: "__name__", Value: "app_requests_total"},
			{Name: "instance", Value: "example-app"},
			{Name: "job", Value: "historic_data"},
		},
		Samples: []prompb.Sample{
			{Value: float64(rand.Intn(100) + 1), Timestamp: timestamp},
		},
	})

	// Gauge: app_active_connections
	timeSeries = append(timeSeries, prompb.TimeSeries{
		Labels: []prompb.Label{
			{Name: "__name__", Value: "app_active_connections"},
			{Name: "instance", Value: "example-app"},
			{Name: "job", Value: "historic_data"},
		},
		Samples: []prompb.Sample{
			{Value: float64(rand.Intn(100)), Timestamp: timestamp},
		},
	})

	// Gauge: app_temperature_celsius
	timeSeries = append(timeSeries, prompb.TimeSeries{
		Labels: []prompb.Label{
			{Name: "__name__", Value: "app_temperature_celsius"},
			{Name: "instance", Value: "example-app"},
			{Name: "job", Value: "historic_data"},
		},
		Samples: []prompb.Sample{
			{Value: 15 + rand.Float64()*20, Timestamp: timestamp},
		},
	})

	// Histogram buckets: app_request_duration_seconds
	buckets := []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	cumulativeCount := 0
	for _, bucket := range buckets {
		cumulativeCount += rand.Intn(5)
		timeSeries = append(timeSeries, prompb.TimeSeries{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "app_request_duration_seconds_bucket"},
				{Name: "instance", Value: "example-app"},
				{Name: "job", Value: "historic_data"},
				{Name: "le", Value: fmt.Sprintf("%g", bucket)},
			},
			Samples: []prompb.Sample{
				{Value: float64(cumulativeCount), Timestamp: timestamp},
			},
		})
	}

	// +Inf bucket
	timeSeries = append(timeSeries, prompb.TimeSeries{
		Labels: []prompb.Label{
			{Name: "__name__", Value: "app_request_duration_seconds_bucket"},
			{Name: "instance", Value: "example-app"},
			{Name: "job", Value: "historic_data"},
			{Name: "le", Value: "+Inf"},
		},
		Samples: []prompb.Sample{
			{Value: float64(cumulativeCount), Timestamp: timestamp},
		},
	})

	// Histogram sum
	timeSeries = append(timeSeries, prompb.TimeSeries{
		Labels: []prompb.Label{
			{Name: "__name__", Value: "app_request_duration_seconds_sum"},
			{Name: "instance", Value: "example-app"},
			{Name: "job", Value: "historic_data"},
		},
		Samples: []prompb.Sample{
			{Value: rand.Float64() * 100, Timestamp: timestamp},
		},
	})

	// Histogram count
	timeSeries = append(timeSeries, prompb.TimeSeries{
		Labels: []prompb.Label{
			{Name: "__name__", Value: "app_request_duration_seconds_count"},
			{Name: "instance", Value: "example-app"},
			{Name: "job", Value: "historic_data"},
		},
		Samples: []prompb.Sample{
			{Value: float64(cumulativeCount), Timestamp: timestamp},
		},
	})

	// Labeled counters: app_jobs_processed_total
	jobTypes := []string{"email", "report", "backup"}
	statuses := []string{"success", "failed"}
	for _, jobType := range jobTypes {
		for _, status := range statuses {
			timeSeries = append(timeSeries, prompb.TimeSeries{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "app_jobs_processed_total"},
					{Name: "instance", Value: "example-app"},
					{Name: "job", Value: "historic_data"},
					{Name: "job_type", Value: jobType},
					{Name: "status", Value: status},
				},
				Samples: []prompb.Sample{
					{Value: float64(rand.Intn(20)), Timestamp: timestamp},
				},
			})
		}
	}

	return timeSeries
}

// PushHistoricData sends historic data to Prometheus via Remote Write API
// prometheusURL: URL of Prometheus remote write endpoint (e.g., "http://localhost:9090/api/v1/write")
// hoursAgo: how many hours in the past to generate data for
func PushHistoricData(prometheusURL string, hoursAgo int) error {
	// Generate historic metrics
	timeSeries := GenerateHistoricMetrics(hoursAgo)

	// Create write request
	writeRequest := &prompb.WriteRequest{
		Timeseries: timeSeries,
	}

	// Marshal to protobuf
	data, err := writeRequest.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal write request: %w", err)
	}

	// Compress with snappy
	compressed := snappy.Encode(nil, data)

	// Send HTTP POST request
	req, err := http.NewRequest("POST", prometheusURL, bytes.NewReader(compressed))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send remote write request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("remote write failed with status %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("Successfully pushed historic data for %d hours ago (timestamp: %s)",
		hoursAgo, time.Now().Add(-time.Duration(hoursAgo)*time.Hour).Format(time.RFC3339))

	return nil
}

// BackfillHistoricData backfills data for multiple time points
// prometheusURL: URL of Prometheus remote write endpoint
// startHoursAgo: how many hours ago to start backfilling
// endHoursAgo: how many hours ago to end backfilling
// intervalHours: interval between data points in hours
func BackfillHistoricData(prometheusURL string, startHoursAgo, endHoursAgo, intervalHours int) error {
	log.Printf("Starting backfill from %d hours ago to %d hours ago (interval: %d hours)",
		startHoursAgo, endHoursAgo, intervalHours)

	successCount := 0
	errorCount := 0

	for hoursAgo := startHoursAgo; hoursAgo >= endHoursAgo; hoursAgo -= intervalHours {
		if err := PushHistoricData(prometheusURL, hoursAgo); err != nil {
			log.Printf("Error pushing data for %d hours ago: %v", hoursAgo, err)
			errorCount++
		} else {
			successCount++
		}

		// Small delay to avoid overwhelming Prometheus
		time.Sleep(100 * time.Millisecond)
	}

	log.Printf("Backfill complete: %d successful, %d errors", successCount, errorCount)

	if errorCount > 0 {
		return fmt.Errorf("backfill completed with %d errors", errorCount)
	}

	return nil
}
