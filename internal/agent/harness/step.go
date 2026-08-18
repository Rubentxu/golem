// Package harness provides the offline evaluation harness for agentic behaviors
// (ADR-070). It executes Given/When/Then steps over held-out fixtures,
// supports checkpoint + deterministic replay, and produces a scored Result.
//
// State machine (ADR-070):
//
//	idle → given → when → then → completed
//	                        ↘ rolled-back
//
// Checkpoints are saved after each step so interrupted runs can resume.
// The harness is deterministic: same fixture + clock + IDs → byte-identical
// result (AC-5).
package harness

import (
	"errors"
	"fmt"
)

// Step is the state of an eval harness run (ADR-070).
type Step uint8

const (
	StepIdle Step = iota
	StepGiven
	StepWhen
	StepThen
	StepCompleted
	StepRolledBack
)

// String returns the string representation of a step.
func (s Step) String() string {
	switch s {
	case StepIdle:
		return "idle"
	case StepGiven:
		return "given"
	case StepWhen:
		return "when"
	case StepThen:
		return "then"
	case StepCompleted:
		return "completed"
	case StepRolledBack:
		return "rolled-back"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// AsUint64 encodes the step for storage in CheckpointStore.
func (s Step) AsUint64() uint64 { return uint64(s) }

// FromUint64 decodes a step from CheckpointStore.
func FromUint64(v uint64) (Step, error) {
	s := Step(v)
	switch s {
	case StepIdle, StepGiven, StepWhen, StepThen, StepCompleted, StepRolledBack:
		return s, nil
	default:
		return StepIdle, fmt.Errorf("agent harness: unknown step %d", v)
	}
}

// IsTerminal returns true if the step is a terminal state.
func (s Step) IsTerminal() bool {
	return s == StepCompleted || s == StepRolledBack
}

// RollbackReason encodes why a rollback occurred.
type RollbackReason string

const (
	// RollbackPolicyViolation indicates the agent's proposal violated policy.
	RollbackPolicyViolation RollbackReason = "policy_violation"
	// RollbackBudgetExceeded indicates a budget limit was exceeded.
	RollbackBudgetExceeded RollbackReason = "budget_exceeded"
	// RollbackLLMError indicates the LLM call failed.
	RollbackLLMError RollbackReason = "llm_error"
	// RollbackProposalConflict indicates a proposal conflict on apply.
	RollbackProposalConflict RollbackReason = "proposal_conflict"
	// RollbackNoAgenticHandler indicates no agentic handler is wired.
	RollbackNoAgenticHandler RollbackReason = "no_agentic_handler"
)

// Valid returns an error if r is not a known rollback reason.
func (r RollbackReason) Valid() error {
	switch r {
	case RollbackPolicyViolation, RollbackBudgetExceeded, RollbackLLMError, RollbackProposalConflict, RollbackNoAgenticHandler:
		return nil
	default:
		return errors.New("agent harness: unknown rollback reason")
	}
}
