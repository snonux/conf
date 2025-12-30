package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

// Define metrics
var (
	// Counter: Monotonically increasing value (e.g., total requests processed)
	requestsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "app_requests_total",
			Help: "Total number of requests processed",
		},
	)

	// Gauge: Value that can go up or down (e.g., current temperature, active connections)
	activeConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "app_active_connections",
			Help: "Number of currently active connections",
		},
	)

	// Gauge for temperature simulation
	temperatureCelsius = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "app_temperature_celsius",
			Help: "Current temperature in Celsius",
		},
	)

	// Histogram: Distribution of values (e.g., request duration)
	requestDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "app_request_duration_seconds",
			Help:    "Histogram of request duration in seconds",
			Buckets: prometheus.DefBuckets, // Default buckets: .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10
		},
	)

	// Counter with labels
	jobsProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "app_jobs_processed_total",
			Help: "Total number of jobs processed by type",
		},
		[]string{"job_type", "status"},
	)
)

// simulateMetrics generates example metric data
func simulateMetrics() {
	// Increment request counter
	requestsTotal.Add(float64(rand.Intn(10) + 1))

	// Update active connections (random number between 0-100)
	activeConnections.Set(float64(rand.Intn(100)))

	// Simulate temperature (random between 15-35 Celsius)
	temperatureCelsius.Set(15 + rand.Float64()*20)

	// Record some request durations
	for i := 0; i < rand.Intn(5)+1; i++ {
		duration := rand.Float64() * 2 // 0-2 seconds
		requestDuration.Observe(duration)
	}

	// Record job completions with labels
	jobTypes := []string{"email", "report", "backup"}
	statuses := []string{"success", "failed"}

	for _, jobType := range jobTypes {
		status := statuses[rand.Intn(len(statuses))]
		jobsProcessed.WithLabelValues(jobType, status).Add(float64(rand.Intn(5)))
	}
}

// pushMetrics pushes all metrics to the Pushgateway
func pushMetrics(pushgatewayURL, jobName string) error {
	// Create a new pusher
	pusher := push.New(pushgatewayURL, jobName).
		Collector(requestsTotal).
		Collector(activeConnections).
		Collector(temperatureCelsius).
		Collector(requestDuration).
		Collector(jobsProcessed).
		Grouping("instance", "example-app")

	// Push metrics to the Pushgateway
	if err := pusher.Push(); err != nil {
		return fmt.Errorf("failed to push metrics: %w", err)
	}

	return nil
}

func main() {
	// Configuration - use localhost:9091 when port-forwarding
	// kubectl port-forward -n monitoring svc/pushgateway 9091:9091
	pushgatewayURL := "http://localhost:9091"
	jobName := "example_metrics_pusher"

	// Seed random number generator
	rand.Seed(time.Now().UnixNano())

	log.Printf("Starting Prometheus metrics pusher")
	log.Printf("Pushgateway URL: %s", pushgatewayURL)
	log.Printf("Job name: %s", jobName)

	// Push metrics every 15 seconds
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// Push immediately on start
	simulateMetrics()
	if err := pushMetrics(pushgatewayURL, jobName); err != nil {
		log.Printf("Error pushing metrics: %v", err)
	} else {
		log.Printf("Successfully pushed metrics to Pushgateway")
	}

	// Continue pushing periodically
	for range ticker.C {
		simulateMetrics()
		if err := pushMetrics(pushgatewayURL, jobName); err != nil {
			log.Printf("Error pushing metrics: %v", err)
		} else {
			log.Printf("Successfully pushed metrics to Pushgateway")
		}
	}
}
