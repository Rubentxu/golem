package harness

import (
	"testing"
)

// TestMigrationHarness_Steps verifies the migration harness step state machine.
func TestMigrationHarness_Steps(t *testing.T) {
	t.Parallel()

	// Verify all steps have string representation.
	steps := []Step{
		StepIdle,
		StepSnapshotting,
		StepLoading,
		StepReplaying,
		StepShadowing,
		StepDiffing,
		StepCutoverPending,
		StepObserving,
		StepCompleted,
		StepRolledBack,
	}

	for _, s := range steps {
		if s.String() == "" {
			t.Errorf("Step %d has empty string", s)
		}
	}

	// Verify terminal states.
	if !StepCompleted.IsTerminal() {
		t.Error("StepCompleted should be terminal")
	}
	if !StepRolledBack.IsTerminal() {
		t.Error("StepRolledBack should be terminal")
	}
	if StepIdle.IsTerminal() {
		t.Error("StepIdle should not be terminal")
	}

	// Verify step encoding.
	for _, s := range steps {
		v := s.AsUint64()
		decoded, err := FromUint64(v)
		if err != nil {
			t.Errorf("FromUint64(%d) error: %v", v, err)
		}
		if decoded != s {
			t.Errorf("FromUint64(%d) = %v, want %v", v, decoded, s)
		}
	}

	// Verify rollback reasons.
	reasons := []RollbackReason{
		RollbackTargetTCKFailed,
		RollbackSemanticDiff,
		RollbackObserveWindowDiff,
	}
	for _, r := range reasons {
		if err := r.Valid(); err != nil {
			t.Errorf("RollbackReason %q valid error: %v", r, err)
		}
	}
}
