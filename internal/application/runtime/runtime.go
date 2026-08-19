// Package runtime is the composition wiring for single-process GOLEM
// deployments (ARCHITECTURE evolution stage A: modular monolith —
// boundaries are kept even when sharing a process). It owns the
// journal-tail loops: the graph projector and the outbox publisher, each
// with an independent checkpoint.
//
// Dependency direction (ADR-047): this package only knows ports. The
// concrete adapters are injected by the host binary (cmd/golem-api,
// cmd/golem-worker), which is where vendor implementations are selected.
//
// Rebuildability note (ADR-049): the graph projection applies batches of
// events and saves its checkpoint after each batch. A crash inside a
// batch re-applies that batch on recovery; node upserts are idempotent
// but edge revisions may advance on re-apply. Pure journal replay (fresh
// store) remains exactly the reference digest — recovery drift is bounded
// to one batch and disappears on the next full rebuild.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Rubentxu/golem/internal/application/command"
	"github.com/Rubentxu/golem/internal/application/outbox"
	"github.com/Rubentxu/golem/internal/application/projection"
	"github.com/Rubentxu/golem/internal/application/search"
	"github.com/Rubentxu/golem/internal/clock"
	"github.com/Rubentxu/golem/internal/ids"
	"github.com/Rubentxu/golem/internal/obs"
	"github.com/Rubentxu/golem/internal/ports"
)

// Checkpoint keys of the tail loops.
const (
	ProjectionCheckpoint = "projection"
	PublishCheckpoint    = outbox.CheckpointKey
	SearchCheckpoint     = "search"
)

// ErrMissingDependency reports an unwired port at boot.
var ErrMissingDependency = errors.New("runtime: missing required port in Options")

// Runtime wires the kernel for one process. All fields are exported for
// host binaries (golem-api adds HTTP; golem-worker runs the loops).
type Runtime struct {
	Journal    ports.JournalStore
	Graph      ports.GraphStore
	Registry   ports.CommandRegistry
	Transport  ports.EventTransport
	Checkpoint ports.CheckpointStore
	Search     ports.SearchIndex // nil disables the search loop
	Bus        *command.Bus
	Projector  projection.Projector
	SearchProj search.Projector
	Clock      ports.Clock
	IDs        ports.IDGenerator
	obs        ports.Observability

	// M8 ports: nil-safe (optional like Search)
	CellRouter    ports.CellRouter
	TenantCatalog ports.TenantCatalog
	QuotaEnforcer ports.QuotaEnforcer
	UsageMeter    ports.UsageMeter
	SLOTracker    ports.SLOTracker
	Paging        ports.Paging
	AuthN         ports.AuthN
	PackRegistry  ports.PackRegistry

	// mu serializes the checkpoint read-process-write cycle in ProjectBatch
	// to prevent races between the background tail loop and explicit drain().
	mu sync.Mutex
	// graphMu protects rt.Graph during SwapGraph cutover. Readers use RLock
	// so they proceed concurrently; writers (SwapGraph) use Lock.
	graphMu sync.RWMutex
}

// Options wires a Runtime. Journal, Graph, Registry, Transport and
// Checkpoint are required (the host selects adapters, ADR-045/047);
// Clock and IDs default to the reference implementations. Obs is the
// observability bundle (zero value = no-ops). Search is optional: nil
// disables the search tail loop (search is a derived projection,
// ADR-015). LLM, Policy, and Budgets are optional and set by
// bootstrap for agentic behaviors (M7). M8 ports (CellRouter,
// TenantCatalog, QuotaEnforcer, UsageMeter, SLOTracker, Paging,
// AuthN, PackRegistry) are nil-safe and default to no-op or memstore
// implementations per profile.
type Options struct {
	Journal    ports.JournalStore
	Graph      ports.GraphStore
	Registry   ports.CommandRegistry
	Transport  ports.EventTransport
	Checkpoint ports.CheckpointStore
	Search     ports.SearchIndex
	Clock      ports.Clock
	IDs        ports.IDGenerator
	Obs        ports.Observability
	// LLM is the LLM provider for agentic behaviors (M7).
	LLM ports.LLMProvider
	// Policy is the policy evaluator for agentic behaviors (M7).
	Policy ports.PolicyEvaluator
	// Budgets maps budget name to BudgetLimits for agentic behaviors (M7).
	Budgets map[string]ports.BudgetLimits

	// M8 ports: nil-safe
	CellRouter    ports.CellRouter
	TenantCatalog ports.TenantCatalog
	QuotaEnforcer ports.QuotaEnforcer
	UsageMeter    ports.UsageMeter
	SLOTracker    ports.SLOTracker
	Paging        ports.Paging
	AuthN         ports.AuthN
	PackRegistry  ports.PackRegistry
}

// New composes a runtime. Handlers are registered on rt.Bus by the host
// before serving traffic.
func New(opts Options) (*Runtime, error) {
	switch {
	case opts.Journal == nil, opts.Graph == nil, opts.Registry == nil,
		opts.Transport == nil, opts.Checkpoint == nil:
		return nil, ErrMissingDependency
	}
	clk := opts.Clock
	if clk == nil {
		clk = clock.SystemClock{}
	}
	gen := opts.IDs
	if gen == nil {
		gen = ids.NewGenerator(clk)
	}
	o := obs.Fill(opts.Obs)
	return &Runtime{
		Journal:    opts.Journal,
		Graph:      opts.Graph,
		Registry:   opts.Registry,
		Transport:  opts.Transport,
		Checkpoint: opts.Checkpoint,
		Search:     opts.Search,
		Bus:        command.NewBus(opts.Journal, opts.Registry, gen, clk).WithObservability(o),
		Projector:  projection.Projector{},
		SearchProj: search.Projector{},
		Clock:      clk,
		IDs:        gen,
		obs:        o,
		// M8 ports: nil-safe
		CellRouter:    opts.CellRouter,
		TenantCatalog: opts.TenantCatalog,
		QuotaEnforcer: opts.QuotaEnforcer,
		UsageMeter:    opts.UsageMeter,
		SLOTracker:    opts.SLOTracker,
		Paging:        opts.Paging,
		AuthN:         opts.AuthN,
		PackRegistry:  opts.PackRegistry,
	}, nil
}

// SwapGraph atomically replaces rt.Graph under a write lock. It is called
// by the host after consuming a migration.harness.cutover.v1 event.
// Readers of rt.Graph hold graphMu.RLock() at the START of each batch
// (not mid-batch) to see a consistent snapshot.
func (rt *Runtime) SwapGraph(ctx context.Context, newGraph ports.GraphStore) error {
	if newGraph == nil {
		return errors.New("runtime: SwapGraph nil graph")
	}
	rt.graphMu.Lock()
	defer rt.graphMu.Unlock()
	rt.Graph = newGraph
	return nil
}

// WithGraphRLock acquires the graph read lock for the duration of fn.
// It returns the error returned by fn, or nil if fn succeeds.
// The lock is always released, even if fn returns an error.
func (rt *Runtime) WithGraphRLock(ctx context.Context, fn func() error) error {
	rt.graphMu.RLock()
	defer rt.graphMu.RUnlock()
	return fn()
}

// ProjectBatch tails the journal into the graph projection once.
// Returns the number of applied events (0 = caught up).
// Serialized by mu to prevent races between background tail and explicit drain.
// graphMu.RLock is held at the start of the batch (not mid-batch) to ensure
// consistent reads during cutover.
func (rt *Runtime) ProjectBatch(ctx context.Context, batchSize int) (int, error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	from, err := rt.Checkpoint.Load(ctx, ProjectionCheckpoint)
	if err != nil {
		return 0, fmt.Errorf("projection checkpoint: %w", err)
	}
	batch, _, err := rt.Journal.Replay(ctx, from, batchSize)
	if err != nil {
		return 0, fmt.Errorf("projection replay from %d: %w", from, err)
	}
	if len(batch) == 0 {
		return 0, nil
	}

	// graphMu.RLock held from start of batch for consistent snapshot during cutover.
	rt.graphMu.RLock()
	defer rt.graphMu.RUnlock()

	// Track highest checkpoint we can safely advance to: the position of the
	// last event that was successfully applied. If any event fails (returns
	// an error, not just applied=false), we stop advancing and let the next
	// call retry from the last successful position. This prevents infinite
	// retry loops while still tolerating transient "dependency not ready" cases.
	// Event positions are from+1, from+2, ... from+len(batch) (Replay returns
	// events with position > from, in position order).
	savedCheckpoint := from
	for i, env := range batch {
		applied, err := projection.ApplyIfHandled(rt.Projector, rt.Graph, env)
		if err != nil {
			rt.obs.Logger.Error(ctx, "projection apply failed",
				ports.A("event_id", env.EventID), ports.A("error", err.Error()))
			return 0, fmt.Errorf("projection apply %s: %w", env.EventID, err)
		}
		if applied {
			// Only advance savedCheckpoint past events that were actually applied.
			// This is the key to avoiding infinite retry: on next call we
			// retry from savedCheckpoint, not from the batch's last position.
			savedCheckpoint = from + ports.StreamPosition(i) + 1
		}
	}

	if savedCheckpoint > from {
		// Advance checkpoint to the last successfully applied position, not to
		// the batch's nominal last position (which may contain a failing event).
		if err := rt.Checkpoint.Save(ctx, ProjectionCheckpoint, savedCheckpoint); err != nil {
			return 0, fmt.Errorf("projection checkpoint save %d: %w", savedCheckpoint, err)
		}
		rt.obs.Meter.Counter("golem.projection.applied").Add(ctx, int64(savedCheckpoint-from))
		rt.recordLag(ctx, "projection", savedCheckpoint)
		rt.obs.Logger.Info(ctx, "project_batch_done", ports.A("from", from), ports.A("processed", savedCheckpoint-from), ports.A("new_checkpoint", savedCheckpoint))
	}
	return int(savedCheckpoint - from), nil
}

// recordLag tracks how far a tail loop trails the journal head
// (OBSERVABILITY.md: projection lag).
func (rt *Runtime) recordLag(ctx context.Context, loop string, last ports.StreamPosition) {
	head, err := rt.Journal.Head(ctx)
	if err != nil {
		return
	}
	rt.obs.Meter.Histogram("golem.tail.lag").Record(ctx, float64(head-last), ports.A("loop", loop))
}

// PublishBatch pumps the outbox once. Returns published events (0 = caught up).
func (rt *Runtime) PublishBatch(ctx context.Context, batchSize int) (int, error) {
	from, err := rt.Checkpoint.Load(ctx, PublishCheckpoint)
	if err != nil {
		return 0, fmt.Errorf("outbox checkpoint: %w", err)
	}
	n, err := outbox.New(rt.Journal, rt.Transport, rt.Checkpoint).Pump(ctx, batchSize)
	if err == nil && n > 0 {
		if last, err := rt.Checkpoint.Load(ctx, PublishCheckpoint); err == nil {
			rt.obs.Meter.Counter("golem.outbox.published").Add(ctx, int64(n))
			rt.recordLag(ctx, "outbox", last)
			_ = from
		}
	}
	return n, err
}

// SearchBatch tails the journal into the search index once (no-op when
// no SearchIndex is wired). Returns indexed documents (0 = caught up).
func (rt *Runtime) SearchBatch(ctx context.Context, batchSize int) (int, error) {
	if rt.Search == nil {
		return 0, nil
	}
	from, err := rt.Checkpoint.Load(ctx, SearchCheckpoint)
	if err != nil {
		return 0, fmt.Errorf("search checkpoint: %w", err)
	}
	batch, last, err := rt.Journal.Replay(ctx, from, batchSize)
	if err != nil {
		return 0, fmt.Errorf("search replay from %d: %w", from, err)
	}
	if len(batch) == 0 {
		return 0, nil
	}
	indexed := 0
	for _, env := range batch {
		docs, err := rt.SearchProj.Project(env)
		if err != nil {
			rt.obs.Logger.Error(ctx, "search project failed",
				ports.A("event_id", env.EventID), ports.A("error", err.Error()))
			return indexed, fmt.Errorf("search project %s: %w", env.EventID, err)
		}
		if len(docs) == 0 {
			continue
		}
		if err := rt.Search.Index(ctx, docs); err != nil {
			return indexed, fmt.Errorf("search index %s: %w", env.EventID, err)
		}
		indexed += len(docs)
	}
	if err := rt.Checkpoint.Save(ctx, SearchCheckpoint, last); err != nil {
		return indexed, fmt.Errorf("search checkpoint save %d: %w", last, err)
	}
	rt.obs.Meter.Counter("golem.search.indexed").Add(ctx, int64(indexed))
	rt.recordLag(ctx, "search", last)
	return indexed, nil
}

// Run drives both tail loops until ctx is cancelled. Interval paces
// polling between caught-up cycles. Run returns ctx.Err() on graceful
// shutdown.
func (rt *Runtime) Run(ctx context.Context, batchSize int, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			for {
				n, err := rt.ProjectBatch(ctx, batchSize)
				if err != nil {
					return err
				}
				m, err := rt.PublishBatch(ctx, batchSize)
				if err != nil {
					return err
				}
				s, err := rt.SearchBatch(ctx, batchSize)
				if err != nil {
					return err
				}
				if n == 0 && m == 0 && s == 0 {
					break
				}
			}
		}
	}
}
