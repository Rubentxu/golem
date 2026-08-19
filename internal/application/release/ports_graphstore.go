// Package release provides narrow-port adapters over the graph store.
package release

import (
	"context"

	"github.com/Rubentxu/golem/internal/application/supplychain"
	"github.com/Rubentxu/golem/internal/ports"
	domainsupplychain "github.com/Rubentxu/golem/internal/supplychain"
)

// graphStoreReleaseGraphReader implements ReleaseGraphReader over a GraphStore.
type graphStoreReleaseGraphReader struct {
	gs ports.GraphStore
}

// NewReleaseGraphReaderOverGraphStore creates a ReleaseGraphReader that delegates to gs.
func NewReleaseGraphReaderOverGraphStore(gs ports.GraphStore) ReleaseGraphReader {
	return &graphStoreReleaseGraphReader{gs: gs}
}

// GetReleaseArtifactDigests implements ReleaseGraphReader by getting the release node
// and extracting the artifacts attribute.
func (r *graphStoreReleaseGraphReader) GetReleaseArtifactDigests(ctx context.Context, tenant, releaseID string) ([]string, error) {
	node, err := r.gs.GetNode(ctx, ports.TenantID(tenant), releaseID)
	if err != nil {
		return nil, err
	}
	rawArtifacts, ok := node.Attributes["artifacts"].([]any)
	if !ok {
		return nil, nil
	}
	artifacts := make([]string, 0, len(rawArtifacts))
	for _, ra := range rawArtifacts {
		if digest, ok := ra.(string); ok {
			artifacts = append(artifacts, digest)
		}
	}
	return artifacts, nil
}

// NodeExists implements ReleaseGraphReader by checking if the node exists.
func (r *graphStoreReleaseGraphReader) NodeExists(ctx context.Context, tenant, nodeID string) (bool, error) {
	_, err := r.gs.GetNode(ctx, ports.TenantID(tenant), nodeID)
	if err != nil {
		if err == ports.ErrNodeNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// graphStoreSupplyChainEvidenceReader implements SupplyChainEvidenceReader and ArtifactVerifier over a GraphStore.
type graphStoreSupplyChainEvidenceReader struct {
	gs ports.GraphStore
}

// NewSupplyChainEvidenceReaderOverGraphStore creates a SupplyChainEvidenceReader that delegates to gs.
func NewSupplyChainEvidenceReaderOverGraphStore(gs ports.GraphStore) supplychain.SupplyChainEvidenceReader {
	return &graphStoreSupplyChainEvidenceReader{gs: gs}
}

// NewArtifactVerifierOverGraphStore creates an ArtifactVerifier that delegates to gs.
func NewArtifactVerifierOverGraphStore(gs ports.GraphStore) ArtifactVerifier {
	return &graphStoreSupplyChainEvidenceReader{gs: gs}
}

// CollectEvidence implements SupplyChainEvidenceReader by walking supply-chain edges from an artifact.
//
// Walk path: artifact → HAS_SBOM → SBOM → CONTAINS → PackageComponent → AFFECTED_BY → Vulnerability
// Separate walk: artifact → ATTESTED_BY → Attestation
// Mitigation check: vulnerability → MITIGATED_BY → VEXStatement
func (r *graphStoreSupplyChainEvidenceReader) CollectEvidence(ctx context.Context, tenant, artifactDigest string) (supplychain.Evidence, error) {
	ev := supplyChainEvidence{}
	tenantID := ports.TenantID(tenant)

	// Walk 1: Find SBOMs attached to this artifact.
	sbomWalk, _ := r.gs.Traversal(ctx, ports.TraversalQuery{
		TenantID:  tenantID,
		Roots:     []string{artifactDigest},
		EdgeTypes: []string{domainsupplychain.RelationHAS_SBOM},
		Kinds:     []string{domainsupplychain.KindSBOM},
		MaxDepth:  1,
		MaxNodes:  100,
		MaxEdges:  200,
	})
	for _, n := range sbomWalk.Nodes {
		if n.ID != artifactDigest {
			ev.SBOMIDs = append(ev.SBOMIDs, n.ID)
		}
	}

	// Walk 2: Walk artifact → ATTESTED_BY → Attestation.
	attWalk, _ := r.gs.Traversal(ctx, ports.TraversalQuery{
		TenantID:  tenantID,
		Roots:     []string{artifactDigest},
		EdgeTypes: []string{domainsupplychain.RelationATTESTED_BY},
		Kinds:     []string{domainsupplychain.KindAttestation},
		MaxDepth:  1,
		MaxNodes:  100,
		MaxEdges:  200,
	})
	ev.TotalAttestations = len(attWalk.Nodes) - 1 // subtract root artifact node
	if ev.TotalAttestations < 0 {
		ev.TotalAttestations = 0
	}
	for _, n := range attWalk.Nodes {
		if n.ID == artifactDigest {
			continue
		}
		if ver, _ := n.Attributes["verification"].(string); ver == domainsupplychain.VerificationVerified {
			ev.VerifiedAttestations++
		}
	}

	if len(ev.SBOMIDs) == 0 {
		// No SBOM means no vulnerability walk needed
		return toSupplyChainEvidence(ev), nil
	}

	// Walk 3: For each SBOM, walk SBOM → CONTAINS → PackageComponent.
	componentIDs := []string{}
	for _, sbomID := range ev.SBOMIDs {
		compWalk, _ := r.gs.Traversal(ctx, ports.TraversalQuery{
			TenantID:  tenantID,
			Roots:     []string{sbomID},
			EdgeTypes: []string{domainsupplychain.RelationCONTAINS},
			Kinds:     []string{domainsupplychain.KindPackageComponent},
			MaxDepth:  1,
			MaxNodes:  500,
			MaxEdges:  1000,
		})
		for _, n := range compWalk.Nodes {
			if n.ID != sbomID {
				componentIDs = append(componentIDs, n.ID)
			}
		}
	}

	// Walk 4: For each component, walk COMPONENT → AFFECTED_BY → Vulnerability.
	vulnSet := map[string]bool{}
	for _, compID := range componentIDs {
		vulnWalk, _ := r.gs.Traversal(ctx, ports.TraversalQuery{
			TenantID:  tenantID,
			Roots:     []string{compID},
			EdgeTypes: []string{domainsupplychain.RelationAFFECTED_BY},
			Kinds:     []string{domainsupplychain.KindVulnerability},
			MaxDepth:  1,
			MaxNodes:  500,
			MaxEdges:  1000,
		})
		for _, n := range vulnWalk.Nodes {
			if n.ID != compID {
				vulnSet[n.ID] = true
			}
		}
	}

	for vulnID := range vulnSet {
		ev.VulnIDs = append(ev.VulnIDs, vulnID)
	}

	// Walk 5: For each vulnerability, check MITIGATED_BY → VEXStatement.
	mitigatedSet := map[string]bool{}
	for _, vulnID := range ev.VulnIDs {
		mitWalk, _ := r.gs.Traversal(ctx, ports.TraversalQuery{
			TenantID:  tenantID,
			Roots:     []string{vulnID},
			EdgeTypes: []string{domainsupplychain.RelationMITIGATED_BY},
			Kinds:     []string{domainsupplychain.KindVEXStatement},
			MaxDepth:  1,
			MaxNodes:  50,
			MaxEdges:  100,
		})
		for _, n := range mitWalk.Nodes {
			if n.ID != vulnID {
				mitigatedSet[n.ID] = true
			}
		}
	}

	// Classify vulnerabilities: open = not mitigated; mitigated = has MITIGATED_BY edge.
	// Only high/critical unmitigated vulns contribute to red reasons.
	openHighCritical := []string{}
	for _, vulnID := range ev.VulnIDs {
		if _, mitigated := mitigatedSet[vulnID]; !mitigated {
			// Check severity attribute on the vuln node.
			vulnNode, err := r.gs.GetNode(ctx, tenantID, vulnID)
			if err == nil {
				severity, _ := vulnNode.Attributes["severity"].(string)
				if severity == domainsupplychain.SeverityHigh || severity == domainsupplychain.SeverityCritical {
					openHighCritical = append(openHighCritical, vulnID)
				}
			}
		} else {
			ev.MITigatedVulns = append(ev.MITigatedVulns, vulnID)
		}
	}
	ev.OpenVulns = openHighCritical

	return toSupplyChainEvidence(ev), nil
}

// toSupplyChainEvidence converts internal evidence type to the supplychain.Evidence type.
func toSupplyChainEvidence(ev supplyChainEvidence) supplychain.Evidence {
	return supplychain.Evidence{
		SBOMIDs:              ev.SBOMIDs,
		TotalAttestations:     ev.TotalAttestations,
		VerifiedAttestations: ev.VerifiedAttestations,
		ComponentPURLs:       nil, // Components are internal to the walk
		VulnerabilityIDs:     ev.VulnIDs,
		OpenVulnIDs:          ev.OpenVulns,
		MitigatedVulnIDs:     ev.MITigatedVulns,
	}
}

// CheckArtifactVerification implements ArtifactVerifier by walking the artifact's incident VERIFIES edges
// and checking whether any source TestRun passed.
func (r *graphStoreSupplyChainEvidenceReader) CheckArtifactVerification(ctx context.Context, tenant, digest string) bool {
	tenantID := ports.TenantID(tenant)
	sub, err := r.gs.Neighborhood(ctx, ports.NeighborhoodQuery{
		TenantID: tenantID, Roots: []string{digest}, MaxDepth: 1, MaxNodes: 50, MaxEdges: 100,
	})
	if err != nil {
		return false
	}
	for _, e := range sub.Edges {
		if e.Type != "VERIFIES" {
			continue
		}
		runID := e.SourceID
		if runID == digest {
			runID = e.TargetID
		}
		run, err := r.gs.GetNode(ctx, tenantID, runID)
		if err != nil {
			continue
		}
		if status, _ := run.Attributes["status"].(string); status == "passed" {
			return true
		}
	}
	return false
}
