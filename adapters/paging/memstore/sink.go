// Package memstore provides an in-memory reference adapter of the Paging port.
// Used for development and as the TCK baseline (REQ-SLO-004).
package memstore

import (
	"context"
	"log/slog"
	"sync"

	"github.com/Rubentxu/golem/internal/ports"
)

// Store is an in-memory Paging adapter. Pages are logged (development) or
// sent to a configured webhook URL in production.
type Store struct {
	mu     sync.RWMutex
	pages  []ports.Alert
	logger *slog.Logger
}

// NewStore creates a new in-memory paging store.
func NewStore() *Store {
	return &Store{
		pages:  make([]ports.Alert, 0, 100),
		logger: slog.Default(),
	}
}

// NewStoreWithLogger creates a store with a custom logger.
func NewStoreWithLogger(logger *slog.Logger) *Store {
	return &Store{
		pages:  make([]ports.Alert, 0, 100),
		logger: logger,
	}
}

// Page implements ports.Paging. It stores the alert in-memory and logs it.
func (s *Store) Page(ctx context.Context, alert ports.Alert) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pages = append(s.pages, alert)
	s.logger.Warn("SLO alert",
		"severity", alert.Severity,
		"route", alert.Route,
		"message", alert.Message,
		"sli", alert.SLIName,
	)
	return nil
}

// Pages returns all paged alerts (for testing).
func (s *Store) Pages() []ports.Alert {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pages
}

// Clear removes all stored pages (for testing).
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pages = s.pages[:0]
}

// Ensure Store implements ports.Paging
var _ ports.Paging = (*Store)(nil)
