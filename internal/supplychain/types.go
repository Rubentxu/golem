// Package supplychain provides domain types and utilities for the supply chain
// security kernel: purl normalization, SBOM events, and graph-model constants.
package supplychain

// Node kind constants used by the supply chain projection.
const (
	KindSBOM             = "SBOM"
	KindPackageComponent = "PackageComponent"
	KindVulnerability     = "Vulnerability"
	KindVEXStatement     = "VEXStatement"
	KindAttestation      = "Attestation"
)

// Relation type constants used in supply chain graph edges.
// These belong to the canonical GOLEM ontology (GRAPH_MODEL).
const (
	RelationHAS_SBOM      = "HAS_SBOM"
	RelationCONTAINS      = "CONTAINS"
	RelationAFFECTED_BY   = "AFFECTED_BY"
	RelationMITIGATED_BY  = "MITIGATED_BY"
	RelationATTESTED_BY   = "ATTESTED_BY"
	RelationSIGNED_BY     = "SIGNED_BY"
)

// Severity constants.
const (
	SeverityLow      = "low"
	SeverityMedium    = "medium"
	SeverityHigh      = "high"
	SeverityCritical  = "critical"
)

// VulnerabilityStatus constants.
const (
	StatusOpen     = "open"
	StatusFixed     = "fixed"
	StatusDisputed  = "disputed"
)

// VEXStatus constants.
const (
	VEXStatusNotAffected    = "not_affected"
	VEXStatusAffected       = "affected"
	VEXStatusFixed          = "fixed"
	VEXStatusInRemediation  = "in_remediation"
)

// VerificationResult constants.
const (
	VerificationVerified = "verified"
	VerificationPending  = "pending"
	VerificationFailed   = "failed"
)
