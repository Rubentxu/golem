// Package behavior implements the GOLEM behavior engine v1
// (BEHAVIOR_RUNTIME.md): deterministic and relation behaviors with native
// Go handlers, subscription-indexed and executed over the typed traversal
// substrate. Workflow and agentic kinds arrive in M7.
package behavior

import (
	"context"
	"time"

	"github.com/Rubentxu/golem/internal/lens"
	"github.com/Rubentxu/golem/internal/ports"
)

// Handler is the execution unit of a behavior. It is a pure function of
// its input: external I/O is only reachable through the injected Tool
// ports (v1: none beyond Clock/IDs) — the determinism contract of
// BEHAVIOR_RUNTIME.md §Determinism.
type Handler func(ctx context.Context, in HandlerInput) (HandlerOutput, error)

// HandlerInput is the execution context handed to a behavior handler.
// NOTE (design amendment): these types live in internal/behavior, not
// internal/ports — ports cannot import internal/lens (import cycle).
type HandlerInput struct {
	Event      ports.RawEvent
	LensResult lens.Result // empty when the behavior declares no lens
	Clock      ports.Clock
	IDs        ports.IDGenerator
}

// HandlerOutput carries the observable outcomes a behavior produces.
type HandlerOutput struct {
	Events    []ports.RawEvent
	Proposals []ProposalNote
}

// ProposalNote is a v1 placeholder for proposals (M7: real Change
// Proposal primitives per ADR-038/039).
type ProposalNote struct {
	Title string
	Body  string
}

// Filter is a cheap predicate evaluated before any graph work. It
// compares a single envelope field for equality.
type Filter struct {
	Field string // "type" | "tenant" | "stream"
	Op    string // "==" only in v1
	Value string
}

// Policy bounds one execution.
type Policy struct {
	MaxEventsPerRun int
	Deadline        time.Duration
}

// Budget bounds resource usage per execution (observable outcome when
// exceeded, not a kernel error — BEHAVIOR_RUNTIME.md §Failure model).
type Budget struct {
	MaxOps        int
	MaxWallMillis int
}

// RelationSpec converts an event into traversal roots: relation behaviors
// coordinate on edges, so their graph pattern is rooted at the entities
// the event refers to (e.g. the vulnerability node that just arrived).
type RelationSpec struct {
	RootsFromEvent func(event ports.RawEvent) ([]string, error)
}

// Behavior is the v1 model: id, version, subscriptions, filters, optional
// lens spec, execution policy, budget and handler (BEHAVIOR_RUNTIME.md
// §Behavior).
type Behavior struct {
	ID            string
	Version       string
	Subscriptions []string // event types this behavior observes
	Filters       []Filter
	LensSpec      *lens.Spec // nil = no graph pattern
	Relation      *RelationSpec
	Policy        Policy
	Budget        Budget
	Handler       Handler
}
