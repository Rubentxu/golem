// Package cellrouter provides the application service for cell routing.
package cellrouter

import (
	"context"
	"time"

	"github.com/Rubentxu/golem/adapters/cell/staticrouter"
	"github.com/Rubentxu/golem/internal/ports"
)

// Service provides cell routing at the application layer.
type Service struct {
	router *staticrouter.Router
	cache  *staticrouter.Cache
}

// NewService creates a cell routing service.
func NewService(cells []ports.CellID, cacheTTLMs int, cacheSize int) *Service {
	return &Service{
		router: staticrouter.NewRouter(cells),
		cache:  staticrouter.NewCache(
			time.Duration(cacheTTLMs)*time.Millisecond,
			cacheSize,
		),
	}
}

// Route returns the cell for a tenant, using cache when available.
func (s *Service) Route(ctx context.Context, tenantID string) (ports.CellID, error) {
	// Check cache first.
	if cell, ok := s.cache.Get(tenantID); ok {
		return cell, nil
	}
	// Route via router.
	cell, err := s.router.Route(ctx, tenantID)
	if err != nil {
		return "", err
	}
	// Cache result.
	s.cache.Set(tenantID, cell)
	return cell, nil
}

// InvalidateCache removes a tenant from the routing cache.
func (s *Service) InvalidateCache(tenantID string) {
	s.cache.Invalidate(tenantID)
}

// Router returns the underlying router.
func (s *Service) Router() ports.CellRouter {
	return s.router
}
