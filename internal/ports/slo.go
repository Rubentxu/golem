package ports

import "context"

// SLO defines a Service Level Objective (REQ-SLO-001..003).
type SLO struct {
	Name        string  `json:"name"`
	Target      float64 `json:"target"`       // e.g., 0.99 for 99%
	WindowHours int     `json:"window_hours"` // e.g., 168 for 7 days
	ErrorBudget float64 `json:"error_budget"` // initial error budget percentage
}

// SLOViolation indicates an SLO has been violated (REQ-SLO-003).
type SLOViolation struct {
	SLOName        string  `json:"slo_name"`
	BudgetConsumed float64 `json:"budget_consumed"` // percentage 0..1
	BurnRate       float64 `json:"burn_rate"`
}

// SLOTracker tracks SLOs and emits alerts on violations (REQ-SLO-001..004).
type SLOTracker interface {
	// Record records a metric value for an SLO.
	Record(ctx context.Context, sloName string, value float64) error
	// Evaluate evaluates all SLOs and returns violations.
	Evaluate(ctx context.Context) ([]SLOViolation, error)
}
