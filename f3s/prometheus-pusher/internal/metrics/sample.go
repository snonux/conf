package metrics

import "time"

// Sample represents a single metric sample with timestamp
type Sample struct {
	MetricName string
	Labels     map[string]string
	Value      float64
	Timestamp  time.Time
}

// NewSample creates a new Sample
func NewSample(name string, labels map[string]string, value float64, timestamp time.Time) Sample {
	if labels == nil {
		labels = make(map[string]string)
	}
	return Sample{
		MetricName: name,
		Labels:     labels,
		Value:      value,
		Timestamp:  timestamp,
	}
}

// Age returns how old the sample is
func (s Sample) Age() time.Duration {
	return time.Since(s.Timestamp)
}

// IsRecent returns true if the sample is recent enough for realtime ingestion
func (s Sample) IsRecent(threshold time.Duration) bool {
	return s.Age() < threshold
}
