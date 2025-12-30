package ingester

import (
	"context"
	"fmt"
	"log"
	"time"

	"prometheus-pusher/internal/config"
	"prometheus-pusher/internal/metrics"
)

const ageThreshold = 5 * time.Minute

// DetermineMode automatically determines which ingestion mode to use based on timestamp age.
// Data older than 5 minutes uses historic mode (Remote Write), newer data uses realtime mode (Pushgateway).
func DetermineMode(timestamp time.Time) config.Mode {
	age := time.Since(timestamp)
	if age > ageThreshold {
		return config.ModeHistoric
	}
	return config.ModeRealtime
}

// AutoIngester handles automatic ingestion by routing samples to appropriate ingesters.
type AutoIngester struct {
	pushgateway PushgatewayIngester
	remoteWrite RemoteWriteIngester
	collectors  metrics.Collectors
}

// NewAutoIngester creates a new auto ingester.
func NewAutoIngester(collectors metrics.Collectors) AutoIngester {
	return AutoIngester{
		pushgateway: NewPushgatewayIngester(),
		remoteWrite: NewRemoteWriteIngester(),
		collectors:  collectors,
	}
}

// Ingest automatically routes samples to appropriate ingestion method based on timestamp age.
func (a AutoIngester) Ingest(ctx context.Context, samples []metrics.Sample, cfg config.Config) error {
	if len(samples) == 0 {
		return fmt.Errorf("no samples to ingest")
	}

	realtimeSamples, historicSamples := groupSamplesByMode(samples)

	logIngestSummary(len(samples), len(realtimeSamples), len(historicSamples))

	if len(realtimeSamples) > 0 {
		if err := a.ingestRealtime(ctx, cfg); err != nil {
			return fmt.Errorf("failed to ingest realtime samples: %w", err)
		}
	}

	if len(historicSamples) > 0 {
		if err := a.ingestHistoric(ctx, historicSamples, cfg); err != nil {
			return fmt.Errorf("failed to ingest historic samples: %w", err)
		}
	}

	log.Printf("\n🎉 Auto-ingest complete!")
	return nil
}

// groupSamplesByMode separates samples into realtime and historic groups.
func groupSamplesByMode(samples []metrics.Sample) (realtime, historic []metrics.Sample) {
	realtimeSamples := make([]metrics.Sample, 0)
	historicSamples := make([]metrics.Sample, 0)

	for _, sample := range samples {
		if DetermineMode(sample.Timestamp) == config.ModeRealtime {
			realtimeSamples = append(realtimeSamples, sample)
		} else {
			historicSamples = append(historicSamples, sample)
		}
	}

	return realtimeSamples, historicSamples
}

// logIngestSummary logs the ingestion summary.
func logIngestSummary(total, realtime, historic int) {
	log.Printf("📊 Auto-ingest summary:")
	log.Printf("  Total samples: %d", total)
	log.Printf("  Realtime samples (< 5min old): %d", realtime)
	log.Printf("  Historic samples (> 5min old): %d", historic)
}

// ingestRealtime ingests realtime samples via Pushgateway.
func (a AutoIngester) ingestRealtime(ctx context.Context, cfg config.Config) error {
	log.Printf("\n🔄 Ingesting REALTIME samples via Pushgateway...")
	log.Printf("  Note: Pushgateway uses current timestamp (original timestamps ignored)")

	if err := a.pushgateway.Ingest(ctx, a.collectors, cfg.PushgatewayURL, cfg.JobName); err != nil {
		return err
	}

	log.Printf("✅ Successfully ingested realtime samples")
	return nil
}

// ingestHistoric ingests historic samples via Remote Write.
func (a AutoIngester) ingestHistoric(ctx context.Context, samples []metrics.Sample, cfg config.Config) error {
	log.Printf("\n⏰ Ingesting %d HISTORIC samples via Remote Write...", len(samples))

	for i, sample := range samples {
		age := time.Since(sample.Timestamp)
		log.Printf("  [%d/%d] %s (age: %s)", i+1, len(samples), sample.MetricName, formatDuration(age))
	}

	if err := a.remoteWrite.Ingest(ctx, samples, cfg.PrometheusURL); err != nil {
		return err
	}

	log.Printf("✅ Successfully ingested %d historic samples", len(samples))
	return nil
}

// formatDuration formats a duration in human-readable form.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0f seconds", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0f minutes", d.Minutes())
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%.1f hours", d.Hours())
	}
	return fmt.Sprintf("%.1f days", d.Hours()/24)
}
