package ports

import "context"

// PolicyEvaluator is the port for policy evaluation (ADR-063).
// Stub type to be implemented in T11.
type PolicyEvaluator interface {
	// Evaluate returns a Decision for the given action.
	Evaluate(ctx context.Context, action Action) (Decision, error)
}

// Action represents an action to be evaluated by PolicyEvaluator.
type Action struct {
	Actor  Actor
	Target string
	Type   string
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

// Obligation represents a policy obligation (ADR-063).
type Obligation struct {
	Action string
	Target string
}
