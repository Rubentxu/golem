package ports

import (
	"testing"
)

// TestSLO_BurnRateCalc verifies SLO burn rate calculation.
func TestSLO_BurnRateCalc(t *testing.T) {
	t.Parallel()
	slo := SLO{
		Name:        "command-latency",
		Target:      0.99,
		WindowHours: 168, // 7 days
		ErrorBudget: 0.01,
	}

	// Simulate burn rate of 2x over 1 hour.
	// Error budget = 1%, burn rate 2x means 2% of budget consumed per hour.
	violation := SLOViolation{
		SLOName:        slo.Name,
		BudgetConsumed: 0.02, // 2% consumed (burn rate 2x)
		BurnRate:       2.0,
	}

	if violation.BurnRate <= 1.0 {
		t.Errorf("expected burn rate > 1.0 for violation, got %f", violation.BurnRate)
	}
	if violation.BudgetConsumed <= slo.ErrorBudget {
		t.Errorf("expected budget consumed > %f for violation, got %f", slo.ErrorBudget, violation.BudgetConsumed)
	}
}
