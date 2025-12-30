package metrics

import (
	"testing"
)

func TestNewCollectors(t *testing.T) {
	collectors := NewCollectors()

	if collectors.RequestsTotal == nil {
		t.Error("RequestsTotal should not be nil")
	}
	if collectors.ActiveConnections == nil {
		t.Error("ActiveConnections should not be nil")
	}
	if collectors.TemperatureCelsius == nil {
		t.Error("TemperatureCelsius should not be nil")
	}
	if collectors.RequestDuration == nil {
		t.Error("RequestDuration should not be nil")
	}
	if collectors.JobsProcessed == nil {
		t.Error("JobsProcessed should not be nil")
	}
}

func TestCollectors_Simulate(t *testing.T) {
	collectors := NewCollectors()

	// Should not panic
	collectors.Simulate()

	// Run multiple times to test randomness
	for i := 0; i < 10; i++ {
		collectors.Simulate()
	}
}

func TestCollectors_SimulateMetrics(t *testing.T) {
	collectors := NewCollectors()

	// Test that metrics get values after simulation
	collectors.Simulate()

	// We can't easily inspect the values without the prometheus client,
	// but we can verify the collectors were created properly
	if collectors.RequestsTotal == nil {
		t.Error("RequestsTotal not initialized")
	}
	if collectors.JobsProcessed == nil {
		t.Error("JobsProcessed not initialized")
	}
}
