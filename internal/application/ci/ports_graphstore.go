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

// DigestExists implements ArtifactReader by checking if the artifact node exists.
func (r *graphStoreArtifactReader) DigestExists(ctx context.Context, tenant, digest string) (bool, error) {
	node, err := r.gs.GetNode(ctx, ports.TenantID(tenant), digest)
	if err != nil {
		if err == ports.ErrNodeNotFound {
			return false, nil
		}
		return false, err
	}
	return node.Kind == "Artifact", nil
}
