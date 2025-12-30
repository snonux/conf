package parser

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCSVParser_Parse(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
		wantErr   bool
	}{
		{
			name: "valid single line",
			input: `test_metric,env=prod;host=server1,42.5,1234567890000`,
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "multiple lines",
			input: `metric1,label1=value1,100,1234567890000
metric2,label2=value2,200,1234567891000
metric3,label3=value3,300,1234567892000`,
			wantCount: 3,
			wantErr:   false,
		},
		{
			name: "with comments",
			input: `# This is a comment
metric1,env=test,50,1234567890000
# Another comment
metric2,env=prod,75,1234567891000`,
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "no timestamp defaults to now",
			input: `metric1,env=test,100`,
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "no labels",
			input: `metric1,,100,1234567890000`,
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:      "empty input",
			input:     "",
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "invalid line causes error",
			input: `metric1,env=test,100,1234567890000
invalid
metric2,env=prod,200,1234567891000`,
			wantCount: 0,
			wantErr:   true,
		},
		{
			name: "invalid value skipped",
			input: `metric1,env=test,not_a_number,1234567890000
metric2,env=prod,200,1234567891000`,
			wantCount: 1,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewCSVParser()
			reader := strings.NewReader(tt.input)
			ctx := context.Background()

			samples, err := parser.Parse(ctx, reader)

			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(samples) != tt.wantCount {
				t.Errorf("Parse() returned %d samples, want %d", len(samples), tt.wantCount)
			}
		})
	}
}

func TestCSVParser_ParseLabels(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{
			name:  "single label",
			input: "env=prod",
			want:  map[string]string{"env": "prod"},
		},
		{
			name:  "multiple labels",
			input: "env=prod;host=server1;region=us-west",
			want:  map[string]string{"env": "prod", "host": "server1", "region": "us-west"},
		},
		{
			name:  "empty string",
			input: "",
			want:  map[string]string{},
		},
		{
			name:  "invalid label format skipped",
			input: "env=prod;invalid;host=server1",
			want:  map[string]string{"env": "prod", "host": "server1"},
		},
		{
			name:  "with spaces",
			input: " env = prod ; host = server1 ",
			want:  map[string]string{"env": "prod", "host": "server1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLabels(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("parseLabels() returned %d labels, want %d", len(got), len(tt.want))
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("parseLabels()[%s] = %v, want %v", k, got[k], v)
				}
			}
		})
	}
}

func TestCSVParser_ParseWithContext(t *testing.T) {
	t.Run("context cancellation", func(t *testing.T) {
		parser := NewCSVParser()
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		input := strings.NewReader(`metric1,env=test,100,1234567890000`)
		_, err := parser.Parse(ctx, input)

		if err != context.Canceled {
			t.Errorf("Expected context.Canceled error, got %v", err)
		}
	})
}

func TestCSVParser_ParseTimestamp(t *testing.T) {
	parser := NewCSVParser()
	input := `metric1,env=test,100,1234567890000`
	reader := strings.NewReader(input)
	ctx := context.Background()

	samples, err := parser.Parse(ctx, reader)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(samples) != 1 {
		t.Fatalf("Expected 1 sample, got %d", len(samples))
	}

	expectedTime := time.UnixMilli(1234567890000)
	if !samples[0].Timestamp.Equal(expectedTime) {
		t.Errorf("Timestamp = %v, want %v", samples[0].Timestamp, expectedTime)
	}
}
