package tck

import (
	"context"
	"testing"

	"github.com/Rubentxu/golem/adapters/observability/slo"
	domainslo "github.com/Rubentxu/golem/internal/domain/slo"
	"github.com/Rubentxu/golem/internal/ports"
)

// TestSLOTracker_Record verifies metric recording (REQ-SLO-001).
func TestSLOTracker_Record(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tracker := slo.NewTracker()

	err := tracker.Record(ctx, "availability", 0.999)
	if err != nil {
		t.Fatalf("Record error: %v", err)
	}
}

// TestSLOTracker_Evaluate verifies SLO evaluation (REQ-SLO-001).
func TestSLOTracker_Evaluate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tracker := slo.NewTracker()
	tracker.RegisterSLO(ports.SLO{
		Name:        "availability",
		Target:      0.999,
		WindowHours: 168,
	})

	// Record some events.
	tracker.Record(ctx, "availability", 0.999)
	tracker.Record(ctx, "availability", 0.998)

	violations, err := tracker.Evaluate(ctx)
	if err != nil {
		t.Fatalf("Evaluate error: %v", err)
	}

	_ = violations
}

// TestSLO_AllNineSLIsInstrumented verifies all 13 SLIs can be recorded and
// evaluated without error (AC-15).
func TestSLO_AllNineSLIsInstrumented(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tracker := slo.NewTracker()

	// Register all 13 SLIs
	for _, def := range domainslo.AllSLIs() {
		tracker.RegisterSLO(ports.SLO{
			Name:        string(def.Name),
			Target:      def.Target,
			WindowHours: 1,
			ErrorBudget: 0.001,
		})
	}

	// Record one event per SLI
	for _, def := range domainslo.AllSLIs() {
		err := tracker.Record(ctx, string(def.Name), def.Target)
		if err != nil {
			t.Fatalf("Record(%s) error: %v", def.Name, err)
		}
	}

	// Evaluate all SLIs
	violations, err := tracker.Evaluate(ctx)
	if err != nil {
		t.Fatalf("Evaluate error: %v", err)
	}

	// All good events should not produce violations
	if len(violations) != 0 {
		t.Errorf("expected 0 violations with all-good events, got %d", len(violations))
	}
}

// TestSLO_HardLimitDeniesCommand verifies that an SLO with exhausted budget
// is flagged in the violation (REQ-SLO-001).
func TestSLO_HardLimitDeniesCommand(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tracker := slo.NewTracker()
	tracker.RegisterSLO(ports.SLO{
		Name:        "latency",
		Target:      0.999, // 0.1% allowed errors
		WindowHours: 1,
		ErrorBudget: 0.001, // 0.1% budget
	})

	// Exhaust the budget: 1 bad out of 1000 = 0.1% error rate = 100% budget consumed
	for i := 0; i < 999; i++ {
		_ = tracker.Record(ctx, "latency", 1.0)
	}
	_ = tracker.Record(ctx, "latency", 0.0) // bad event

	violations, err := tracker.Evaluate(ctx)
	if err != nil {
		t.Fatalf("Evaluate error: %v", err)
	}

	if len(violations) == 0 {
		t.Fatal("expected violation when budget exhausted, got none")
	}

	if violations[0].SLOName != "latency" {
		t.Errorf("expected violation for 'latency', got '%s'", violations[0].SLOName)
	}
}

// TestSLOTracker_ImplementsPort verifies the tracker implements the SLOTracker port.
func TestSLOTracker_ImplementsPort(t *testing.T) {
	t.Parallel()
	tracker := slo.NewTracker()
	var _ ports.SLOTracker = tracker
}
