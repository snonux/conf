package parser

import (
	"context"
	"strings"
	"testing"
)

func TestParseFile_CSV(t *testing.T) {
	// We can't easily test file operations without creating temp files
	// So we'll test the error case
	ctx := context.Background()
	_, err := ParseFile(ctx, "/nonexistent/file.csv", "csv")

	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestParseWithFormat_CSV(t *testing.T) {
	ctx := context.Background()
	input := `test_metric,env=prod,100,1234567890000`
	reader := strings.NewReader(input)

	samples, err := parseWithFormat(ctx, reader, "csv")
	if err != nil {
		t.Fatalf("parseWithFormat(csv) error = %v", err)
	}
	if len(samples) != 1 {
		t.Errorf("Expected 1 sample, got %d", len(samples))
	}
}

func TestParseWithFormat_JSON(t *testing.T) {
	ctx := context.Background()
	input := `[{"metric": "test_metric", "value": 100, "timestamp_ms": 1234567890000}]`
	reader := strings.NewReader(input)

	samples, err := parseWithFormat(ctx, reader, "json")
	if err != nil {
		t.Fatalf("parseWithFormat(json) error = %v", err)
	}
	if len(samples) != 1 {
		t.Errorf("Expected 1 sample, got %d", len(samples))
	}
}

func TestParseWithFormat_UnsupportedFormat(t *testing.T) {
	ctx := context.Background()
	reader := strings.NewReader("")

	_, err := parseWithFormat(ctx, reader, "xml")
	if err == nil {
		t.Error("Expected error for unsupported format")
	}
	if err.Error() != "unsupported format: xml (use csv or json)" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestParseWithFormat_EmptyResult(t *testing.T) {
	ctx := context.Background()
	input := `[]` // Empty JSON array
	reader := strings.NewReader(input)

	_, err := parseWithFormat(ctx, reader, "json")
	if err == nil {
		t.Error("Expected error for empty samples")
	}
	if err.Error() != "no valid samples found" {
		t.Errorf("Expected 'no valid samples found' error, got: %v", err)
	}
}

func TestParseStdin_Format(t *testing.T) {
	// We can't easily test stdin without mocking,
	// but we can verify the error path
	ctx := context.Background()

	// Test with invalid format
	_, err := parseWithFormat(ctx, strings.NewReader(""), "invalid_format")
	if err == nil {
		t.Error("Expected error for invalid format")
	}
}

func TestNewCSVParser(t *testing.T) {
	parser := NewCSVParser()
	if parser == nil {
		t.Error("NewCSVParser() returned nil")
	}
}

func TestNewJSONParser(t *testing.T) {
	parser := NewJSONParser()
	if parser == nil {
		t.Error("NewJSONParser() returned nil")
	}
}
