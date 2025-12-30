package ingester

import (
	"context"
	"fmt"

	"prometheus-pusher/internal/metrics"

	"github.com/prometheus/client_golang/prometheus/push"
)

// PushgatewayIngester handles realtime metric ingestion via Pushgateway.
// Note: Pushgateway does not preserve custom timestamps - all metrics are
// timestamped with the current time when pushed.
type PushgatewayIngester struct{}

// NewPushgatewayIngester creates a new Pushgateway ingester.
func NewPushgatewayIngester() PushgatewayIngester {
	return PushgatewayIngester{}
}

// Ingest pushes metrics to Pushgateway.
// The samples parameter is currently ignored because Pushgateway doesn't support
// custom metric values from samples - it uses registered Prometheus collectors.
// This ingests generated metrics using the provided collectors.
func (i PushgatewayIngester) Ingest(ctx context.Context, collectors metrics.Collectors, url, jobName string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Generate random metric values
	collectors.Simulate()

	// Create pusher with all collectors
	pusher := push.New(url, jobName).
		Collector(collectors.RequestsTotal).
		Collector(collectors.ActiveConnections).
		Collector(collectors.TemperatureCelsius).
		Collector(collectors.RequestDuration).
		Collector(collectors.JobsProcessed).
		Grouping("instance", "example-app")

	// Push metrics to Pushgateway
	if err := pusher.Push(); err != nil {
		return fmt.Errorf("failed to push to pushgateway: %w", err)
	}

	return nil
}
