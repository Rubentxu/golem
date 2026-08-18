package ports

import "fmt"

// Budget constrains resource usage per execution (ADR-069).
// It defines limits that are checked before allowing operations.
type Budget struct {
	TokenCost       float64 // cost per token (USD)
	WallClockMs     int     // max wall clock time in ms
	ToolCalls       int     // max tool invocations per run
	ProposalsPerRun int     // max proposals per run
}

// Validate checks if the budget values are valid.
// All values must be non-negative.
func (b Budget) Validate() error {
	if b.TokenCost < 0 {
		return fmt.Errorf("budget: TokenCost must be non-negative")
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

// Actual tracks the actual resource usage for comparison with Budget.
type Actual struct {
	TokenCost   float64
	WallClockMs int
	ToolCalls   int
	Proposals   int
}

// Exceeded returns true if any actual usage exceeds the budget limits.
func (b Budget) Exceeded(actual Actual) bool {
	if actual.TokenCost > b.TokenCost && b.TokenCost > 0 {
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
