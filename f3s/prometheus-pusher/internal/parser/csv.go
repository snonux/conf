package parser

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"prometheus-pusher/internal/metrics"
)

// CSVParser parses metrics from CSV format
type CSVParser struct{}

// NewCSVParser creates a new CSV parser
func NewCSVParser() *CSVParser {
	return &CSVParser{}
}

// Parse reads metrics from CSV format
// Format: metric_name,label1=value1;label2=value2,value,timestamp_ms
func (p *CSVParser) Parse(ctx context.Context, reader io.Reader) ([]metrics.Sample, error) {
	var samples []metrics.Sample

	csvReader := csv.NewReader(reader)
	csvReader.Comment = '#'

	lineNum := 0
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}
		lineNum++

		if len(record) < 3 {
			continue // Skip invalid records
		}

		sample, err := p.parseRecord(record, lineNum)
		if err != nil {
			continue // Skip records with errors
		}

		samples = append(samples, sample)
	}

	return samples, nil
}

func (p *CSVParser) parseRecord(record []string, lineNum int) (metrics.Sample, error) {
	metricName := strings.TrimSpace(record[0])
	if metricName == "" {
		return metrics.Sample{}, fmt.Errorf("empty metric name")
	}

	labels := parseLabels(record[1])

	value, err := strconv.ParseFloat(strings.TrimSpace(record[2]), 64)
	if err != nil {
		return metrics.Sample{}, fmt.Errorf("invalid value: %w", err)
	}

	timestamp := time.Now()
	if len(record) > 3 && record[3] != "" {
		timestampMs, err := strconv.ParseInt(strings.TrimSpace(record[3]), 10, 64)
		if err == nil {
			timestamp = time.UnixMilli(timestampMs)
		}
	}

	return metrics.NewSample(metricName, labels, value, timestamp), nil
}

func parseLabels(labelStr string) map[string]string {
	labels := make(map[string]string)
	if labelStr == "" {
		return labels
	}

	labelPairs := strings.Split(labelStr, ";")
	for _, pair := range labelPairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			labels[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return labels
}
