// Package memstore provides the in-memory reference adapter of the
// CheckpointStore port. Positions are lost on process restart — safe
// (tailing components replay idempotently) but wasteful; durable
// adapters arrive with SP-002 (journal persistence).
package memstore

import (
	"context"
	"sync"

	"github.com/Rubentxu/golem/internal/ports"
)

// Store is an in-memory keyed CheckpointStore. Safe for concurrent use.
type Store struct {
	mu  sync.Mutex
	pos map[string]ports.StreamPosition
}

// NewCheckpoints builds an empty checkpoint store.
func NewCheckpoints() *Store { return &Store{pos: map[string]ports.StreamPosition{}} }

// Load returns the saved position for key; 0 when never saved.
func (s *Store) Load(_ context.Context, key string) (ports.StreamPosition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pos[key], nil
}

// Save atomically persists the position for key.
func (s *Store) Save(_ context.Context, key string, pos ports.StreamPosition) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pos[key] = pos
	return nil
}
