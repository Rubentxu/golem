// Package harness provides the Migration Rehearsal R4 state machine.
package harness

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/Rubentxu/golem/internal/application/projection"
	"github.com/Rubentxu/golem/internal/canonical"
	"github.com/Rubentxu/golem/internal/clock"
	"github.com/Rubentxu/golem/internal/ids"
	"github.com/Rubentxu/golem/internal/ports"
)

// HarnessEndpoint wraps a graph + journal for the migration harness.
type HarnessEndpoint struct {
	Graph   ports.GraphStore
	Journal ports.JournalStore // read-side for replay
}

// HarnessOptions configures a harness run.
type HarnessOptions struct {
	// ObserveWindow is how long to monitor for new diffs after cutover.
	// Defaults to 5s, tests use 100ms via GOLEM_MIGRATION_OBSERVE_WINDOW.
	ObserveWindow time.Duration
	// DiffPoll is the interval between shadow reads during observe window.
	// Default 250ms.
	DiffPoll time.Duration
	// SampleSeed is the deterministic seed for sampling.
	SampleSeed uint64
	// TenantID is the tenant to migrate.
	TenantID ports.TenantID
	// TargetValidator is an optional TCK-style validation run against the target
	// graph before cutover. If it returns an error, the harness rolls back
	// with RollbackTargetTCKFailed. The harness package cannot import tck/
	// (cycle isolation); callers wire tck.RunGraphStoreTCK through this field.
	TargetValidator func(ctx context.Context, target ports.GraphStore) error
}

// DefaultHarnessOptions returns the standard options for a harness run.
func DefaultHarnessOptions(tenant ports.TenantID) HarnessOptions {
	observeWindow := 5 * time.Second
	if env := os.Getenv("GOLEM_MIGRATION_OBSERVE_WINDOW"); env != "" {
		if d, err := time.ParseDuration(env); err == nil {
			observeWindow = d
		} else {
			log.Printf("invalid GOLEM_MIGRATION_OBSERVE_WINDOW=%s, using default 5s", env)
		}
	}
	poll := 250 * time.Millisecond
	if observeWindow < 500*time.Millisecond {
		poll = 50 * time.Millisecond // faster polling for tests
	}
	return HarnessOptions{
		ObserveWindow: observeWindow,
		DiffPoll:      poll,
		SampleSeed:    42, // deterministic default
		TenantID:      tenant,
	}
}

// Harness orchestrates a migration rehearsal R4.
type Harness struct {
	ID         string
	Journal    ports.JournalStore
	Checkpoint ports.CheckpointStore
	Source     HarnessEndpoint
	Target     HarnessEndpoint
	Options    HarnessOptions
	clk        ports.Clock
	ids        ports.IDGenerator
	// snapshotTar holds the canonical export tar bytes between snapshotting and loading.
	// It is nil until stepSnapshotting completes, and consumed in stepLoading.
	snapshotTar []byte
}

// NewHarness creates a new migration harness.
func NewHarness(id string, journal ports.JournalStore, cp ports.CheckpointStore, source, target HarnessEndpoint, opts HarnessOptions, clk ports.Clock, idgen ports.IDGenerator) *Harness {
	if clk == nil {
		clk = clock.SystemClock{}
	}
	if idgen == nil {
		idgen = ids.NewGenerator(clk)
	}
	return &Harness{
		ID:         id,
		Journal:    journal,
		Checkpoint: cp,
		Source:     source,
		Target:     target,
		Options:    opts,
		clk:        clk,
		ids:        idgen,
	}
}

// stateKey returns the checkpoint key for harness state.
func stateKey(harnessID string) string {
	return fmt.Sprintf("migration.%s.state", harnessID)
}

// StateKey is the public alias for tests.
func StateKey(harnessID string) string { return stateKey(harnessID) }

// cursorKey returns the checkpoint key for harness cursor.
func cursorKey(harnessID string) string {
	return fmt.Sprintf("migration.%s.cursor", harnessID)
}

// Run executes the 9-step migration harness.
// It is resumable: if a prior Run with the same ID was interrupted,
// this run resumes from the last saved step.
func (h *Harness) Run(ctx context.Context) error {
	// Resume from checkpoint if available.
	currentStep, err := h.Checkpoint.Load(ctx, stateKey(h.ID))
	if err != nil {
		return fmt.Errorf("load state checkpoint: %w", err)
	}
	step := StepIdle
	if currentStep > 0 {
		var loadErr error
		step, loadErr = FromUint64(uint64(currentStep))
		if loadErr != nil {
			step = StepIdle // start fresh on corrupted state
		}
		log.Printf("migration harness %s resuming from step %s", h.ID, step)
	}

	if step == StepIdle {
		// Transition idle → snapshotting
		if err := h.transitionTo(ctx, StepSnapshotting); err != nil {
			return err
		}
		step = StepSnapshotting // Update local variable after checkpoint save
	}

	// Execute from current step to completion or rollback.
	for !step.IsTerminal() {
		next, err := h.executeStep(ctx, step)
		if err != nil {
			return err
		}
		if next == step {
			return fmt.Errorf("harness %s: step %s did not advance", h.ID, step)
		}
		step = next
	}
	return nil
}

// executeStep runs one step and returns the next step.
func (h *Harness) executeStep(ctx context.Context, step Step) (Step, error) {
	switch step {
	case StepSnapshotting:
		return h.stepSnapshotting(ctx)
	case StepLoading:
		return h.stepLoading(ctx)
	case StepReplaying:
		return h.stepReplaying(ctx)
	case StepShadowing:
		return h.stepShadowing(ctx)
	case StepDiffing:
		return h.stepDiffing(ctx)
	case StepCutoverPending:
		return h.stepCutoverPending(ctx)
	case StepObserving:
		return h.stepObserving(ctx)
	default:
		return step, fmt.Errorf("harness %s: unexpected step %s", h.ID, step)
	}
}

// transitionTo saves the new step to checkpoint and returns it.
func (h *Harness) transitionTo(ctx context.Context, newStep Step) error {
	if err := h.Checkpoint.Save(ctx, stateKey(h.ID), ports.StreamPosition(newStep.AsUint64())); err != nil {
		return fmt.Errorf("save step %s: %w", newStep, err)
	}
	log.Printf("migration harness %s: %s", h.ID, newStep)
	return nil
}

// stepSnapshotting: canonical export of source.
func (h *Harness) stepSnapshotting(ctx context.Context) (Step, error) {
	audit := newAudit(h.Journal, h.ids, h.clk, h.ID, "memstore", "memstore")
	if err := audit.started(ctx); err != nil {
		return StepRolledBack, fmt.Errorf("audit started: %w", err)
	}

	// Export source to a temp tar.
	exported, err := h.exportSource(ctx)
	if err != nil {
		audit.rolledBack(ctx, RollbackTargetTCKFailed, "snapshotting")
		return StepRolledBack, fmt.Errorf("snapshot export: %w", err)
	}
	log.Printf("migration harness %s: exported snapshot %d nodes %d edges",
		h.ID, exported.Counts.Nodes, exported.Counts.Edges)
	return StepLoading, nil
}

// stepLoading: bulk load the export into target via full reload.
// We clear existing target data first, then load the source snapshot.
// This ensures source and target are byte-identical after loading.
func (h *Harness) stepLoading(ctx context.Context) (Step, error) {
	if h.snapshotTar == nil {
		return StepRolledBack, fmt.Errorf("no snapshot tar — stepSnapshotting must run first")
	}

	// Full reload: clear existing target data, then load source snapshot.
	// After loading, target = source exactly, enabling clean diff.
	tr := tar.NewReader(bytes.NewReader(h.snapshotTar))
	loaded, err := h.copyIntoTarget(ctx, tr)
	if err != nil {
		return StepRolledBack, fmt.Errorf("copy into target: %w", err)
	}
	log.Printf("migration harness %s: loaded %d nodes %d edges into target",
		h.ID, loaded.Nodes, loaded.Edges)
	if err := h.Checkpoint.Save(ctx, cursorKey(h.ID), ports.StreamPosition(1)); err != nil {
		return StepRolledBack, fmt.Errorf("save cursor: %w", err)
	}
	return StepReplaying, nil
}

// stepReplaying: replay journal events from source into target.
//
// This implements the "dual projection" phase of the migration rehearsal:
// after the snapshot, any new events written to the source journal are
// replayed into the target so target stays in sync with source's live state.
// If a source mutation is applied directly (not journaled), replay cannot
// replicate it — the subsequent diffing step will detect the divergence and
// roll back. This is the honest migration divergence scenario.
//
// Contract: after this step, target reflects all journaled events that
// occurred on source after the snapshot position.
func (h *Harness) stepReplaying(ctx context.Context) (Step, error) {
	cursorPos, err := h.Checkpoint.Load(ctx, cursorKey(h.ID))
	if err != nil {
		return StepRolledBack, fmt.Errorf("load cursor: %w", err)
	}

	projector := projection.Projector{}
	for {
		batch, newPos, err := h.Journal.Replay(ctx, cursorPos, 100)
		if err != nil {
			audit := newAudit(h.Journal, h.ids, h.clk, h.ID, "memstore", "memstore")
			audit.rolledBack(ctx, RollbackSemanticDiff, "replaying")
			return StepRolledBack, fmt.Errorf("replay: %w", err)
		}
		if len(batch) == 0 {
			break // caught up — no new events since snapshot
		}

		for i, env := range batch {
			applied, err := projection.ApplyIfHandled(projector, h.Target.Graph, env)
			if err != nil {
				audit := newAudit(h.Journal, h.ids, h.clk, h.ID, "memstore", "memstore")
				audit.rolledBack(ctx, RollbackSemanticDiff, "replaying")
				return StepRolledBack, fmt.Errorf("apply event %s: %w", env.EventID, err)
			}
			if applied {
				cursorPos = newPos
			}
			_ = i // position tracked via newPos
		}
	}

	// Persist cursor so resume is always from the last replayed position.
	if err := h.Checkpoint.Save(ctx, cursorKey(h.ID), cursorPos); err != nil {
		return StepRolledBack, fmt.Errorf("save cursor: %w", err)
	}

	log.Printf("migration harness %s: replayed journal events up to position %d", h.ID, cursorPos)

	// C-3: run TargetValidator (TCK) against target before cutover.
	// This is injected by the caller (e.g. cmd/tck wires tck.RunGraphStoreTCK).
	// On failure the harness rolls back with RollbackTargetTCKFailed.
	// The error is logged but NOT returned — the harness completes gracefully
	// with StepRolledBack as terminal state (matching the other rollback paths).
	if h.Options.TargetValidator != nil {
		if err := h.Options.TargetValidator(ctx, h.Target.Graph); err != nil {
			audit := newAudit(h.Journal, h.ids, h.clk, h.ID, "memstore", "memstore")
			audit.rolledBack(ctx, RollbackTargetTCKFailed, "replaying")
			log.Printf("migration harness %s: target TCK failed: %v — rolling back", h.ID, err)
			return StepRolledBack, nil
		}
		log.Printf("migration harness %s: target TCK passed", h.ID)
	}

	return StepShadowing, nil
}

// stepShadowing: run shadow reads against both graphs.
func (h *Harness) stepShadowing(ctx context.Context) (Step, error) {
	// Shadow reads were done as part of diffing step.
	// This step is present for completeness of the state machine.
	return StepDiffing, nil
}

// stepDiffing: semantic diff between source and target.
func (h *Harness) stepDiffing(ctx context.Context) (Step, error) {
	audit := newAudit(h.Journal, h.ids, h.clk, h.ID, "memstore", "memstore")
	result, err := Diff(ctx, h.Source.Graph, h.Target.Graph, h.Options.TenantID, h.Options.SampleSeed)
	if err != nil {
		audit.rolledBack(ctx, RollbackSemanticDiff, "diffing")
		return StepRolledBack, fmt.Errorf("diff: %w", err)
	}

	if err := audit.diffed(ctx, result.NodeDiffs, result.EdgeDiffs); err != nil {
		return StepRolledBack, fmt.Errorf("audit diffed: %w", err)
	}

	if result.NodeDiffs > 0 || result.EdgeDiffs > 0 {
		log.Printf("migration harness %s: diff detected (%d node diffs, %d edge diffs) — rolling back",
			h.ID, result.NodeDiffs, result.EdgeDiffs)
		audit.rolledBack(ctx, RollbackSemanticDiff, "diffing")
		return StepRolledBack, nil
	}

	log.Printf("migration harness %s: diff clean, proceeding to cutover", h.ID)
	return StepCutoverPending, nil
}

// stepCutoverPending: publish cutover event (host calls runtime.SwapGraph).
func (h *Harness) stepCutoverPending(ctx context.Context) (Step, error) {
	audit := newAudit(h.Journal, h.ids, h.clk, h.ID, "memstore", "memstore")
	// Cutover is host-side: harness publishes the event, host calls SwapGraph.
	// The harness does NOT call runtime.SwapGraph directly (design decision).
	if err := audit.cutover(ctx, true); err != nil {
		return StepRolledBack, fmt.Errorf("audit cutover: %w", err)
	}
	if err := h.transitionTo(ctx, StepObserving); err != nil {
		return StepRolledBack, err
	}
	return StepObserving, nil
}

// stepObserving: monitor for new diffs during observe window.
func (h *Harness) stepObserving(ctx context.Context) (Step, error) {
	audit := newAudit(h.Journal, h.ids, h.clk, h.ID, "memstore", "memstore")
	ticker := time.NewTicker(h.Options.DiffPoll)
	defer ticker.Stop()
	deadline := time.Now().Add(h.Options.ObserveWindow)

	for {
		select {
		case <-ctx.Done():
			return StepRolledBack, ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				// Observe window elapsed with no new diffs → complete.
				if err := audit.completed(ctx); err != nil {
					return StepRolledBack, fmt.Errorf("audit completed: %w", err)
				}
				if err := h.transitionTo(ctx, StepCompleted); err != nil {
					return StepRolledBack, err
				}
				return StepCompleted, nil
			}
			// Quick shadow check.
			result, err := Diff(ctx, h.Source.Graph, h.Target.Graph, h.Options.TenantID, h.Options.SampleSeed)
			if err != nil {
				continue // non-fatal, retry
			}
			if result.NodeDiffs > 0 || result.EdgeDiffs > 0 {
				log.Printf("migration harness %s: observe window diff — rolling back", h.ID)
				audit.rolledBack(ctx, RollbackObserveWindowDiff, "observing")
				if err := h.transitionTo(ctx, StepRolledBack); err != nil {
					return StepRolledBack, err
				}
				return StepRolledBack, nil
			}
		}
	}
}

// exportSource produces a canonical export of the source graph and stores
// the tar bytes in h.snapshotTar for loading into the target.
func (h *Harness) exportSource(ctx context.Context) (*canonical.Manifest, error) {
	var buf bytes.Buffer
	exporter := canonical.Exporter{
		TenantID: h.Options.TenantID,
		Graph:    h.Source.Graph,
		Journal:  h.Source.Journal,
		Out:      &buf,
	}
	manifest, err := exporter.Export(ctx)
	if err != nil {
		return nil, fmt.Errorf("canonical export: %w", err)
	}
	h.snapshotTar = buf.Bytes()
	return &manifest, nil
}

// loadSnapshot loads the canonical export tar into the target graph.
func (h *Harness) loadSnapshot(ctx context.Context) error {
	if h.snapshotTar == nil {
		return fmt.Errorf("no snapshot tar available — stepSnapshotting must run first")
	}
	tr := tar.NewReader(bytes.NewReader(h.snapshotTar))
	reader := canonical.Reader{
		TenantID: h.Options.TenantID,
		Graph:    h.Target.Graph,
	}
	return reader.ReadFromReader(ctx, tr)
}

// copyCounts tracks how many items were loaded.
type copyCounts struct {
	Nodes int
	Edges int
}

// copyIntoTarget reads nodes.jsonl and edges.jsonl from the tar and applies
// them to the target graph (full reload: removes existing data first).
// This ensures source and target are byte-identical after loading.
func (h *Harness) copyIntoTarget(ctx context.Context, tr *tar.Reader) (*copyCounts, error) {
	// First, clear existing target data for this tenant (full reload).
	existingNodes, err := h.Target.Graph.ListNodes(ctx, h.Options.TenantID)
	if err != nil {
		return nil, fmt.Errorf("list existing nodes: %w", err)
	}
	for _, n := range existingNodes {
		_, err := h.Target.Graph.Apply(ctx, ports.GraphMutation{
			TenantID: h.Options.TenantID,
			Ops:      []ports.GraphOp{{Kind: ports.OpRemoveNode, Target: n.ID}},
		})
		if err != nil {
			return nil, fmt.Errorf("remove node %s: %w", n.ID, err)
		}
	}

	c := &copyCounts{}
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		switch hdr.Name {
		case "nodes.jsonl":
			nodes, err := readCanonicalNodes(tr)
			if err != nil {
				return nil, fmt.Errorf("read nodes: %w", err)
			}
			for _, n := range nodes {
				_, err := h.Target.Graph.Apply(ctx, ports.GraphMutation{
					TenantID: h.Options.TenantID,
					Ops: []ports.GraphOp{{Kind: ports.OpUpsertNode, Target: n.ID, Data: map[string]any{
						"kind":       n.Kind,
						"revision":   n.Revision,
						"attributes": n.Attributes,
					}}},
				})
				if err != nil {
					return nil, fmt.Errorf("apply node %s: %w", n.ID, err)
				}
				c.Nodes++
			}
		case "edges.jsonl":
			edges, err := readCanonicalEdges(tr)
			if err != nil {
				return nil, fmt.Errorf("read edges: %w", err)
			}
			for _, e := range edges {
				_, err := h.Target.Graph.Apply(ctx, ports.GraphMutation{
					TenantID: h.Options.TenantID,
					Ops: []ports.GraphOp{{Kind: ports.OpUpsertEdge, Target: e.ID, Data: map[string]any{
						"type":       e.Type,
						"source":     e.SourceID,
						"target":     e.TargetID,
						"revision":   e.Revision,
						"attributes": e.Attributes,
					}}},
				})
				if err != nil {
					return nil, fmt.Errorf("apply edge %s: %w", e.ID, err)
				}
				c.Edges++
			}
		}
	}
	return c, nil
}

// readCanonicalNodes reads canonical nodes from a tar entry positioned at nodes.jsonl.
func readCanonicalNodes(tr *tar.Reader) ([]canonical.CanonicalNode, error) {
	data, err := io.ReadAll(tr)
	if err != nil {
		return nil, err
	}
	return canonical.ParseCanonicalNodes(data)
}

// readCanonicalEdges reads canonical edges from a tar entry positioned at edges.jsonl.
func readCanonicalEdges(tr *tar.Reader) ([]canonical.CanonicalEdge, error) {
	data, err := io.ReadAll(tr)
	if err != nil {
		return nil, err
	}
	return canonical.ParseCanonicalEdges(data)
}

// Rollback restores the source graph. In the R4 harness, this is called
// after a diff failure to restore source to its pre-migration state.
// For in-memory graphs, the source is untouched (we never mutate it during
// the rehearsal). This method is a placeholder for future adapter support.
func (h *Harness) Rollback(ctx context.Context) error {
	audit := newAudit(h.Journal, h.ids, h.clk, h.ID, "memstore", "memstore")
	step := StepRolledBack
	if err := h.transitionTo(ctx, step); err != nil {
		return err
	}
	return audit.rolledBack(ctx, RollbackSemanticDiff, "diffing")
}
