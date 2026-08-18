package ports

import "fmt"

// BudgetLimits constrains resource usage per execution (ADR-069, M8 I-1).
// NOTE: W4 spec type divergence — BudgetLimits uses TokenCostUSD float64 for
// dollar cost tracking, while the Quota subsystem (W4) uses token_count int64
// for per-tenant quota counters. These are separate concepts: BudgetLimits
// constrains execution cost, while QuotaCounters track per-tenant consumption.
// The QuotaEnforcer port translates between them via the budget aggregator.
// It defines limits that are checked before allowing operations.
// The JSON shape matches the held-out fixture format.
type BudgetLimits struct {
	TokenCostUSD    float64 `json:"token_cost_usd"`    // cost per token (USD), 0 = unlimited
	WallClockMs     int64   `json:"wall_clock_ms"`     // max wall clock time in milliseconds, 0 = unlimited
	ToolCalls       int64   `json:"tool_calls"`        // max tool invocations per run, 0 = unlimited
	ProposalsPerRun int64   `json:"proposals_per_run"` // max proposals per run, 0 = unlimited
}

// Validate checks if the budget values are valid.
// All values must be non-negative.
func (b BudgetLimits) Validate() error {
	if b.TokenCostUSD < 0 {
		return fmt.Errorf("budget: TokenCostUSD must be non-negative")
	}
	if b.WallClockMs < 0 {
		return fmt.Errorf("budget: WallClockMs must be non-negative")
	}
	if b.ToolCalls < 0 {
		return fmt.Errorf("budget: ToolCalls must be non-negative")
	}
	if b.ProposalsPerRun < 0 {
		return fmt.Errorf("budget: ProposalsPerRun must be non-negative")
	}
	return nil
}

// Actual tracks the actual resource usage for comparison with BudgetLimits.
type Actual struct {
	TokenCostUSD float64 `json:"token_cost_usd"`
	WallClockMs  int64   `json:"wall_clock_ms"`
	ToolCalls    int64   `json:"tool_calls"`
	Proposals    int64   `json:"proposals"`
}

// Exceeded returns true if any actual usage exceeds the budget limits.
func (b BudgetLimits) Exceeded(actual Actual) bool {
	if actual.TokenCostUSD > b.TokenCostUSD && b.TokenCostUSD > 0 {
		return true
	}
	if actual.WallClockMs > b.WallClockMs && b.WallClockMs > 0 {
		return true
	}
	if actual.ToolCalls > b.ToolCalls && b.ToolCalls > 0 {
		return true
	}
	if actual.Proposals > b.ProposalsPerRun && b.ProposalsPerRun > 0 {
		return true
	}
	return false
}
