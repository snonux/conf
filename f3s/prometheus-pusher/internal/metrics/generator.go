package metrics

import (
	"math/rand"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	minTemperature = 15.0
	maxTemperature = 35.0
	maxConnections = 100
	maxRequests    = 10
)

var (
	jobTypes = []string{"email", "report", "backup"}
	statuses = []string{"success", "failed"}
)

// Collectors holds Prometheus metric collectors for realtime mode
type Collectors struct {
	RequestsTotal      prometheus.Counter
	ActiveConnections  prometheus.Gauge
	TemperatureCelsius prometheus.Gauge
	RequestDuration    prometheus.Histogram
	JobsProcessed      *prometheus.CounterVec
}

// NewCollectors creates new Prometheus metric collectors
func NewCollectors() Collectors {
	return Collectors{
		RequestsTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "app_requests_total",
				Help: "Total number of requests processed",
			},
		),
		ActiveConnections: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "app_active_connections",
				Help: "Number of currently active connections",
			},
		),
		TemperatureCelsius: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "app_temperature_celsius",
				Help: "Current temperature in Celsius",
			},
		),
		RequestDuration: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "app_request_duration_seconds",
				Help:    "Histogram of request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
		),
		JobsProcessed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "app_jobs_processed_total",
				Help: "Total number of jobs processed by type",
			},
			[]string{"job_type", "status"},
		),
	}
}

// Simulate generates random metric values for the collectors
func (c Collectors) Simulate() {
	c.RequestsTotal.Add(float64(rand.Intn(maxRequests) + 1))
	c.ActiveConnections.Set(float64(rand.Intn(maxConnections)))
	c.TemperatureCelsius.Set(minTemperature + rand.Float64()*(maxTemperature-minTemperature))

	for i := 0; i < rand.Intn(5)+1; i++ {
		duration := rand.Float64() * 2
		c.RequestDuration.Observe(duration)
	}

	for _, jobType := range jobTypes {
		status := statuses[rand.Intn(len(statuses))]
		c.JobsProcessed.WithLabelValues(jobType, status).Add(float64(rand.Intn(5)))
	}
}
