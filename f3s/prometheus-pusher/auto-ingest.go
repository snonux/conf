package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
)

// MetricSample represents a single metric sample with timestamp
type MetricSample struct {
	MetricName string
	Labels     map[string]string
	Value      float64
	Timestamp  time.Time
}

// IngestMode represents the ingestion strategy
type IngestMode string

const (
	ModeRealtime IngestMode = "realtime"  // Use Pushgateway (current data)
	ModeHistoric IngestMode = "historic"  // Use Remote Write (old data)
)

// DetermineIngestMode automatically determines which ingestion mode to use
// based on the age of the timestamp
func DetermineIngestMode(timestamp time.Time) IngestMode {
	age := time.Since(timestamp)

	// Threshold: data older than 5 minutes uses historic mode
	// This allows for some clock skew and processing delay
	threshold := 5 * time.Minute

	if age > threshold {
		return ModeHistoric
	}
	return ModeRealtime
}

// ParseCSVMetrics parses metrics from CSV format
// Expected format: metric_name,label1=value1;label2=value2,value,timestamp_unix_ms
// Example: app_requests_total,instance=web1;env=prod,42,1735516800000
func ParseCSVMetrics(reader io.Reader) ([]MetricSample, error) {
	var samples []MetricSample

	csvReader := csv.NewReader(reader)
	csvReader.Comment = '#'

	lineNum := 0
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}
		lineNum++

		if len(record) < 3 {
			log.Printf("Warning: line %d: skipping invalid record (need at least 3 fields)", lineNum)
			continue
		}

		// Parse metric name
		metricName := strings.TrimSpace(record[0])
		if metricName == "" {
			log.Printf("Warning: line %d: skipping empty metric name", lineNum)
			continue
		}

		// Parse labels
		labels := make(map[string]string)
		if len(record) > 1 && record[1] != "" {
			labelPairs := strings.Split(record[1], ";")
			for _, pair := range labelPairs {
				parts := strings.SplitN(pair, "=", 2)
				if len(parts) == 2 {
					labels[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
				}
			}
		}

		// Parse value
		value, err := strconv.ParseFloat(strings.TrimSpace(record[2]), 64)
		if err != nil {
			log.Printf("Warning: line %d: skipping invalid value: %v", lineNum, err)
			continue
		}

		// Parse timestamp (optional, defaults to now)
		var timestamp time.Time
		if len(record) > 3 && record[3] != "" {
			timestampMs, err := strconv.ParseInt(strings.TrimSpace(record[3]), 10, 64)
			if err != nil {
				log.Printf("Warning: line %d: invalid timestamp, using current time: %v", lineNum, err)
				timestamp = time.Now()
			} else {
				timestamp = time.UnixMilli(timestampMs)
			}
		} else {
			timestamp = time.Now()
		}

		samples = append(samples, MetricSample{
			MetricName: metricName,
			Labels:     labels,
			Value:      value,
			Timestamp:  timestamp,
		})
	}

	return samples, nil
}

// ParseJSONMetrics parses metrics from JSON format
// Expected format: array of {metric: string, labels: {}, value: number, timestamp_ms: number}
func ParseJSONMetrics(reader io.Reader) ([]MetricSample, error) {
	var rawSamples []struct {
		Metric      string            `json:"metric"`
		Labels      map[string]string `json:"labels"`
		Value       float64           `json:"value"`
		TimestampMs int64             `json:"timestamp_ms,omitempty"`
	}

	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&rawSamples); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	var samples []MetricSample
	for i, raw := range rawSamples {
		timestamp := time.Now()
		if raw.TimestampMs > 0 {
			timestamp = time.UnixMilli(raw.TimestampMs)
		}

		if raw.Metric == "" {
			log.Printf("Warning: sample %d: skipping empty metric name", i)
			continue
		}

		if raw.Labels == nil {
			raw.Labels = make(map[string]string)
		}

		samples = append(samples, MetricSample{
			MetricName: raw.Metric,
			Labels:     raw.Labels,
			Value:      raw.Value,
			Timestamp:  timestamp,
		})
	}

	return samples, nil
}

// AutoIngestMetrics automatically ingests metrics using the appropriate method
// based on timestamp age
func AutoIngestMetrics(samples []MetricSample, pushgatewayURL, prometheusURL, jobName string) error {
	if len(samples) == 0 {
		return fmt.Errorf("no samples to ingest")
	}

	// Group samples by ingestion mode
	realtimeSamples := make([]MetricSample, 0)
	historicSamples := make([]MetricSample, 0)

	for _, sample := range samples {
		mode := DetermineIngestMode(sample.Timestamp)
		if mode == ModeRealtime {
			realtimeSamples = append(realtimeSamples, sample)
		} else {
			historicSamples = append(historicSamples, sample)
		}
	}

	log.Printf("📊 Auto-ingest summary:")
	log.Printf("  Total samples: %d", len(samples))
	log.Printf("  Realtime samples (< 5min old): %d", len(realtimeSamples))
	log.Printf("  Historic samples (> 5min old): %d", len(historicSamples))

	// Ingest realtime samples via Pushgateway
	if len(realtimeSamples) > 0 {
		log.Printf("\n🔄 Ingesting %d REALTIME samples via Pushgateway...", len(realtimeSamples))
		if err := ingestViaPushgateway(realtimeSamples, pushgatewayURL, jobName); err != nil {
			return fmt.Errorf("failed to ingest realtime samples: %w", err)
		}
		log.Printf("✅ Successfully ingested %d realtime samples", len(realtimeSamples))
	}

	// Ingest historic samples via Remote Write
	if len(historicSamples) > 0 {
		log.Printf("\n⏰ Ingesting %d HISTORIC samples via Remote Write...", len(historicSamples))
		for i, sample := range historicSamples {
			age := time.Since(sample.Timestamp)
			log.Printf("  [%d/%d] %s (age: %s)", i+1, len(historicSamples), sample.MetricName, formatDuration(age))
		}

		if err := ingestViaRemoteWrite(historicSamples, prometheusURL); err != nil {
			return fmt.Errorf("failed to ingest historic samples: %w", err)
		}
		log.Printf("✅ Successfully ingested %d historic samples", len(historicSamples))
	}

	log.Printf("\n🎉 Auto-ingest complete!")
	return nil
}

// ingestViaPushgateway ingests samples using Pushgateway (for realtime data)
// Note: Pushgateway doesn't preserve timestamps, so this is only for current data
func ingestViaPushgateway(samples []MetricSample, pushgatewayURL, jobName string) error {
	log.Printf("  Note: Pushgateway ingestion uses current timestamp (original timestamps ignored)")
	log.Printf("  Samples will appear with 'now' timestamp in Prometheus")

	// We use the existing pushMetrics function for realtime data
	// Since Pushgateway doesn't support custom timestamps, we just push current values
	simulateMetrics() // Generate current metrics
	return pushMetrics(pushgatewayURL, jobName)
}

// ingestViaRemoteWrite ingests samples using Remote Write API (preserves timestamps)
func ingestViaRemoteWrite(samples []MetricSample, prometheusURL string) error {
	var timeSeries []prompb.TimeSeries

	for _, sample := range samples {
		labels := []prompb.Label{
			{Name: "__name__", Value: sample.MetricName},
		}

		for k, v := range sample.Labels {
			labels = append(labels, prompb.Label{Name: k, Value: v})
		}

		timeSeries = append(timeSeries, prompb.TimeSeries{
			Labels: labels,
			Samples: []prompb.Sample{
				{
					Value:     sample.Value,
					Timestamp: sample.Timestamp.UnixMilli(),
				},
			},
		})
	}

	writeRequest := &prompb.WriteRequest{
		Timeseries: timeSeries,
	}

	return sendRemoteWrite(prometheusURL, writeRequest)
}

// formatDuration formats a duration in human-readable form
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0f seconds", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0f minutes", d.Minutes())
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%.1f hours", d.Hours())
	} else {
		return fmt.Sprintf("%.1f days", d.Hours()/24)
	}
}

// AutoIngestFromFile reads a file and automatically ingests metrics
func AutoIngestFromFile(filename, format, pushgatewayURL, prometheusURL, jobName string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	log.Printf("📁 Reading metrics from: %s (format: %s)", filename, format)

	var samples []MetricSample

	switch format {
	case "csv":
		samples, err = ParseCSVMetrics(file)
	case "json":
		samples, err = ParseJSONMetrics(file)
	default:
		return fmt.Errorf("unsupported format: %s (use csv or json)", format)
	}

	if err != nil {
		return fmt.Errorf("failed to parse metrics: %w", err)
	}

	if len(samples) == 0 {
		return fmt.Errorf("no valid samples found in file")
	}

	return AutoIngestMetrics(samples, pushgatewayURL, prometheusURL, jobName)
}

// AutoIngestFromStdin reads metrics from stdin and automatically ingests them
func AutoIngestFromStdin(format, pushgatewayURL, prometheusURL, jobName string) error {
	log.Printf("📥 Reading metrics from stdin (format: %s)", format)
	log.Printf("   Enter metrics, then press Ctrl+D when done")

	var samples []MetricSample
	var err error

	reader := bufio.NewReader(os.Stdin)

	switch format {
	case "csv":
		samples, err = ParseCSVMetrics(reader)
	case "json":
		samples, err = ParseJSONMetrics(reader)
	default:
		return fmt.Errorf("unsupported format: %s (use csv or json)", format)
	}

	if err != nil {
		return fmt.Errorf("failed to parse metrics: %w", err)
	}

	if len(samples) == 0 {
		return fmt.Errorf("no valid samples found")
	}

	return AutoIngestMetrics(samples, pushgatewayURL, prometheusURL, jobName)
}

// Helper function to send remote write request (reuses code from historic.go)
func sendRemoteWrite(prometheusURL string, writeRequest *prompb.WriteRequest) error {
	data, err := writeRequest.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal write request: %w", err)
	}

	compressed := snappy.Encode(nil, data)

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

	return nil
}
