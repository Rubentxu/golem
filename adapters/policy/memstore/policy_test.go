package memstore

import (
	"context"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

// TestMemStore_DefaultDeny_AgentWrite verifies that agent direct writes are denied.
func TestMemStore_DefaultDeny_AgentWrite(t *testing.T) {
	store := New()

	actor := ports.Actor{Type: "agent", ID: "test-agent"}
	action := ports.Action{
		Actor:  actor,
		Target: "graph:node:test",
		Type:   "write",
	}

	decision, err := store.Evaluate(context.Background(), action)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if decision.Outcome != ports.DecisionOutcomeDeny {
		t.Errorf("expected DecisionOutcomeDeny for agent write, got %v", decision.Outcome)
	}
}

// TestMemStore_AllowForNonAgent verifies that non-agents can read by default.
func TestMemStore_AllowForNonAgent(t *testing.T) {
	store := New()

	actor := ports.Actor{Type: "user", ID: "test-user"}
	action := ports.Action{
		Actor:  actor,
		Target: "graph:node:test",
		Type:   "read",
	}

	decision, err := store.Evaluate(context.Background(), action)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if decision.Outcome != ports.DecisionOutcomeAllow {
		t.Errorf("expected DecisionOutcomeAllow for user read, got %v", decision.Outcome)
	}
}

// TestMemStore_ExplicitRule verifies explicit allow rules.
func TestMemStore_ExplicitRule(t *testing.T) {
	store := New()

	// Add explicit allow rule for agent to read
	store.AddRule("agent", "graph:node:*", ports.DecisionOutcomeAllow)

	actor := ports.Actor{Type: "agent", ID: "test-agent"}
	action := ports.Action{
		Actor:  actor,
		Target: "graph:node:test",
		Type:   "read",
	}

	decision, err := store.Evaluate(context.Background(), action)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if decision.Outcome != ports.DecisionOutcomeAllow {
		t.Errorf("expected DecisionOutcomeAllow for agent with explicit rule, got %v", decision.Outcome)
	}
}
