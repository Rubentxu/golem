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

// ArtifactVerifier is the narrow port for v1 artifact verification (VERIFIES edges).
type ArtifactVerifier interface {
	// CheckArtifactVerification walks the artifact's incident VERIFIES edges and
	// checks whether any source TestRun passed.
	CheckArtifactVerification(ctx context.Context, tenant, digest string) bool
}
