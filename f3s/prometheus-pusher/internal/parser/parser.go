package parser

import (
	"context"
	"fmt"
	"io"
	"os"

	"prometheus-pusher/internal/metrics"
)

// Parser defines the interface for metric parsers.
type Parser interface {
	Parse(ctx context.Context, reader io.Reader) ([]metrics.Sample, error)
}

// ParseFile parses metrics from a file.
func ParseFile(ctx context.Context, filename, format string) ([]metrics.Sample, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return parseWithFormat(ctx, file, format)
}

// ParseStdin parses metrics from standard input.
func ParseStdin(ctx context.Context, format string) ([]metrics.Sample, error) {
	return parseWithFormat(ctx, os.Stdin, format)
}

// parseWithFormat parses metrics using the specified format.
func parseWithFormat(ctx context.Context, reader io.Reader, format string) ([]metrics.Sample, error) {
	var parser Parser

	switch format {
	case "csv":
		parser = NewCSVParser()
	case "json":
		parser = NewJSONParser()
	default:
		return nil, fmt.Errorf("unsupported format: %s (use csv or json)", format)
	}

	samples, err := parser.Parse(ctx, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse metrics: %w", err)
	}

	if len(samples) == 0 {
		return nil, fmt.Errorf("no valid samples found")
	}

	return samples, nil
}
