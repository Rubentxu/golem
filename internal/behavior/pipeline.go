package behavior

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/Rubentxu/golem/internal/agent/observability"
	"github.com/Rubentxu/golem/internal/lens"
	"github.com/Rubentxu/golem/internal/ports"
)

// ErrNoHandler is returned when a behavior has no handler wired.
var ErrNoHandler = errors.New("behavior: handler is nil")

// Engine drives the execution pipeline:
//
//	Accepted Event → subscription index → cheap predicates → candidate set
//	→ graph pattern (lens) → handler → events/proposals
type Engine struct {
	registry *Registry
	graph    ports.GraphStore
	clock    ports.Clock
	// agenticCtx provides the injected ports for agentic behaviors.
	// Nil when no agentic behaviors are registered.
	agenticCtx *AgenticContext
}

// EngineOptions extends Engine with optional agentic ports.
type EngineOptions struct {
	// AgenticLLM is the LLM provider for agentic behaviors.
	AgenticLLM ports.LLMProvider
	// AgenticTools are the tools available to agentic behaviors.
	AgenticTools []ports.Tool
	// AgenticFrame is the default frame for agentic behaviors
	// (overridden per-run via context).
	AgenticFrame ports.Frame
	// AgenticBudget is the budget for agentic behaviors.
	AgenticBudget ports.Budget
	// AgenticTracer is the OTel tracer for agentic spans (ADR-068).
	AgenticTracer ports.Tracer
	// AgenticJournal is the journal for agentic events.
	AgenticJournal ports.JournalStore
	// AgenticRedactor redacts PII from prompts/responses (ADR-066).
	AgenticRedactor *observability.Redactor
}

// NewEngine wires the engine. Clock is injected; output event IDs are
// derived deterministically from the triggering event (see runIDs), which
// makes replay byte-reproducible.
func NewEngine(reg *Registry, graph ports.GraphStore, clock ports.Clock) *Engine {
	return &Engine{registry: reg, graph: graph, clock: clock}
}

// NewEngineWithAgentic wires the engine with agentic ports (ADR-070).
func NewEngineWithAgentic(reg *Registry, graph ports.GraphStore, clock ports.Clock, opts EngineOptions) *Engine {
	e := NewEngine(reg, graph, clock)
	if opts.AgenticLLM != nil || len(opts.AgenticTools) > 0 {
		e.agenticCtx = &AgenticContext{
			LLM:      opts.AgenticLLM,
			Tools:    opts.AgenticTools,
			Frame:    opts.AgenticFrame,
			Budget:   opts.AgenticBudget,
			Tracer:   opts.AgenticTracer,
			Journal:  opts.AgenticJournal,
			Redactor: opts.AgenticRedactor,
			Clock:    clock,
		}
	}
	return e
}

// Clock exposes the injected clock (shadow runs re-wire engines with the
// same clock to keep executions comparable).
func (e *Engine) Clock() ports.Clock {
	return e.clock
}

// Outcome is the observable result of executing one behavior for one event.
type Outcome struct {
	BehaviorID string
	Version    string
	Output     HandlerOutput
	// Skipped records a cheap-predicate rejection or budget failure as an
	// observable outcome, never as a kernel error.
	Skipped string
}

// Handle runs the pipeline for one accepted event against every candidate
// behavior, deterministically (candidates in registration order, outputs
// sorted by EventID).
func (e *Engine) Handle(ctx context.Context, event ports.RawEvent) ([]Outcome, error) {
	candidates := e.registry.Candidates(event.EventType)
	if len(candidates) == 0 {
		return nil, nil // no-op (S5)
	}
	outcomes := make([]Outcome, 0, len(candidates))
	run := newRunIDs(event.EventID)
	for _, b := range candidates {
		oc := Outcome{BehaviorID: b.ID, Version: b.Version}
		if skip := reject(b, event); skip != "" {
			oc.Skipped = skip
			outcomes = append(outcomes, oc)
			continue
		}

		in := HandlerInput{Event: event, Clock: e.clock, IDs: run}

		// Execute LensSpec for ALL behaviors (both Agentic and Deterministic)
		// BEFORE the kind dispatch. This fixes C5: Agentic behaviors were
		// skipping lens execution entirely.
		var lensResult *lens.Result
		if b.LensSpec != nil {
			tenant := ports.TenantID(event.TenantID)
			spec := *b.LensSpec
			if b.Relation != nil {
				roots, err := b.Relation.RootsFromEvent(event)
				if err != nil {
					return nil, fmt.Errorf("behavior %s: relation roots: %w", b.ID, err)
				}
				if len(roots) == 0 {
					oc.Skipped = "relation roots empty"
					outcomes = append(outcomes, oc)
					continue
				}
				spec.Roots = roots
			}
			res, err := lens.Execute(ctx, e.graph, tenant, spec)
			if err != nil {
				if errors.Is(err, lens.ErrLensBudgetExceeded) {
					// Observable outcome, not a kernel error.
					oc.Skipped = "lens budget exceeded"
					outcomes = append(outcomes, oc)
					continue
				}
				return nil, fmt.Errorf("behavior %s lens: %w", b.ID, err)
			}
			lensResult = res
		}

		// Dispatch based on behavior Kind (D10).
		if b.IsAgentic() {
			// Agentic behavior: use AgenticHandler with AgenticContext.
			ah := b.AgenticHandler()
			if ah == nil {
				return nil, fmt.Errorf("%w: agentic handler nil for %s@%s", ErrNoHandler, b.ID, b.Version)
			}
			if e.agenticCtx == nil {
				return nil, fmt.Errorf("agentic behavior %s@%s requires agentic context (call NewEngineWithAgentic)", b.ID, b.Version)
			}
			// Inject agentic context with run-specific tenant.
			agentCtx := *e.agenticCtx // copy
			agentCtx.TenantID = ports.TenantID(event.TenantID)
			agentCtx.IDGenerator = run
			agentCtx.LensResult = lensResult // C5: lens result now available to agentic handler
			in.Agentic = &agentCtx
			out, err := ah(ctx, event, &agentCtx)
			if err != nil {
				return nil, fmt.Errorf("behavior %s@%s agentic handler: %w", b.ID, b.Version, err)
			}
			sort.Slice(out.Events, func(i, j int) bool { return out.Events[i].EventID < out.Events[j].EventID })
			oc.Output = out
			outcomes = append(outcomes, oc)
			continue
		}

		// v1 behavior.
		if b.Handler == nil {
			return nil, fmt.Errorf("%w: %s@%s", ErrNoHandler, b.ID, b.Version)
		}
		if lensResult != nil {
			in.LensResult = *lensResult
		}

		out, err := b.Handler(ctx, in)
		if err != nil {
			// Broken invariants or invalid inputs are errors
			// (BEHAVIOR_RUNTIME.md §Failure model).
			return nil, fmt.Errorf("behavior %s@%s handler: %w", b.ID, b.Version, err)
		}
		sort.Slice(out.Events, func(i, j int) bool { return out.Events[i].EventID < out.Events[j].EventID })
		oc.Output = out
		outcomes = append(outcomes, oc)
	}
	return outcomes, nil
}

// reject evaluates the cheap predicates; it returns a non-empty reason
// when the behavior must skip the event.
func reject(b *Behavior, event ports.RawEvent) string {
	for _, f := range b.Filters {
		var actual string
		switch f.Field {
		case "type":
			actual = event.EventType
		case "tenant":
			actual = event.TenantID
		case "stream":
			actual = event.StreamID
		default:
			return "unknown filter field"
		}
		if f.Op != "==" {
			return "unsupported filter op"
		}
		if actual != f.Value {
			return "filter mismatch"
		}
	}
	return ""
}
