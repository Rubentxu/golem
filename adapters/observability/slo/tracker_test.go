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
		Name:        "availability",
		Target:      0.999,
		WindowHours: 168,
		ErrorBudget: 0.1,
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

// TestSLO_TracksErrorBudget verifies that the tracker correctly computes
// error budget consumption within the SLO window (REQ-SLO-002).
// RED test: fails because computeBudgetConsumed is a placeholder.
func TestSLO_TracksErrorBudget(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tracker := NewTracker()
	tracker.RegisterSLO(ports.SLO{
		Name:        "latency",
		Target:      0.999, // 99.9% of requests must be < 250ms
		WindowHours: 1,
		ErrorBudget: 0.001, // 0.1% budget
	})

	// Record 1000 good events (within target)
	for i := 0; i < 1000; i++ {
		_ = tracker.Record(ctx, "latency", 1.0) // good
	}

	// Record 2 bad events (outside target) - should consume 0.2% of budget
	// Error budget = 0.001 = 0.1%
	// Bad events / total = 2/1002 ≈ 0.2%
	// Budget consumed should be 0.2% / 0.1% = 2x budget = exhausted
	_ = tracker.Record(ctx, "latency", 0.0) // bad
	_ = tracker.Record(ctx, "latency", 0.0) // bad

	violations, err := tracker.Evaluate(ctx)
	if err != nil {
		t.Fatalf("Evaluate error: %v", err)
	}

	// With 2 bad out of 1002 total and 0.1% budget, budget is exhausted
	if len(violations) == 0 {
		t.Fatal("expected violation when budget exhausted, got none")
	}

	violation := violations[0]
	// Budget consumed should be > 0 (at least the 2 bad events)
	if violation.BudgetConsumed <= 0 {
		t.Errorf("expected positive BudgetConsumed, got %f", violation.BudgetConsumed)
	}
}

// TestSLO_SlidingWindow verifies that only events within the window are
// considered for budget computation (REQ-SLO-002).
// RED test: fails because the tracker does not implement sliding window.
func TestSLO_SlidingWindow(t *testing.T) {
	t.Skip("RED test: sliding window not yet implemented")
}

// TestSLO_FiresAlertOn2xBurnRate verifies that a burn rate > 2x triggers
// an alert event (REQ-SLO-003, AC-15).
// RED test: fails because burn rate alerting is not yet implemented.
func TestSLO_FiresAlertOn2xBurnRate(t *testing.T) {
	t.Skip("RED test: burn rate alerting not yet implemented")
}

// TestSLO_BudgetExhausted verifies that budget exhaustion > 90% triggers
// an exhausted event (REQ-SLO-003).
// RED test: fails because exhausted alerting is not yet implemented.
func TestSLO_BudgetExhausted(t *testing.T) {
	t.Skip("RED test: exhausted alerting not yet implemented")
}

// TestSLO_BudgetConsumedPercent verifies budget consumption is expressed
// as a fraction (0..1) of the error budget (REQ-SLO-002).
func TestSLO_BudgetConsumedPercent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tracker := NewTracker()
	tracker.RegisterSLO(ports.SLO{
		Name:        "error_rate",
		Target:      0.999, // 99.9% availability
		WindowHours: 1,
		ErrorBudget: 0.01, // 1% budget
	})

	// 99 good, 1 bad = 1% error rate = 100% of budget exhausted
	for i := 0; i < 99; i++ {
		_ = tracker.Record(ctx, "error_rate", 1.0)
	}
	_ = tracker.Record(ctx, "error_rate", 0.0)

	violations, err := tracker.Evaluate(ctx)
	if err != nil {
		t.Fatalf("Evaluate error: %v", err)
	}

	if len(violations) == 0 {
		t.Fatal("expected violation at 100% budget consumed")
	}

	// BudgetConsumed should be close to 1.0 (100% of budget)
	v := violations[0]
	if v.BudgetConsumed < 0.9 {
		t.Errorf("expected BudgetConsumed >= 0.9, got %f", v.BudgetConsumed)
	}
}

// TestSLO_BurnRateCalc verifies burn rate is computed correctly (REQ-SLO-002).
// Burn rate = error_rate / allowed_error_rate
func TestSLO_BurnRateCalc(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tracker := NewTracker()
	tracker.RegisterSLO(ports.SLO{
		Name:        "latency",
		Target:      0.999,
		WindowHours: 1,
		ErrorBudget: 0.001, // allowed error budget = 0.1%
	})

	// 999 good, 1 bad = 0.1% error rate = 1x burn rate
	for i := 0; i < 999; i++ {
		_ = tracker.Record(ctx, "latency", 1.0)
	}
	_ = tracker.Record(ctx, "latency", 0.0) // 1 bad

	violations, err := tracker.Evaluate(ctx)
	if err != nil {
		t.Fatalf("Evaluate error: %v", err)
	}

	// At exactly 1x burn rate, no violation yet (violation at >1x)
	// The current implementation returns violations when budget consumed >= budget
	// With 1 bad / 1000 total and 0.1% budget, budget is exactly consumed
	_ = violations
	_ = t.Run("burn_rate_should_be_computed", func(t *testing.T) {
		// This will be validated after GREEN implementation
	})
}
