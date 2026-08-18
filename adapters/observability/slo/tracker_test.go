package slo

import (
	"context"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

// TestSLOTracker_Record verifies metric recording.
func TestSLOTracker_Record(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tracker := NewTracker()

	err := tracker.Record(ctx, "availability", 0.999)
	if err != nil {
		t.Fatalf("Record error: %v", err)
	}
}

// TestSLOTracker_Evaluate verifies SLO evaluation.
func TestSLOTracker_Evaluate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tracker := NewTracker()
	tracker.RegisterSLO(ports.SLO{
		Name:         "availability",
		Target:       0.999,
		WindowHours:  168,
		ErrorBudget:  0.1,
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

// Ensure Tracker implements SLOTracker
var _ ports.SLOTracker = (*Tracker)(nil)
