// Package memstore provides the in-memory reference adapter of the
// CommandRegistry port: the command-side idempotent inbox (ADR-020).
package memstore

import (
	"context"
	"sync"

	"github.com/Rubentxu/golem/internal/ports"
)

// Registry is an in-memory CommandRegistry. Safe for concurrent use.
type Registry struct {
	mu       sync.Mutex
	receipts map[string]ports.CommandReceipt
}

// NewRegistry builds an empty command registry.
func NewRegistry() *Registry {
	return &Registry{receipts: map[string]ports.CommandReceipt{}}
}

// Find returns the stored receipt; found is false for unknown commands.
func (r *Registry) Find(ctx context.Context, commandID string) (ports.CommandReceipt, bool, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	receipt, ok := r.receipts[commandID]
	return receipt, ok, nil
}

// Save atomically registers a receipt; a repeated command_id yields
// ErrDuplicateCommand.
func (r *Registry) Save(ctx context.Context, receipt ports.CommandReceipt) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.receipts[receipt.CommandID]; exists {
		return ports.ErrDuplicateCommand
	}
	r.receipts[receipt.CommandID] = receipt
	return nil
}
