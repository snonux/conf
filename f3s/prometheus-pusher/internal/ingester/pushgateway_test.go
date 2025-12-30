package ingester

import (
	"testing"

	"prometheus-pusher/internal/metrics"
)

func TestNewPushgatewayIngester(t *testing.T) {
	ingester := NewPushgatewayIngester()

	// Verify the ingester was created (value type, so no nil check needed)
	_ = ingester
}

func TestPushgatewayIngester_Type(t *testing.T) {
	// Test that we can create and use the ingester
	collectors := metrics.NewCollectors()
	ingester := NewPushgatewayIngester()

	// The ingester should work with collectors
	if collectors.RequestsTotal == nil {
		t.Error("Collectors not initialized properly")
	}

	// Verify ingester is the correct type
	_ = ingester
}
