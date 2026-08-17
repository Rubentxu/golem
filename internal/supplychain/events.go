// Package supplychain defines the supply chain security bounded context:
// SBOM, vulnerability, VEX, and attestation domain events and graph-model
// constants (BOUNDED_CONTEXTS).
package supplychain

import "encoding/json"

// SBOMComponent describes one software component parsed from an SBOM document.
type SBOMComponent struct {
	Purl      string `json:"purl,omitempty"`
	Name      string `json:"name,omitempty"`
	Version   string `json:"version,omitempty"`
	Synthetic bool   `json:"synthetic,omitempty"` // true when purl was derived from name+version+ecosystem
}

// SBOMIngested is the payload of supplychain.sbom.ingested.v1.
type SBOMIngested struct {
	SBOMID         string          `json:"sbom_id"` // sbm-<document-digest-hex>
	Format         string          `json:"format"`  // spdx-2.3 | spdx-3.0 | cyclonedx-1.5 | cyclonedx-1.6
	SpecVersion    string          `json:"spec_version"`
	ArtifactDigest string          `json:"artifact_digest"` // sha256:... of the described artifact
	Components     []SBOMComponent `json:"components"`
	SourceProvider string          `json:"source_provider"`
	SourceDocID    string          `json:"source_doc_id"`
}

// VulnerabilityReported is the payload of supplychain.vulnerability.reported.v1.
type VulnerabilityReported struct {
	VulnID        string `json:"vuln_id"`        // CVE-... or GHSA-...
	Severity      string `json:"severity"`       // low | medium | high | critical
	Status        string `json:"status"`         // open | fixed | disputed
	ComponentPurl string `json:"component_purl"` // normalized purl
	Provider      string `json:"provider"`
}

// VEXStatementRecorded is the payload of supplychain.vex.statement.v1.
type VEXStatementRecorded struct {
	StatementID   string `json:"statement_id"`
	VulnID        string `json:"vuln_id"`
	ProductDigest string `json:"product_digest,omitempty"` // artifact digest when referencing artifact
	ProductPurl   string `json:"product_purl,omitempty"`   // purl when referencing component
	Status        string `json:"status"`                   // not_affected | affected | fixed | in_remediation
	Justification string `json:"justification"`
	Provider      string `json:"provider"`
}

// AttestationIngested is the payload of supplychain.attestation.ingested.v1.
type AttestationIngested struct {
	AttestationID      string          `json:"attestation_id"`
	ArtifactDigest     string          `json:"artifact_digest"`
	PredicateType      string          `json:"predicate_type"` // slsa-provenance | intoto-statement | intoto-link
	BuilderID          string          `json:"builder_id"`
	VerificationResult string          `json:"verification_result"` // verified | pending | failed
	VerificationReason string          `json:"verification_reason"`
	Statement          json.RawMessage `json:"statement,omitempty"`
}

// Event type names of this context.
const (
	EventSBOMIngested          = "supplychain.sbom.ingested.v1"
	EventVulnerabilityReported = "supplychain.vulnerability.reported.v1"
	EventVEXStatementRecorded  = "supplychain.vex.statement.v1"
	EventAttestationIngested   = "supplychain.attestation.ingested.v1"
)
