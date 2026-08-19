package memstore

import (
	"context"
	"errors"
	"sync"

	"github.com/Rubentxu/golem/internal/ports"
)

// ErrTenantNotFound is returned when a tenant does not exist.
var ErrTenantNotFound = errors.New("tenant not found")

// catalog is an in-memory TenantCatalog adapter.
type catalog struct {
	mu sync.RWMutex
	m  map[string]ports.TenantRecord
}

// New creates a new in-memory TenantCatalog.
func New() ports.TenantCatalog {
	return &catalog{m: make(map[string]ports.TenantRecord)}
}

func (c *catalog) Get(ctx context.Context, tenantID string) (ports.TenantRecord, error) {
	_ = ctx
	c.mu.RLock()
	defer c.mu.RUnlock()
	rec, ok := c.m[tenantID]
	if !ok {
		return ports.TenantRecord{}, ErrTenantNotFound
	}
	return rec, nil
}

func (c *catalog) Assign(ctx context.Context, tenantID string, cellID ports.CellID) error {
	_ = ctx
	c.mu.Lock()
	defer c.mu.Unlock()
	rec, ok := c.m[tenantID]
	if !ok {
		rec = ports.TenantRecord{ID: tenantID}
	}
	rec.CellID = cellID
	c.m[tenantID] = rec
	return nil
}

func (c *catalog) List(ctx context.Context, filter ports.TenantFilter) ([]ports.TenantRecord, error) {
	_ = ctx
	c.mu.RLock()
	defer c.mu.RUnlock()
	var result []ports.TenantRecord
	for _, rec := range c.m {
		if filter.CellID != "" && rec.CellID != filter.CellID {
			continue
		}
		if filter.Tier != "" && rec.Tier != filter.Tier {
			continue
		}
		result = append(result, rec)
	}
	return result, nil
}

func (c *catalog) Export(ctx context.Context, tenantID string) ([]byte, error) {
	_ = ctx
	rec, err := c.Get(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return []byte(`{"id":"` + rec.ID + `","cell_id":"` + string(rec.CellID) + `"}`), nil
}

// Ensure catalog implements ports.TenantCatalog.
var _ ports.TenantCatalog = (*catalog)(nil)
