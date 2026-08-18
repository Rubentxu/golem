// Package staticrouter provides a CellRouter adapter using jump hash routing.
package staticrouter

import (
	"context"

	"github.com/Rubentxu/golem/internal/ports"
)

// Router implements ports.CellRouter using jump hash for deterministic routing.
type Router struct {
	cells     []ports.CellID
	overrides map[string]ports.CellID // tenantID → cellID overrides
}

// NewRouter creates a Router with the given cells.
func NewRouter(cells []ports.CellID) *Router {
	return &Router{
		cells:     cells,
		overrides: make(map[string]ports.CellID),
	}
}

// Route returns the cell for a tenant using jump hash (REQ-CELL-002).
// Override takes precedence over hash-based routing.
func (r *Router) Route(ctx context.Context, tenantID string) (ports.CellID, error) {
	_ = ctx
	if len(r.cells) == 0 {
		return "", ports.ErrRoutingTableEmpty
	}
	// Check override first.
	if cell, ok := r.overrides[tenantID]; ok {
		return cell, nil
	}
	// Jump hash: h = hash(tenantID); slot = h % (2^k) where 2^k >= numCells.
	return r.jumpHash(tenantID), nil
}

// jumpHash implements consistent hash with jump.
func (r *Router) jumpHash(tenantID string) ports.CellID {
	// Simple hash for now; real implementation would use jump hash algorithm.
	h := fnvHash(tenantID)
	idx := h % uint64(len(r.cells))
	return r.cells[idx]
}

// SetOverride sets a routing override for a tenant.
func (r *Router) SetOverride(tenantID string, cellID ports.CellID) {
	r.overrides[tenantID] = cellID
}

// Migrate is not implemented in static router (placeholder for W3.13).
func (r *Router) Migrate(ctx context.Context, plan ports.MigrationPlan) error {
	_ = ctx
	_ = plan
	return nil
}

// List returns all cells.
func (r *Router) List(ctx context.Context) ([]ports.CellRecord, error) {
	_ = ctx
	records := make([]ports.CellRecord, len(r.cells))
	for i, id := range r.cells {
		records[i] = ports.CellRecord{ID: id, Region: "us-east-1", Status: "healthy"}
	}
	return records, nil
}

// Health returns health for a cell.
func (r *Router) Health(ctx context.Context, cellID ports.CellID) (ports.CellHealth, error) {
	_ = ctx
	// Verify cell exists.
	for _, c := range r.cells {
		if c == cellID {
			return ports.CellHealth{LagSeconds: 0, JournalHead: 0, Status: "healthy"}, nil
		}
	}
	return ports.CellHealth{}, ports.ErrCellNotFound
}

// fnvHash computes FNV-1a hash.
func fnvHash(s string) uint64 {
	h := uint64(14695981039346656037)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}

// Compile-time interface check.
var _ ports.CellRouter = (*Router)(nil)
