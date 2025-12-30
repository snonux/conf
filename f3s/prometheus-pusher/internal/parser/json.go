package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"prometheus-pusher/internal/metrics"
)

// JSONParser parses metrics from JSON format
type JSONParser struct{}

// NewJSONParser creates a new JSON parser
func NewJSONParser() *JSONParser {
	return &JSONParser{}
}

type jsonSample struct {
	Metric      string            `json:"metric"`
	Labels      map[string]string `json:"labels"`
	Value       float64           `json:"value"`
	TimestampMs int64             `json:"timestamp_ms,omitempty"`
}

// Parse reads metrics from JSON format
func (p *JSONParser) Parse(ctx context.Context, reader io.Reader) ([]metrics.Sample, error) {
	var rawSamples []jsonSample

	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&rawSamples); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	samples := make([]metrics.Sample, 0, len(rawSamples))
	for _, raw := range rawSamples {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if raw.Metric == "" {
			continue
		}

		timestamp := time.Now()
		if raw.TimestampMs > 0 {
			timestamp = time.UnixMilli(raw.TimestampMs)
		}

		if raw.Labels == nil {
			raw.Labels = make(map[string]string)
		}

		samples = append(samples, metrics.NewSample(raw.Metric, raw.Labels, raw.Value, timestamp))
	}

	return samples, nil
}
