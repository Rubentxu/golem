// Package memstore provides an in-memory CellRouter adapter.
package memstore

import (
	"context"
	"hash/fnv"

	"github.com/Rubentxu/golem/internal/ports"
)

// Router implements ports.CellRouter with in-memory storage.
type Router struct {
	cells []ports.CellID
}

// NewRouter creates a Router with the given cells.
func NewRouter(cells []ports.CellID) *Router {
	return &Router{cells: cells}
}

// Route returns the cell for a tenant using jump hash.
func (r *Router) Route(ctx context.Context, tenantID string) (ports.CellID, error) {
	if len(r.cells) == 0 {
		return "", nil
	}
	h := fnv.New64a()
	h.Write([]byte(tenantID))
	idx := h.Sum64() % uint64(len(r.cells))
	return r.cells[idx], nil
}

// Migrate is not implemented in memstore adapter.
func (r *Router) Migrate(ctx context.Context, plan ports.MigrationPlan) error {
	return nil
}

// List returns all cells.
func (r *Router) List(ctx context.Context) ([]ports.CellRecord, error) {
	records := make([]ports.CellRecord, len(r.cells))
	for i, id := range r.cells {
		records[i] = ports.CellRecord{ID: id, Region: "us-east-1", Status: "healthy"}
	}
	return records, nil
}

// Health returns health for a cell.
func (r *Router) Health(ctx context.Context, cellID ports.CellID) (ports.CellHealth, error) {
	return ports.CellHealth{LagSeconds: 0, JournalHead: 100, Status: "healthy"}, nil
}

// Compile-time interface check.
var _ ports.CellRouter = (*Router)(nil)
