// Package harness provides the Migration Rehearsal R4 state machine.
// It orchestrates the 9-step migration flow: snapshot → load → replay → shadow → diff → cutover-pending → observe → completed (or rolled-back).
//
// State machine (ADR-005 naming):
//
//	idle → snapshotting → loading → replaying → shadowing → diffing → cutover-pending → observing → completed
//	                                                                                         ↘ rolled-back
//
// Each transition emits an audit event before mutating state.
package harness

import (
	"errors"
	"fmt"
)

// Step is the state of the migration harness.
type Step uint8

const (
	StepIdle Step = iota
	StepSnapshotting
	StepLoading
	StepReplaying
	StepShadowing
	StepDiffing
	StepCutoverPending
	StepObserving
	StepCompleted
	StepRolledBack
)

// String returns the string representation of a step.
func (s Step) String() string {
	switch s {
	case StepIdle:
		return "idle"
	case StepSnapshotting:
		return "snapshotting"
	case StepLoading:
		return "loading"
	case StepReplaying:
		return "replaying"
	case StepShadowing:
		return "shadowing"
	case StepDiffing:
		return "diffing"
	case StepCutoverPending:
		return "cutover-pending"
	case StepObserving:
		return "observing"
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
	case StepIdle, StepSnapshotting, StepLoading, StepReplaying,
		StepShadowing, StepDiffing, StepCutoverPending,
		StepObserving, StepCompleted, StepRolledBack:
		return s, nil
	default:
		return StepIdle, fmt.Errorf("migration harness: unknown step %d", v)
	}
}

// IsTerminal returns true if the step is a terminal state.
func (s Step) IsTerminal() bool {
	return s == StepCompleted || s == StepRolledBack
}

// RollbackReason encodes why a rollback occurred.
type RollbackReason string

const (
	RollbackTargetTCKFailed   RollbackReason = "target_tck_failed"
	RollbackSemanticDiff      RollbackReason = "semantic_diff"
	RollbackObserveWindowDiff RollbackReason = "observe_window_diff"
)

// Valid returns an error if r is not a known rollback reason.
func (r RollbackReason) Valid() error {
	switch r {
	case RollbackTargetTCKFailed, RollbackSemanticDiff, RollbackObserveWindowDiff:
		return nil
	default:
		return errors.New("migration harness: unknown rollback reason")
	}
}
