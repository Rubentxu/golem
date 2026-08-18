package behavior

import (
	"fmt"
	"sync"
)

// Registry indexes behaviors by (ID, Version) and by subscription event
// type. In-memory by design (D2: no second provider exists yet — the port
// is born with the second adapter, the ObjectStore pattern).
type Registry struct {
	mu    sync.RWMutex
	byID  map[string]map[string]*Behavior // id → version → behavior
	bySub map[string][]*Behavior          // event type → candidates
}

// NewRegistry builds an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		byID:  map[string]map[string]*Behavior{},
		bySub: map[string][]*Behavior{},
	}
}

// Register indexes a behavior. Two versions of the same ID coexist
// (shadow executions need both).
func (r *Registry) Register(b *Behavior) error {
	if b.ID == "" || b.Version == "" {
		return fmt.Errorf("behavior: id and version are mandatory")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	versions, ok := r.byID[b.ID]
	if !ok {
		versions = map[string]*Behavior{}
		r.byID[b.ID] = versions
	}
	if _, dup := versions[b.Version]; dup {
		return fmt.Errorf("behavior: %s@%s already registered", b.ID, b.Version)
	}
	versions[b.Version] = b
	for _, sub := range b.Subscriptions {
		r.bySub[sub] = append(r.bySub[sub], b)
	}
	return nil
}

// Get returns the behavior registered under (id, version), or nil.
func (r *Registry) Get(id, version string) *Behavior {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if versions, ok := r.byID[id]; ok {
		return versions[version]
	}
	return nil
}

// Candidates returns the behaviors subscribed to an event type. Order is
// insertion order (deterministic for a given registry build).
func (r *Registry) Candidates(eventType string) []*Behavior {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Behavior, len(r.bySub[eventType]))
	copy(out, r.bySub[eventType])
	return out
}
