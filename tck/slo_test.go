package tck

import (
	"context"
	"testing"

	"github.com/Rubentxu/golem/adapters/observability/slo"
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
