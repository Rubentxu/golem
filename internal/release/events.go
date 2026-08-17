// Package release defines the Release bounded context: release
// candidates, their artifact composition and evidence gates
// (BOUNDED_CONTEXTS; openapi: /releases).
package release

// CandidateCreated is the payload of release.candidate.created.v1.
// Artifacts are content-addressed digests (ADR-022); the projector adds
// RELEASED_AS edges from each artifact to the release.
type CandidateCreated struct {
	ReleaseID string   `json:"release_id"`
	Name      string   `json:"name"`
	Artifacts []string `json:"artifacts"`
}

// GateDetail is the per-artifact outcome of a gate evaluation.
type GateDetail struct {
	Artifact string `json:"artifact"`
	Verified bool   `json:"verified"`
}

// GateEvaluated is the payload of release.gate.evaluated.v1. Result is
// "green" when every artifact has at least one passed test run
// (VERIFIES edge) — the evidence-based production gate seed of M4.
type GateEvaluated struct {
	ReleaseID string       `json:"release_id"`
	Result    string       `json:"result"` // green | red
	Details   []GateDetail `json:"details"`
}

// Event type names of this context.
const (
	EventCandidateCreated = "release.candidate.created.v1"
	EventGateEvaluated    = "release.gate.evaluated.v1"
)
