// Package ci provides narrow-port adapters over the graph store.
package ci

import (
	"context"

	"github.com/Rubentxu/golem/internal/ports"
)

// graphStoreArtifactReader implements ArtifactReader over a GraphStore.
type graphStoreArtifactReader struct {
	gs ports.GraphStore
}

// NewArtifactReaderOverGraphStore creates an ArtifactReader that delegates to gs.
func NewArtifactReaderOverGraphStore(gs ports.GraphStore) ArtifactReader {
	return &graphStoreArtifactReader{gs: gs}
}

// DigestExists implements ArtifactReader by checking if there is an inbound
// PRODUCED edge to the given digest node. A node whose Kind is "ContainerImage"
// but does not have an inbound PRODUCED edge is NOT considered an artifact.
func (r *graphStoreArtifactReader) DigestExists(ctx context.Context, tenant, digest string) (bool, error) {
	sub, err := r.gs.Neighborhood(ctx, ports.NeighborhoodQuery{
		TenantID: ports.TenantID(tenant),
		Roots:    []string{digest},
		MaxDepth: 1,
		MaxNodes: 64,
		MaxEdges: 64,
	})
	if err != nil {
		return false, err
	}
	for _, e := range sub.Edges {
		if e.Type == "PRODUCED" && e.TargetID == digest {
			return true, nil
		}
	}
	return false, nil
}
