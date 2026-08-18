// Package memstore provides an in-memory implementation of ports.ProposalStore.
// It is not durable and is intended for testing and development (ADR-065).
package memstore

import (
	"context"
	"errors"
	"sync"

	"github.com/Rubentxu/golem/internal/application/proposal"
	"github.com/Rubentxu/golem/internal/ports"
)

// Store implements ports.ProposalStore using in-memory maps.
// Journal source-of-truth: all proposal events are appended to the journal slice
// and the store maintains a projection as the authoritative proposal state.
type Store struct {
	mu    sync.RWMutex
	props map[string]ports.Proposal // id → Proposal
}

// NewStore creates an empty in-memory proposal store.
func NewStore() *Store {
	return &Store{
		props: make(map[string]ports.Proposal),
	}
}

// Append implements ports.ProposalStore.Append.
func (s *Store) Append(ctx context.Context, p ports.Proposal) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.props[p.ID]; exists {
		return errors.New("memstore: proposal already exists")
	}
	s.props[p.ID] = p
	return nil
}

// Get implements ports.ProposalStore.Get.
func (s *Store) Get(ctx context.Context, id string) (ports.Proposal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if p, ok := s.props[id]; ok {
		return p, nil
	}
	return ports.Proposal{}, proposal.ErrProposalNotFound
}

// List implements ports.ProposalStore.List.
func (s *Store) List(ctx context.Context, tenantID string) ([]ports.Proposal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []ports.Proposal
	for _, p := range s.props {
		if p.TenantID == ports.TenantID(tenantID) {
			result = append(result, p)
		}
	}
	return result, nil
}

// Replay implements ports.ProposalStore.Replay.
// In this in-memory implementation, it returns the same as List.
func (s *Store) Replay(ctx context.Context, tenantID string) ([]ports.Proposal, error) {
	return s.List(ctx, tenantID)
}

// UpdateStatus implements ports.ProposalStore.UpdateStatus with optimistic locking.
func (s *Store) UpdateStatus(ctx context.Context, id string, expectedRevision uint64, newStatus string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.props[id]
	if !ok {
		return proposal.ErrProposalNotFound
	}
	if p.Revision != expectedRevision {
		return ports.ErrVersionConflict
	}
	p.Status = newStatus
	p.Revision++
	s.props[id] = p
	return nil
}

// Ensure Store implements ports.ProposalStore
var _ ports.ProposalStore = (*Store)(nil)
