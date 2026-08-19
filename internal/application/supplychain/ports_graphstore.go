// Package supplychain provides narrow-port adapters over the graph store.
package supplychain

import (
	"context"

	"github.com/Rubentxu/golem/internal/ports"
)

// graphStoreBlastRadiusQuery implements BlastRadiusQuery over a GraphStore.
type graphStoreBlastRadiusQuery struct {
	gs ports.GraphStore
}

// NewBlastRadiusQueryOverGraphStore creates a BlastRadiusQuery that delegates to gs.
func NewBlastRadiusQueryOverGraphStore(gs ports.GraphStore) BlastRadiusQuery {
	return &graphStoreBlastRadiusQuery{gs: gs}
}

// Query implements BlastRadiusQuery by delegating to the package-level BlastRadius function.
func (r *graphStoreBlastRadiusQuery) Query(ctx context.Context, tenant, purl string) (*BlastRadiusResult, error) {
	result, err := BlastRadius(ctx, r.gs, ports.TenantID(tenant), purl)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
