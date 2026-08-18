package tck

import (
	"context"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

// TestPolicy_AgentProposal_Gated verifies that agent proposals are gated by PolicyEvaluator (AC-3).
func TestPolicy_AgentProposal_Gated(t *testing.T) {
	// T11.RED: This test should FAIL before implementation
	// After T11.GREEN, agent proposals should require policy evaluation

	actor := ports.Actor{Type: "agent", ID: "test-agent"}
	action := ports.Action{
		Actor:  actor,
		Target: "proposal:new",
		Type:   "propose",
	}

	var policy ports.PolicyEvaluator
	if policy == nil {
		t.Skip("PolicyEvaluator not yet implemented")
	}

	ctx := context.Background()
	decision, err := policy.Evaluate(ctx, action)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// Agent proposing should require approval
	if decision.Outcome != ports.DecisionOutcomeApprovalRequired {
		t.Errorf("expected DecisionOutcomeApprovalRequired for agent propose, got %v", decision.Outcome)
	}
}

// TestPolicy_DefaultDeny verifies default deny when no explicit policy exists.
func TestPolicy_DefaultDeny(t *testing.T) {
	actor := ports.Actor{Type: "agent", ID: "test-agent"}
	action := ports.Action{
		Actor:  actor,
		Target: "unknown:resource",
		Type:   "read",
	}

	var policy ports.PolicyEvaluator
	if policy == nil {
		t.Skip("PolicyEvaluator not yet implemented")
	}

	ctx := context.Background()
	decision, err := policy.Evaluate(ctx, action)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// Default should be deny for agent
	if decision.Outcome != ports.DecisionOutcomeDeny {
		t.Errorf("expected DecisionOutcomeDeny as default for agent, got %v", decision.Outcome)
	}
}
