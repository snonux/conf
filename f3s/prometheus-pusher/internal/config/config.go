package config

import "time"

// Mode represents the ingestion mode
type Mode string

const (
	ModeRealtime Mode = "realtime"
	ModeHistoric Mode = "historic"
	ModeBackfill Mode = "backfill"
	ModeAuto     Mode = "auto"
)

// Config holds all configuration for the prometheus-pusher
type Config struct {
	Mode           Mode
	PushgatewayURL string
	PrometheusURL  string
	JobName        string
	Continuous     bool
	InputFile      string
	InputFormat    string
	HoursAgo       int
	StartHours     int
	EndHours       int
	Interval       int
}

// NewConfig creates a new Config with default values
func NewConfig() Config {
	return Config{
		Mode:           ModeRealtime,
		PushgatewayURL: "http://localhost:9091",
		PrometheusURL:  "http://localhost:9090/api/v1/write",
		JobName:        "example_metrics_pusher",
		InputFormat:    "csv",
		HoursAgo:       24,
		StartHours:     48,
		EndHours:       0,
		Interval:       1,
	}
}

// AutoIngestThreshold is the age threshold for auto mode routing
const AutoIngestThreshold = 5 * time.Minute

// DefaultHTTPTimeout is the default timeout for HTTP requests
const DefaultHTTPTimeout = 10 * time.Second
