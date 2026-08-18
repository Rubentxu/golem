package tck

import (
	"context"
	"testing"
	"time"

	"github.com/Rubentxu/golem/adapters/checkpoint/memstore"
	graphmem "github.com/Rubentxu/golem/adapters/graph/memstore"
	journalmem "github.com/Rubentxu/golem/adapters/journal/memstore"
	harnesspkg "github.com/Rubentxu/golem/internal/application/migration/harness"
	"github.com/Rubentxu/golem/internal/clock"
	idgen "github.com/Rubentxu/golem/internal/ids"
	"github.com/Rubentxu/golem/internal/ports"
)

func testHarness(t *testing.T) *harnesspkg.Harness {
	journal := journalmem.NewJournal()
	cp := memstore.NewCheckpoints()
	source := graphmem.NewGraph()
	target := graphmem.NewGraph()
	clk := clock.Fixed(time.Now())
	idgen := idgen.NewGenerator(clk)
	opts := harnesspkg.DefaultHarnessOptions(ports.TenantID("test-tenant"))
	opts.ObserveWindow = 100 * time.Millisecond
	return harnesspkg.NewHarness(t.Name(), journal, cp,
		harnesspkg.HarnessEndpoint{Graph: source, Journal: journal},
		harnesspkg.HarnessEndpoint{Graph: target, Journal: journal},
		opts, clk, idgen)
}

// TestStepEnumMapping verifies the step ↔ uint64 encoding.
func TestStepEnumMapping(t *testing.T) {
	t.Parallel()
	cases := []struct {
		step      harnesspkg.Step
		wantStr   string
		wantUint  uint64
		wantRound bool
	}{
		{harnesspkg.StepIdle, "idle", 0, true},
		{harnesspkg.StepSnapshotting, "snapshotting", 1, true},
		{harnesspkg.StepLoading, "loading", 2, true},
		{harnesspkg.StepReplaying, "replaying", 3, true},
		{harnesspkg.StepShadowing, "shadowing", 4, true},
		{harnesspkg.StepDiffing, "diffing", 5, true},
		{harnesspkg.StepCutoverPending, "cutover-pending", 6, true},
		{harnesspkg.StepObserving, "observing", 7, true},
		{harnesspkg.StepCompleted, "completed", 8, true},
		{harnesspkg.StepRolledBack, "rolled-back", 9, true},
	}
	for _, c := range cases {
		if got := c.step.String(); got != c.wantStr {
			t.Errorf("Step(%d).String() = %q, want %q", c.step, got, c.wantStr)
		}
		if c.wantRound {
			got, err := harnesspkg.FromUint64(c.wantUint)
			if err != nil {
				t.Errorf("FromUint64(%d) error: %v", c.wantUint, err)
			}
			if got != c.step {
				t.Errorf("FromUint64(%d) = %v, want %v", c.wantUint, got, c.step)
			}
		}
		if got := c.step.AsUint64(); got != c.wantUint {
			t.Errorf("Step(%s).AsUint64() = %d, want %d", c.wantStr, got, c.wantUint)
		}
	}
}

// TestStepIsTerminal verifies terminal state detection.
func TestStepIsTerminal(t *testing.T) {
	t.Parallel()
	if !harnesspkg.StepCompleted.IsTerminal() {
		t.Error("StepCompleted should be terminal")
	}
	if !harnesspkg.StepRolledBack.IsTerminal() {
		t.Error("StepRolledBack should be terminal")
	}
	if harnesspkg.StepIdle.IsTerminal() {
		t.Error("StepIdle should not be terminal")
	}
}

// TestRollbackReasonValid verifies rollback reason validation.
func TestRollbackReasonValid(t *testing.T) {
	t.Parallel()
	if err := harnesspkg.RollbackTargetTCKFailed.Valid(); err != nil {
		t.Errorf("RollbackTargetTCKFailed.Valid() = %v, want nil", err)
	}
	if err := harnesspkg.RollbackSemanticDiff.Valid(); err != nil {
		t.Errorf("RollbackSemanticDiff.Valid() = %v, want nil", err)
	}
	if err := harnesspkg.RollbackObserveWindowDiff.Valid(); err != nil {
		t.Errorf("RollbackObserveWindowDiff.Valid() = %v, want nil", err)
	}
}

// TestDefaultHarnessOptions verifies environment variable parsing.
func TestDefaultHarnessOptions(t *testing.T) {
	t.Parallel()
	// Default: 5s when env is not set.
	opts := harnesspkg.DefaultHarnessOptions(ports.TenantID("tenant1"))
	if opts.ObserveWindow != 5*time.Second {
		t.Errorf("default ObserveWindow = %v, want 5s", opts.ObserveWindow)
	}
}

// TestHarnessResumeFromCheckpoint verifies that a harness can resume from checkpoint.
func TestHarnessResumeFromCheckpoint(t *testing.T) {
	ctx := context.Background()
	h := testHarness(t)

	// Save a mid-flight checkpoint (diffing).
	h.Checkpoint.Save(ctx, harnesspkg.StateKey(h.ID), ports.StreamPosition(harnesspkg.StepDiffing.AsUint64()))

	// Run should resume from diffing (no diffs since graphs are empty).
	err := h.Run(ctx)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	// Graph is empty, no diffs → should complete successfully.
}

// TestHarnessNoDiffClean verifies happy path with empty graphs.
func TestHarnessNoDiffClean(t *testing.T) {
	ctx := context.Background()
	h := testHarness(t)

	err := h.Run(ctx)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Verify final state is completed.
	state, _ := h.Checkpoint.Load(ctx, harnesspkg.StateKey(h.ID))
	if state != ports.StreamPosition(harnesspkg.StepCompleted.AsUint64()) {
		t.Errorf("final state = %d, want %d (StepCompleted)", state, harnesspkg.StepCompleted.AsUint64())
	}
}

// TestDiffCleanGraphs verifies diff reports no diffs for identical empty graphs.
func TestDiffCleanGraphs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	g := graphmem.NewGraph()
	opts := harnesspkg.DefaultHarnessOptions(ports.TenantID("tenant1"))

	result, err := harnesspkg.Diff(ctx, g, g, opts.TenantID, opts.SampleSeed)
	if err != nil {
		t.Fatalf("Diff error = %v", err)
	}
	if !result.CutoverSafe {
		t.Errorf("empty vs empty: CutoverSafe = false, want true")
	}
}

// TestHarnessCutoverPendingEmitsEvent verifies that cutover-pending emits the cutover event.
func TestHarnessCutoverPendingEmitsEvent(t *testing.T) {
	ctx := context.Background()
	h := testHarness(t)

	// Seed both graphs identically.
	h.Source.Graph.Apply(ctx, ports.GraphMutation{
		TenantID: ports.TenantID("test-tenant"),
		Ops: []ports.GraphOp{
			{Kind: ports.OpUpsertNode, Target: "n1", Data: map[string]any{"kind": "test"}},
		},
	})
	h.Target.Graph.Apply(ctx, ports.GraphMutation{
		TenantID: ports.TenantID("test-tenant"),
		Ops: []ports.GraphOp{
			{Kind: ports.OpUpsertNode, Target: "n1", Data: map[string]any{"kind": "test"}},
		},
	})

	// Run should complete (no diffs).
	err := h.Run(ctx)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Verify cutover event was journaled.
	events, _, err := h.Journal.Replay(ctx, 0, 0)
	if err != nil {
		t.Fatalf("Replay error = %v", err)
	}
	cutoverSeen := false
	for _, e := range events {
		if e.EventType == ports.EventMigrationHarnessCutover {
			cutoverSeen = true
			break
		}
	}
	if !cutoverSeen {
		t.Error("cutover event not found in journal")
	}
}

// TestDiffWithDivergence verifies diff detects content divergence.
func TestDiffWithDivergence(t *testing.T) {
	ctx := context.Background()
	src := graphmem.NewGraph()
	tgt := graphmem.NewGraph()
	tenantID := ports.TenantID("test-tenant")

	// Same structure, different attributes.
	src.Apply(ctx, ports.GraphMutation{
		TenantID: tenantID,
		Ops: []ports.GraphOp{
			{Kind: ports.OpUpsertNode, Target: "n1", Data: map[string]any{"kind": "test", "attributes": map[string]any{"color": "red"}}},
		},
	})
	tgt.Apply(ctx, ports.GraphMutation{
		TenantID: tenantID,
		Ops: []ports.GraphOp{
			{Kind: ports.OpUpsertNode, Target: "n1", Data: map[string]any{"kind": "test", "attributes": map[string]any{"color": "blue"}}},
		},
	})

	opts := harnesspkg.DefaultHarnessOptions(tenantID)
	result, err := harnesspkg.Diff(ctx, src, tgt, tenantID, opts.SampleSeed)
	if err != nil {
		t.Fatalf("Diff error = %v", err)
	}
	if result.CutoverSafe {
		t.Error("graphs with different attrs: CutoverSafe = true, want false")
	}
	if result.NodeDiffs == 0 {
		t.Error("expected at least 1 node diff")
	}
}
