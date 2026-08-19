// Package release hosts the application handlers of the Release bounded context.
package release

import "context"

// ReleaseGraphReader is the narrow port for reading release graph data.
type ReleaseGraphReader interface {
	// GetReleaseArtifactDigests returns the artifact digests for a release.
	GetReleaseArtifactDigests(ctx context.Context, tenant, releaseID string) ([]string, error)

	// NodeExists returns true if a node with the given ID exists.
	NodeExists(ctx context.Context, tenant, nodeID string) (bool, error)
}
