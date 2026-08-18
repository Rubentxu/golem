// Package cellrouter provides the application service for cell routing.
package cellrouter

import (
	"context"
	"sync"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// Service provides cell routing at the application layer.
type Service struct {
	router ports.CellRouter
	cache  *routingCache
}

// routingCache is a simple in-memory cache for routing decisions.
type routingCache struct {
	ttl   time.Duration
	items map[string]cacheEntry
	mu    sync.RWMutex
}

type cacheEntry struct {
	cell    ports.CellID
	expires time.Time
}

// NewRoutingCache creates a new routing cache.
func newRoutingCache(ttl time.Duration, size int) *routingCache {
	return &routingCache{
		ttl:   ttl,
		items: make(map[string]cacheEntry, size),
	}
}

func (c *routingCache) Get(tenantID string) (ports.CellID, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.items[tenantID]
	if !ok || time.Now().After(e.expires) {
		return "", false
	}
	return e.cell, true
}

func (c *routingCache) Set(tenantID string, cell ports.CellID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[tenantID] = cacheEntry{
		cell:    cell,
		expires: time.Now().Add(c.ttl),
	}
}

func (c *routingCache) Invalidate(tenantID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, tenantID)
}

// NewService creates a cell routing service.
func NewService(router ports.CellRouter, cacheTTL time.Duration, cacheSize int) *Service {
	return &Service{
		router: router,
		cache:  newRoutingCache(cacheTTL, cacheSize),
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
