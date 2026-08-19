// Package projection translates accepted journal events into graph mutations:
// the Engineering Graph is the semantic projection of the Graph Journal.
// Projection Registry enables strangler-fig pattern migrations: a registered
// Projection claims event types, falling back to the legacy switch for
// unclaimed types.
package projection

import (
	"fmt"

	"github.com/Rubentxu/golem/internal/ports"
)

// Projection is the interface for event-type-specific projectors that can
// replace portions of the legacy switch. A Projection claims an event type
// by returning handled=true (including for no-op events).
type Projection interface {
	// Domain returns the bounded context name (e.g. "supplychain", "work").
	Domain() string
	// EventTypes returns the event type names this Projection claims.
	EventTypes() []string
	// Handle processes one event. It returns (mutation, handled=true, err)
	// when it claims this event type, or (zero, false, nil) when the caller
	// should fall through to legacy handling.
	Handle(env ports.RawEvent) (mutation ports.GraphMutation, handled bool, err error)
}

// Registry tracks registered Projections and dispatches events to the first
// Projection that claims them. It is the strangler-dispatcher: legacy switch
// is the fallback for unclaimed event types.
type Registry struct {
	projections []Projection
	eventIndex  map[string]Projection // eventType -> Projection (last writer wins for duplicates)
}

// globalRegistry is the package-level registry. nil means all events fall
// through to the legacy switch (backward-compatible zero-value).
var globalRegistry *Registry

// NewRegistry creates a fresh Registry with no registered Projections.
func NewRegistry() *Registry {
	return &Registry{
		projections: nil,
		eventIndex:  make(map[string]Projection),
	}
}

// Register adds p to the registry. It returns an error if any EventType in p
// is already claimed by another registered Projection (no two Projections may
// claim the same event type).
func (r *Registry) Register(p Projection) error {
	for _, et := range p.EventTypes() {
		if existing, ok := r.eventIndex[et]; ok {
			return fmt.Errorf("projection.Registry: event type %q already registered by %q (cannot re-register for %q)",
				et, existing.Domain(), p.Domain())
		}
	}
	for _, et := range p.EventTypes() {
		r.eventIndex[et] = p
	}
	r.projections = append(r.projections, p)
	return nil
}

// Handle dispatches env to the first registered Projection that claims its
// EventType. Returns (mutation, handled=true, err) when a Projection claims
// the event, or (zero mutation, handled=false, nil) when no registered
// Projection claims it (caller should fall through to legacy switch).
//
// handled=true is returned even when the mutation is empty (no-op event);
// this is the C3 signal that the event was intentionally processed.
func (r *Registry) Handle(env ports.RawEvent) (mutation ports.GraphMutation, handled bool, err error) {
	if r == nil {
		return ports.GraphMutation{}, false, nil
	}
	p, ok := r.eventIndex[env.EventType]
	if !ok {
		return ports.GraphMutation{}, false, nil
	}
	return p.Handle(env)
}

// SetGlobal sets the global registry. nil clears it (all events fall through
// to legacy switch). This is not thread-safe; call before serving requests.
func SetGlobal(r *Registry) {
	globalRegistry = r
}

// Global returns the current global registry.
func Global() *Registry {
	return globalRegistry
}
