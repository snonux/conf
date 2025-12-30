package main

import (
	"flag"
	"log"
	"math/rand"
	"time"
)

func main() {
	// Command-line flags
	mode := flag.String("mode", "realtime", "Mode: realtime (push to pushgateway) or historic (backfill via remote write)")
	pushgatewayURL := flag.String("pushgateway", "http://localhost:9091", "Pushgateway URL for realtime mode")
	prometheusURL := flag.String("prometheus", "http://localhost:9090/api/v1/write", "Prometheus remote write URL for historic mode")
	hoursAgo := flag.Int("hours-ago", 24, "For historic mode: how many hours ago (single datapoint)")
	startHours := flag.Int("start-hours", 48, "For backfill: start time in hours ago")
	endHours := flag.Int("end-hours", 0, "For backfill: end time in hours ago")
	interval := flag.Int("interval", 1, "For backfill: interval between datapoints in hours")
	continuous := flag.Bool("continuous", false, "For realtime mode: push continuously every 15s")
	jobName := flag.String("job", "example_metrics_pusher", "Job name for realtime mode")

	flag.Parse()

	// Seed random number generator
	rand.Seed(time.Now().UnixNano())

	switch *mode {
	case "realtime":
		runRealtimeMode(*pushgatewayURL, *jobName, *continuous)

	case "historic":
		if err := PushHistoricData(*prometheusURL, *hoursAgo); err != nil {
			log.Fatalf("Failed to push historic data: %v", err)
		}

	case "backfill":
		if err := BackfillHistoricData(*prometheusURL, *startHours, *endHours, *interval); err != nil {
			log.Fatalf("Failed to backfill data: %v", err)
		}

	default:
		log.Fatalf("Unknown mode: %s (use realtime, historic, or backfill)", *mode)
	}
}

func runRealtimeMode(pushgatewayURL, jobName string, continuous bool) {
	log.Printf("Starting Prometheus metrics pusher in REALTIME mode")
	log.Printf("Pushgateway URL: %s", pushgatewayURL)
	log.Printf("Job name: %s", jobName)

	// Push immediately on start
	simulateMetrics()
	if err := pushMetrics(pushgatewayURL, jobName); err != nil {
		log.Printf("Error pushing metrics: %v", err)
	} else {
		log.Printf("Successfully pushed metrics to Pushgateway")
	}

	if continuous {
		// Push metrics every 15 seconds
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		log.Printf("Continuous mode: pushing metrics every 15 seconds. Press Ctrl+C to stop.")

		for range ticker.C {
			simulateMetrics()
			if err := pushMetrics(pushgatewayURL, jobName); err != nil {
				log.Printf("Error pushing metrics: %v", err)
			} else {
				log.Printf("Successfully pushed metrics to Pushgateway")
			}
		}
	}
}
