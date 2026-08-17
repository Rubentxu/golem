// Package ci defines the CI bounded context: pipelines, builds and jobs
// (BOUNDED_CONTEXTS), including the build→artifact linkage that feeds
// the supply-chain lineage (ADR-022 content addressing).
package ci

// ArtifactOut is one artifact produced by a build.
type ArtifactOut struct {
	Digest string `json:"digest"` // content-addressed identity (sha256:<hex>)
	Name   string `json:"name"`
	Kind   string `json:"kind"` // Artifact | Package | ContainerImage
}

// BuildCompleted is the payload of ci.build.completed.v1. The projector
// materializes the Build node, BUILT_BY (commit→build) and PRODUCED
// (build→artifact) edges, and the Artifact nodes themselves.
type BuildCompleted struct {
	BuildID   string        `json:"build_id"`
	Pipeline  string        `json:"pipeline"`
	Commit    string        `json:"commit"` // sha of the built commit
	Status    string        `json:"status"` // success | failure | unstable
	Artifacts []ArtifactOut `json:"artifacts"`
}

// Event type names of this context.
const (
	EventBuildCompleted = "ci.build.completed.v1"
)
