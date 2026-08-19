// Package ci hosts the application handlers of the CI bounded context.
package ci

import "context"

// ArtifactReader is the narrow port for reading build artifacts.
type ArtifactReader interface {
	// DigestExists returns true if an artifact with the given digest exists.
	DigestExists(ctx context.Context, tenant, digest string) (bool, error)
}
