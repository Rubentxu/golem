package lens

import (
	"github.com/Rubentxu/golem/internal/supplychain"
)

// VulnerabilityImpactLens builds the spec that answers "which releases and
// artifacts are affected by the given components/vulnerabilities?" — the
// lens version of the M4 blast-radius walk:
//
//	PackageComponent → CONTAINS⁻¹ → SBOM → HAS_SBOM⁻¹ → Artifact → RELEASED_AS → Release
//
// plus the vulnerability side (AFFECTED_BY, MITIGATED_BY). Traversal is
// undirected with explicit edge-type filters, so one walk covers all
// directions.
func VulnerabilityImpactLens(roots []string, depth, maxNodes, maxEdges int) Spec {
	return Spec{
		Roots:     roots,
		NodeTypes: append([]string{kindRelease}, supplyChainKinds()...),
		EdgeTypes: supplyChainEdges(),
		MaxDepth:  depth,
		MaxNodes:  maxNodes,
		MaxEdges:  maxEdges,
		Evidence:  true,
	}
}

// ReleaseEvidenceLens builds the spec that answers "what is the full
// evidence chain behind these releases?" — Release → RELEASED_AS →
// Artifact → SBOM → PackageComponent, with vulnerability/VEX statements
// attached (AFFECTED_BY / MITIGATED_BY).
func ReleaseEvidenceLens(roots []string, depth, maxNodes, maxEdges int) Spec {
	return Spec{
		Roots:     roots,
		NodeTypes: append([]string{kindRelease}, supplyChainKinds()...),
		EdgeTypes: supplyChainEdges(),
		MaxDepth:  depth,
		MaxNodes:  maxNodes,
		MaxEdges:  maxEdges,
		Evidence:  true,
	}
}

const kindRelease = "Release"

func supplyChainKinds() []string {
	return []string{
		supplychain.KindSBOM,
		supplychain.KindPackageComponent,
		supplychain.KindVulnerability,
		supplychain.KindVEXStatement,
		supplychain.KindAttestation,
		"Artifact",
	}
}

func supplyChainEdges() []string {
	return []string{
		supplychain.RelationCONTAINS,
		supplychain.RelationHAS_SBOM,
		"RELEASED_AS",
		supplychain.RelationAFFECTED_BY,
		supplychain.RelationMITIGATED_BY,
	}
}
