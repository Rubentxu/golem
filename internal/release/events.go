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
// For artifactless evidence (v1 contract) the Verified field reflects the
// original VERIFIES walk result.
type GateDetail struct {
	Artifact string `json:"artifact"`
	Verified bool   `json:"verified"`
}

// ArtifactEvidence captures supply-chain evidence for one artifact under
// the supply-chain-gate-v1 policy.
type ArtifactEvidence struct {
	ArtifactDigest string `json:"artifact_digest,omitempty"`
	SBOMPresent    bool   `json:"sbom_present"`
	Attestations   struct {
		Verified int `json:"verified"`
		Total    int `json:"total"`
	} `json:"attestations"`
	Vulnerabilities struct {
		Open      int `json:"open"`
		Mitigated int `json:"mitigated"`
	} `json:"vulnerabilities"`
}

// GateEvaluated is the payload of release.gate.evaluated.v1. Result is
// "green" when every artifact has at least one passed test run
// (VERIFIES edge) — the evidence-based production gate seed of M4.
// v2 extends additively with PolicyVersion, per-artifact Evidence, and
// a typed Reasons taxonomy under the supply-chain-gate-v1 policy.
// The v1 Details[] contract is preserved for artifactless evidence.
type GateEvaluated struct {
	ReleaseID     string                      `json:"release_id"`
	Result        string                      `json:"result"` // green | red
	Details       []GateDetail                `json:"details"`
	PolicyVersion string                      `json:"policy_version,omitempty"` // "supply-chain-gate-v1" when supply-chain evidence is evaluated
	Evidence      map[string]ArtifactEvidence `json:"evidence,omitempty"`
	Reasons       []string                    `json:"reasons,omitempty"` // sbom_missing | attestation_unverified | vuln_unmitigated:<id>
}

// Policy version constant for the supply-chain gate v1.
const PolicyVersionSupplyChainGateV1 = "supply-chain-gate-v1"

// Event type names of this context.
const (
	EventCandidateCreated = "release.candidate.created.v1"
	EventGateEvaluated    = "release.gate.evaluated.v1"
)
