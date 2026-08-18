package ports

import (
	"context"
	"errors"
)

// Policy evaluator errors (ADR-063).
var (
	// ErrPolicyDenied is returned when an action is denied by policy.
	ErrPolicyDenied = errors.New("policy: action denied")
	// ErrPolicyApprovalRequired is returned when an action requires approval.
	ErrPolicyApprovalRequired = errors.New("policy: approval required")
)

// PolicyEvaluator is the port for policy evaluation (ADR-063).
// It enforces the closed world policy: agents cannot directly mutate the graph
// without explicit approval (AC-2, AC-3).
type PolicyEvaluator interface {
	// Evaluate returns a Decision for the given action.
	// Default policy: deny all actions by actors with Type="agent"
	// unless explicitly allowed by policy configuration.
	Evaluate(ctx context.Context, action Action) (Decision, error)
}

// Action represents an action to be evaluated by PolicyEvaluator.
type Action struct {
	Actor  Actor
	Target string
	Type   string // "read", "write", "delete", "propose", "execute"
}

// Decision represents the result of policy evaluation (ADR-063).
type Decision struct {
	Outcome    DecisionOutcome
	Obligation *Obligation
}

// DecisionOutcome represents the 4 outcomes of policy evaluation.
type DecisionOutcome int

const (
	// DecisionOutcomeAllow indicates the action is allowed.
	DecisionOutcomeAllow DecisionOutcome = iota
	// DecisionOutcomeDeny indicates the action is denied.
	DecisionOutcomeDeny
	// DecisionOutcomeApprovalRequired indicates the action requires approval.
	DecisionOutcomeApprovalRequired
	// DecisionOutcomeNoOpinion indicates no opinion on the action.
	DecisionOutcomeNoOpinion
)

// String returns a human-readable name for the outcome.
func (d DecisionOutcome) String() string {
	switch d {
	case DecisionOutcomeAllow:
		return "allow"
	case DecisionOutcomeDeny:
		return "deny"
	case DecisionOutcomeApprovalRequired:
		return "approval_required"
	case DecisionOutcomeNoOpinion:
		return "no_opinion"
	default:
		return "unknown"
	}
}

// Obligation represents a policy obligation attached to a decision (ADR-063).
type Obligation struct {
	Action string
	Target string
}
