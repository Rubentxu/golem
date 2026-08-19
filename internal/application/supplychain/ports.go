// Package supplychain hosts the application handlers of the Supply Chain bounded context.
package supplychain

import "context"

// BlastRadiusQuery is the narrow port for querying blast radius of a component.
type BlastRadiusQuery interface {
	// Query returns the blast radius for a given component purl.
	// Returns ErrComponentNotFound if the component does not exist.
	Query(ctx context.Context, tenant, purl string) (*BlastRadiusResult, error)
}

// SupplyChainEvidenceReader is the narrow port for reading supply chain evidence.
// This is used by the release context to collect evidence for gate evaluation.
type SupplyChainEvidenceReader interface {
	// CollectEvidence collects all supply chain evidence for an artifact digest.
	// Returns evidence that can be used for gate evaluation.
	CollectEvidence(ctx context.Context, tenant, artifactDigest string) (Evidence, error)
}

// Evidence holds supply chain evidence collected for gate evaluation.
type Evidence struct {
	SBOMIDs              []string
	TotalAttestations    int
	VerifiedAttestations int
	ComponentPURLs       []string
	VulnerabilityIDs     []string
	OpenVulnIDs          []string
	MitigatedVulnIDs     []string
}
