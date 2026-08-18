package tck

import (
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

// TestBudget_ExceededDenies verifies that budget exceeded denies operations (AC-8).
func TestBudget_ExceededDenies(t *testing.T) {
	budget := ports.BudgetLimits{
		TokenCostUSD:    0.01, // $0.01 max
		WallClockMs:     5000, // 5 seconds max
		ToolCalls:       10,
		ProposalsPerRun: 5,
	}

	// Within budget
	actual := ports.Actual{
		TokenCostUSD: 0.005,
		WallClockMs:  2500,
		ToolCalls:    5,
		Proposals:    2,
	}
	if budget.Exceeded(actual) {
		t.Error("expected within budget to not be exceeded")
	}

	// Token cost exceeded
	exceededToken := ports.Actual{
		TokenCostUSD: 0.015, // exceeds 0.01
		WallClockMs:  2500,
		ToolCalls:    5,
		Proposals:    2,
	}
	if !budget.Exceeded(exceededToken) {
		t.Error("expected exceeded token cost to be exceeded")
	}

	// Wall clock exceeded
	exceededWallClock := ports.Actual{
		TokenCostUSD: 0.005,
		WallClockMs:  6000, // exceeds 5000
		ToolCalls:    5,
		Proposals:    2,
	}
	if !budget.Exceeded(exceededWallClock) {
		t.Error("expected exceeded wall clock to be exceeded")
	}

	// Zero budget = unlimited for that dimension
	unlimitedBudget := ports.BudgetLimits{
		TokenCostUSD:    0, // 0 means unlimited
		WallClockMs:     5000,
		ToolCalls:       10,
		ProposalsPerRun: 5,
	}
	exceededToken2 := ports.Actual{
		TokenCostUSD: 1.0, // any value
		WallClockMs:  2500,
		ToolCalls:    5,
		Proposals:    2,
	}
	if unlimitedBudget.Exceeded(exceededToken2) {
		t.Error("expected zero budget = unlimited, not exceeded")
	}
}

// TestBudget_Validate verifies budget validation.
func TestBudget_Validate(t *testing.T) {
	validBudget := ports.BudgetLimits{
		TokenCostUSD:    0.01,
		WallClockMs:     5000,
		ToolCalls:       10,
		ProposalsPerRun: 5,
	}
	if err := validBudget.Validate(); err != nil {
		t.Errorf("expected valid budget, got error: %v", err)
	}

	invalidBudget := ports.BudgetLimits{
		TokenCostUSD:    -1,
		WallClockMs:     5000,
		ToolCalls:       10,
		ProposalsPerRun: 5,
	}
	if err := invalidBudget.Validate(); err == nil {
		t.Error("expected error for negative TokenCost")
	}
}
