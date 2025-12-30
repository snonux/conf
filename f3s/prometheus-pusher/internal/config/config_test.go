package config

import (
	"testing"
	"time"
)

func TestNewConfig(t *testing.T) {
	cfg := NewConfig()

	if cfg.Mode != ModeRealtime {
		t.Errorf("Default mode = %v, want %v", cfg.Mode, ModeRealtime)
	}
	if cfg.PushgatewayURL != "http://localhost:9091" {
		t.Errorf("Default PushgatewayURL = %v, want http://localhost:9091", cfg.PushgatewayURL)
	}
	if cfg.PrometheusURL != "http://localhost:9090/api/v1/write" {
		t.Errorf("Default PrometheusURL = %v, want http://localhost:9090/api/v1/write", cfg.PrometheusURL)
	}
	if cfg.JobName != "example_metrics_pusher" {
		t.Errorf("Default JobName = %v, want example_metrics_pusher", cfg.JobName)
	}
	if cfg.InputFormat != "csv" {
		t.Errorf("Default InputFormat = %v, want csv", cfg.InputFormat)
	}
	if cfg.HoursAgo != 24 {
		t.Errorf("Default HoursAgo = %v, want 24", cfg.HoursAgo)
	}
	if cfg.Interval != 1 {
		t.Errorf("Default Interval = %v, want 1", cfg.Interval)
	}
}

func TestModeConstants(t *testing.T) {
	modes := []Mode{ModeRealtime, ModeHistoric, ModeBackfill, ModeAuto}
	expected := []string{"realtime", "historic", "backfill", "auto"}

	for i, mode := range modes {
		if string(mode) != expected[i] {
			t.Errorf("Mode constant %d = %v, want %v", i, mode, expected[i])
		}
	}
}

func TestConstants(t *testing.T) {
	if AutoIngestThreshold != 5*time.Minute {
		t.Errorf("AutoIngestThreshold = %v, want 5m", AutoIngestThreshold)
	}
	if DefaultHTTPTimeout != 10*time.Second {
		t.Errorf("DefaultHTTPTimeout = %v, want 10s", DefaultHTTPTimeout)
	}
}
