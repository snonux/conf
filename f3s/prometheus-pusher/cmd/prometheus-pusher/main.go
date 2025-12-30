package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"prometheus-pusher/internal/config"
	"prometheus-pusher/internal/ingester"
	"prometheus-pusher/internal/metrics"
	"prometheus-pusher/internal/parser"
	"prometheus-pusher/internal/version"
)

func main() {
	cfg := parseFlags()

	rand.Seed(time.Now().UnixNano())

	ctx, cancel := createContextWithSignalHandler()
	defer cancel()

	if err := run(ctx, cfg); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

// parseFlags parses command-line flags and returns a Config.
func parseFlags() config.Config {
	cfg := config.NewConfig()

	showVersion := flag.Bool("version", false, "Print version and exit")
	mode := flag.String("mode", "realtime", "Mode: realtime, historic, backfill, or auto")
	pushgatewayURL := flag.String("pushgateway", cfg.PushgatewayURL, "Pushgateway URL for realtime mode")
	prometheusURL := flag.String("prometheus", cfg.PrometheusURL, "Prometheus remote write URL for historic mode")
	jobName := flag.String("job", cfg.JobName, "Job name for metrics")
	continuous := flag.Bool("continuous", false, "For realtime mode: push continuously every 15s")

	hoursAgo := flag.Int("hours-ago", cfg.HoursAgo, "For historic mode: how many hours ago (single datapoint)")
	startHours := flag.Int("start-hours", cfg.StartHours, "For backfill: start time in hours ago")
	endHours := flag.Int("end-hours", cfg.EndHours, "For backfill: end time in hours ago")
	interval := flag.Int("interval", cfg.Interval, "For backfill: interval between datapoints in hours")

	inputFile := flag.String("file", "", "For auto mode: input file with metrics")
	inputFormat := flag.String("format", cfg.InputFormat, "For auto mode: input format (csv or json)")

	flag.Parse()

	if *showVersion {
		fmt.Printf("prometheus-pusher version %s\n", version.Version)
		os.Exit(0)
	}

	cfg.Mode = config.Mode(*mode)
	cfg.PushgatewayURL = *pushgatewayURL
	cfg.PrometheusURL = *prometheusURL
	cfg.JobName = *jobName
	cfg.Continuous = *continuous
	cfg.HoursAgo = *hoursAgo
	cfg.StartHours = *startHours
	cfg.EndHours = *endHours
	cfg.Interval = *interval
	cfg.InputFile = *inputFile
	cfg.InputFormat = *inputFormat

	return cfg
}

// createContextWithSignalHandler creates a context that cancels on interrupt signals.
func createContextWithSignalHandler() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Printf("\nReceived interrupt signal, shutting down...")
		cancel()
	}()

	return ctx, cancel
}

// run executes the appropriate mode based on configuration.
func run(ctx context.Context, cfg config.Config) error {
	switch cfg.Mode {
	case config.ModeRealtime:
		return runRealtimeMode(ctx, cfg)
	case config.ModeHistoric:
		return runHistoricMode(ctx, cfg)
	case config.ModeBackfill:
		return runBackfillMode(ctx, cfg)
	case config.ModeAuto:
		return runAutoMode(ctx, cfg)
	default:
		return fmt.Errorf("unknown mode: %s (use realtime, historic, backfill, or auto)", cfg.Mode)
	}
}

// runRealtimeMode runs the realtime ingestion mode.
func runRealtimeMode(ctx context.Context, cfg config.Config) error {
	log.Printf("Starting Prometheus metrics pusher in REALTIME mode")
	log.Printf("Pushgateway URL: %s", cfg.PushgatewayURL)
	log.Printf("Job name: %s", cfg.JobName)

	collectors := metrics.NewCollectors()
	pushgateway := ingester.NewPushgatewayIngester()

	if err := pushgateway.Ingest(ctx, collectors, cfg.PushgatewayURL, cfg.JobName); err != nil {
		return fmt.Errorf("failed to push metrics: %w", err)
	}
	log.Printf("Successfully pushed metrics to Pushgateway")

	if cfg.Continuous {
		return runContinuousMode(ctx, pushgateway, collectors, cfg)
	}

	return nil
}

// runContinuousMode pushes metrics continuously every 15 seconds.
func runContinuousMode(ctx context.Context, pushgateway ingester.PushgatewayIngester, collectors metrics.Collectors, cfg config.Config) error {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	log.Printf("Continuous mode: pushing metrics every 15 seconds. Press Ctrl+C to stop.")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := pushgateway.Ingest(ctx, collectors, cfg.PushgatewayURL, cfg.JobName); err != nil {
				log.Printf("Error pushing metrics: %v", err)
			} else {
				log.Printf("Successfully pushed metrics to Pushgateway")
			}
		}
	}
}

// runHistoricMode runs the historic ingestion mode.
func runHistoricMode(ctx context.Context, cfg config.Config) error {
	remoteWrite := ingester.NewRemoteWriteIngester()
	return remoteWrite.IngestHistoric(ctx, cfg.PrometheusURL, cfg.HoursAgo)
}

// runBackfillMode runs the backfill ingestion mode.
func runBackfillMode(ctx context.Context, cfg config.Config) error {
	remoteWrite := ingester.NewRemoteWriteIngester()
	return remoteWrite.Backfill(ctx, cfg.PrometheusURL, cfg.StartHours, cfg.EndHours, cfg.Interval)
}

// runAutoMode runs the auto ingestion mode.
func runAutoMode(ctx context.Context, cfg config.Config) error {
	log.Printf("🤖 AUTO mode: Automatically detecting timestamp age and choosing ingestion method")

	samples, err := loadSamples(ctx, cfg)
	if err != nil {
		return err
	}

	logFileSource(cfg)

	collectors := metrics.NewCollectors()
	autoIngester := ingester.NewAutoIngester(collectors)

	return autoIngester.Ingest(ctx, samples, cfg)
}

// loadSamples loads samples from file or stdin based on configuration.
func loadSamples(ctx context.Context, cfg config.Config) ([]metrics.Sample, error) {
	if cfg.InputFile != "" {
		return parser.ParseFile(ctx, cfg.InputFile, cfg.InputFormat)
	}
	return parser.ParseStdin(ctx, cfg.InputFormat)
}

// logFileSource logs the source of the input data.
func logFileSource(cfg config.Config) {
	if cfg.InputFile != "" {
		log.Printf("📁 Reading metrics from: %s (format: %s)", cfg.InputFile, cfg.InputFormat)
	} else {
		log.Printf("📥 Reading metrics from stdin (format: %s)", cfg.InputFormat)
	}
}
