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
}

// Options wires a Runtime. Journal, Graph, Registry, Transport and
// Checkpoint are required (the host selects adapters, ADR-045/047);
// Clock and IDs default to the reference implementations. Obs is the
// observability bundle (zero value = no-ops). Search is optional: nil
// disables the search tail loop (search is a derived projection,
// ADR-015).
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
	}, nil
}

// ProjectBatch tails the journal into the graph projection once.
// Returns the number of applied events (0 = caught up).
func (rt *Runtime) ProjectBatch(ctx context.Context, batchSize int) (int, error) {
	from, err := rt.Checkpoint.Load(ctx, ProjectionCheckpoint)
	if err != nil {
		return 0, fmt.Errorf("projection checkpoint: %w", err)
	}
	batch, last, err := rt.Journal.Replay(ctx, from, batchSize)
	if err != nil {
		return 0, fmt.Errorf("projection replay from %d: %w", from, err)
	}
	if len(batch) == 0 {
		return 0, nil
	}
	for _, env := range batch {
		if _, err := projection.ApplyIfHandled(rt.Projector, rt.Graph, env); err != nil {
			rt.obs.Logger.Error(ctx, "projection apply failed",
				ports.A("event_id", env.EventID), ports.A("error", err.Error()))
			return 0, fmt.Errorf("projection apply %s: %w", env.EventID, err)
		}
	}
	if err := rt.Checkpoint.Save(ctx, ProjectionCheckpoint, last); err != nil {
		return 0, fmt.Errorf("projection checkpoint save %d: %w", last, err)
	}
	rt.obs.Meter.Counter("golem.projection.applied").Add(ctx, int64(len(batch)))
	rt.recordLag(ctx, "projection", last)
	return len(batch), nil
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
