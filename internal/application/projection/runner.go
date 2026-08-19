// Package projection translates accepted journal events into graph mutations.
package projection

import (
	"context"
	"fmt"

	"github.com/Rubentxu/golem/internal/ports"
)

// Runner drives the projection tail loop with per-event cursor advance.
// It applies events to the graph and advances the checkpoint for every event
// consumed, regardless of whether the projector produced a mutation.
type Runner struct {
	Projector  Projector
	Graph      ports.GraphStore
	Checkpoint ports.CheckpointStore
	Logger     Logger
	BatchSize  int
}

// Logger is the observability logger interface used by Runner.
type Logger interface {
	Info(ctx context.Context, msg string, attrs ...ports.Attr)
	Error(ctx context.Context, msg string, attrs ...ports.Attr)
}

// Result reports the outcome of projecting one event.
type Result struct {
	// Applied is true when the projector produced ops and they were applied.
	// Applied is false for unknown events (no projector handles them) and
	// for no-op mutations (projector handled but produced empty ops).
	// In both cases the runner consumed the event and checkpoint advances.
	Applied bool
	// Chunks is the number of mutation chunks applied (0 when Applied=false).
	Chunks int
	// Err is non-nil when the runner encountered a fatal error.
	// On Err, the batch aborts and the caller should retry from the
	// last successful checkpoint.
	Err error
}

// ProjectBatch projects one batch of events from the journal, starting at the
// saved checkpoint. Returns the number of events processed and any error.
// The checkpoint advances for every event consumed, even unknown events and
// no-op mutations.
func (r *Runner) ProjectBatch(ctx context.Context, jrnl JournalReplayer, batchSize int) (int, error) {
	if batchSize <= 0 {
		batchSize = r.BatchSize
	}
	if batchSize <= 0 {
		batchSize = 100
	}

	from, err := r.Checkpoint.Load(ctx, ProjectionCheckpoint)
	if err != nil {
		return 0, fmt.Errorf("runner checkpoint load: %w", err)
	}

	batch, _, err := jrnl.Replay(ctx, from, batchSize)
	if err != nil {
		return 0, fmt.Errorf("runner replay from %d: %w", from, err)
	}
	if len(batch) == 0 {
		return 0, nil
	}

	savedCheckpoint := from
	for i, env := range batch {
		result := r.Run(ctx, env)
		if result.Err != nil {
			// Save what we have so far, return error
			if saveErr := r.Checkpoint.Save(ctx, ProjectionCheckpoint, savedCheckpoint); saveErr != nil {
				return i, fmt.Errorf("runner checkpoint save after error: %w", saveErr)
			}
			return i, result.Err
		}
		// ALWAYS advance checkpoint for every event consumed
		savedCheckpoint = from + ports.StreamPosition(i) + 1
	}

	if err := r.Checkpoint.Save(ctx, ProjectionCheckpoint, savedCheckpoint); err != nil {
		return len(batch), fmt.Errorf("runner checkpoint save: %w", err)
	}
	return len(batch), nil
}

// Run projects one event. It always consumes the event (checkpoint advances).
// Returns Result.Applied=false for unknown events and no-op mutations.
func (r *Runner) Run(ctx context.Context, env ports.RawEvent) Result {
	mutation, projected, err := r.tryProject(env)
	if err != nil {
		return Result{Err: err}
	}

	if !projected {
		// Unknown event type — runner consumed it, checkpoint advances
		return Result{Applied: false, Chunks: 0}
	}

	if len(mutation.Ops) == 0 {
		// Projector handled but produced no ops — runner consumed it, checkpoint advances
		return Result{Applied: false, Chunks: 0}
	}

	// Apply all chunks concatenated into one mutation for atomicity
	chunks, err := r.Projector.ProjectAll(env)
	if err != nil {
		return Result{Err: err}
	}

	if len(chunks) == 0 {
		return Result{Applied: false, Chunks: 0}
	}

	// Concatenate all ops from all chunks into one mutation
	combined := ports.GraphMutation{
		TenantID: mutation.TenantID,
		Ops:      make([]ports.GraphOp, 0),
	}
	for _, chunk := range chunks {
		combined.Ops = append(combined.Ops, chunk.Ops...)
	}

	if len(combined.Ops) == 0 {
		return Result{Applied: false, Chunks: 0}
	}

	_, err = r.Graph.Apply(ctx, combined)
	if err != nil {
		return Result{Applied: true, Chunks: len(chunks), Err: err}
	}
	return Result{Applied: true, Chunks: len(chunks)}
}

// tryProject wraps Projector.Project and returns (mutation, wasProjected, error).
// wasProjected=false means no projector handled this event type (unknown).
// wasProjected=true with len(mutation.Ops)==0 means projector handled but no-op.
func (r *Runner) tryProject(env ports.RawEvent) (ports.GraphMutation, bool, error) {
	m, err := r.Projector.Project(env)
	if err != nil {
		return ports.GraphMutation{}, false, err
	}
	// Check if any projector actually handled this event type.
	// A zero mutation (empty Ops) could mean either unknown or no-op.
	// We distinguish by checking if the projector returned a non-empty mutation.
	wasProjected := len(m.Ops) > 0 || isKnownEventType(env.EventType)
	return m, wasProjected, nil
}

// isKnownEventType returns true for event types that the projector handles.
// This is a simple heuristic: if the projector returns an empty mutation for
// a known event type, it means no-op.
func isKnownEventType(eventType string) bool {
	knownPrefixes := []string{
		"work.item.",
		"work.type.",
		"work.item-linked.",
		"requirements.requirement.",
		"planning.iteration.",
		"planning.milestone.",
		"scm.commit.",
		"ci.build.",
		"verification.test-run.",
		"release.candidate.",
		"release.gate.",
		"supply-chain.sbom.",
		"supply-chain.vulnerability.",
		"supply-chain.vex.",
		"supply-chain.attestation.",
	}
	for _, prefix := range knownPrefixes {
		if len(eventType) >= len(prefix) && eventType[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// JournalReplayer is the interface for replaying journal events.
type JournalReplayer interface {
	Replay(ctx context.Context, from ports.StreamPosition, limit int) ([]ports.RawEvent, ports.StreamPosition, error)
}

// ProjectionCheckpoint is the checkpoint key for the projection runner.
const ProjectionCheckpoint = "projection"
