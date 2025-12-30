package ingester

import (
	"context"
	"testing"
	"time"

	"prometheus-pusher/internal/config"
	"prometheus-pusher/internal/metrics"
)

func TestDetermineMode(t *testing.T) {
	tests := []struct {
		name      string
		timestamp time.Time
		want      config.Mode
	}{
		{
			name:      "current time is realtime",
			timestamp: time.Now(),
			want:      config.ModeRealtime,
		},
		{
			name:      "1 minute ago is realtime",
			timestamp: time.Now().Add(-1 * time.Minute),
			want:      config.ModeRealtime,
		},
		{
			name:      "4 minutes ago is realtime",
			timestamp: time.Now().Add(-4 * time.Minute),
			want:      config.ModeRealtime,
		},
		{
			name:      "6 minutes ago is historic",
			timestamp: time.Now().Add(-6 * time.Minute),
			want:      config.ModeHistoric,
		},
		{
			name:      "1 hour ago is historic",
			timestamp: time.Now().Add(-1 * time.Hour),
			want:      config.ModeHistoric,
		},
		{
			name:      "1 day ago is historic",
			timestamp: time.Now().Add(-24 * time.Hour),
			want:      config.ModeHistoric,
		},
		{
			name:      "exactly 5 minutes is historic (edge case)",
			timestamp: time.Now().Add(-5 * time.Minute),
			want:      config.ModeHistoric,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetermineMode(tt.timestamp)
			if got != tt.want {
				age := time.Since(tt.timestamp)
				t.Errorf("DetermineMode() = %v, want %v (age: %v)", got, tt.want, age)
			}
		})
	}
}

func TestGroupSamplesByMode(t *testing.T) {
	now := time.Now()
	samples := []metrics.Sample{
		{MetricName: "metric1", Timestamp: now.Add(-1 * time.Minute)},   // realtime
		{MetricName: "metric2", Timestamp: now.Add(-2 * time.Minute)},   // realtime
		{MetricName: "metric3", Timestamp: now.Add(-10 * time.Minute)},  // historic
		{MetricName: "metric4", Timestamp: now.Add(-1 * time.Hour)},     // historic
		{MetricName: "metric5", Timestamp: now.Add(-30 * time.Second)},  // realtime
	}

	realtime, historic := groupSamplesByMode(samples)

	if len(realtime) != 3 {
		t.Errorf("Got %d realtime samples, want 3", len(realtime))
	}
	if len(historic) != 2 {
		t.Errorf("Got %d historic samples, want 2", len(historic))
	}

	// Verify correct grouping
	for _, s := range realtime {
		if DetermineMode(s.Timestamp) != config.ModeRealtime {
			t.Errorf("Sample %s incorrectly grouped as realtime (age: %v)", s.MetricName, s.Age())
		}
	}
	for _, s := range historic {
		if DetermineMode(s.Timestamp) != config.ModeHistoric {
			t.Errorf("Sample %s incorrectly grouped as historic (age: %v)", s.MetricName, s.Age())
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "seconds",
			duration: 45 * time.Second,
			want:     "45 seconds",
		},
		{
			name:     "minutes",
			duration: 5 * time.Minute,
			want:     "5 minutes",
		},
		{
			name:     "hours",
			duration: 2*time.Hour + 30*time.Minute,
			want:     "2.5 hours",
		},
		{
			name:     "days",
			duration: 36 * time.Hour,
			want:     "1.5 days",
		},
		{
			name:     "less than minute",
			duration: 30 * time.Second,
			want:     "30 seconds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("formatDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAutoIngester_Ingest_EmptySamples(t *testing.T) {
	collectors := metrics.NewCollectors()
	autoIngester := NewAutoIngester(collectors)
	ctx := context.Background()
	cfg := config.NewConfig()

	err := autoIngester.Ingest(ctx, []metrics.Sample{}, cfg)
	if err == nil {
		t.Error("Expected error for empty samples, got nil")
	}
	if err.Error() != "no samples to ingest" {
		t.Errorf("Expected 'no samples to ingest' error, got: %v", err)
	}
}

func TestAutoIngester_New(t *testing.T) {
	collectors := metrics.NewCollectors()
	ingester := NewAutoIngester(collectors)

	// Verify ingester was created with components
	if ingester.collectors.RequestsTotal == nil {
		t.Error("AutoIngester.collectors not initialized properly")
	}
}
