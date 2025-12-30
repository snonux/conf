package metrics

import (
	"testing"
	"time"
)

func TestNewSample(t *testing.T) {
	tests := []struct {
		name      string
		metric    string
		labels    map[string]string
		value     float64
		timestamp time.Time
		wantNil   bool
	}{
		{
			name:      "with labels",
			metric:    "test_metric",
			labels:    map[string]string{"env": "prod", "host": "server1"},
			value:     42.5,
			timestamp: time.Now(),
			wantNil:   false,
		},
		{
			name:      "nil labels initialized",
			metric:    "test_metric",
			labels:    nil,
			value:     100,
			timestamp: time.Now(),
			wantNil:   false,
		},
		{
			name:      "empty labels",
			metric:    "test_metric",
			labels:    map[string]string{},
			value:     0,
			timestamp: time.Now(),
			wantNil:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sample := NewSample(tt.metric, tt.labels, tt.value, tt.timestamp)

			if sample.MetricName != tt.metric {
				t.Errorf("MetricName = %v, want %v", sample.MetricName, tt.metric)
			}
			if sample.Value != tt.value {
				t.Errorf("Value = %v, want %v", sample.Value, tt.value)
			}
			if sample.Labels == nil {
				t.Error("Labels should never be nil")
			}
			if !sample.Timestamp.Equal(tt.timestamp) {
				t.Errorf("Timestamp = %v, want %v", sample.Timestamp, tt.timestamp)
			}
		})
	}
}

func TestSample_Age(t *testing.T) {
	tests := []struct {
		name     string
		sample   Sample
		wantNear time.Duration
	}{
		{
			name: "recent sample",
			sample: Sample{
				MetricName: "test",
				Timestamp:  time.Now().Add(-5 * time.Minute),
			},
			wantNear: 5 * time.Minute,
		},
		{
			name: "old sample",
			sample: Sample{
				MetricName: "test",
				Timestamp:  time.Now().Add(-1 * time.Hour),
			},
			wantNear: 1 * time.Hour,
		},
		{
			name: "very recent",
			sample: Sample{
				MetricName: "test",
				Timestamp:  time.Now().Add(-10 * time.Second),
			},
			wantNear: 10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			age := tt.sample.Age()
			// Allow 1 second tolerance for test execution time
			if age < tt.wantNear-time.Second || age > tt.wantNear+time.Second {
				t.Errorf("Age() = %v, want near %v", age, tt.wantNear)
			}
		})
	}
}

func TestSample_IsRecent(t *testing.T) {
	threshold := 5 * time.Minute

	tests := []struct {
		name      string
		sample    Sample
		threshold time.Duration
		want      bool
	}{
		{
			name: "within threshold",
			sample: Sample{
				MetricName: "test",
				Timestamp:  time.Now().Add(-2 * time.Minute),
			},
			threshold: threshold,
			want:      true,
		},
		{
			name: "beyond threshold",
			sample: Sample{
				MetricName: "test",
				Timestamp:  time.Now().Add(-10 * time.Minute),
			},
			threshold: threshold,
			want:      false,
		},
		{
			name: "exactly at threshold",
			sample: Sample{
				MetricName: "test",
				Timestamp:  time.Now().Add(-5 * time.Minute),
			},
			threshold: threshold,
			want:      false,
		},
		{
			name: "very recent",
			sample: Sample{
				MetricName: "test",
				Timestamp:  time.Now().Add(-10 * time.Second),
			},
			threshold: threshold,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sample.IsRecent(tt.threshold); got != tt.want {
				t.Errorf("IsRecent() = %v, want %v (age: %v)", got, tt.want, tt.sample.Age())
			}
		})
	}
}
