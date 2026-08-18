// Package memstore is the reference ScenarioStore adapter: scenarios held
// in memory (the v1 model — file-backed storage arrives in M6.1 behind
// the same port).
package memstore

import (
	"context"
	"errors"
	"sync"

	"github.com/Rubentxu/golem/internal/ports"
)

// ErrNotFound is returned when a scenario does not exist.
var ErrNotFound = errors.New("scenario memstore: not found")

// Store is an in-memory ports.ScenarioStore. Safe for concurrent use.
type Store struct {
	mu    sync.RWMutex
	items map[string]*ports.Scenario
}

// NewStore builds an empty store.
func NewStore() *Store {
	return &Store{items: map[string]*ports.Scenario{}}
}

// Save upserts a scenario.
func (s *Store) Save(_ context.Context, sc *ports.Scenario) error {
	if sc.ID == "" {
		return errors.New("scenario memstore: id is mandatory")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *sc
	s.items[sc.ID] = &cp
	return nil
}

// Load returns the scenario or ErrNotFound.
func (s *Store) Load(_ context.Context, id string) (*ports.Scenario, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sc, ok := s.items[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *sc
	return &cp, nil
}

// Delete removes a scenario (idempotent).
func (s *Store) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, id)
	return nil
}
