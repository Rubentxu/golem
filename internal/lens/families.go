package lens

import (
	"github.com/Rubentxu/golem/internal/canonical"
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

// AgentChangeLens builds the spec that answers "what behavior changes did this
// agent propose or apply?" — AgentEval → EVALUATED → Behavior and
// AgentEval → OBSERVED → Proposal (REQ-023, ADR-067).
//
// Roots are AgentEval node IDs. The lens walks both EVALUATED edges
// (to the Behavior being evaluated) and OBSERVED edges (to the Proposal
// that triggered the eval, if any).
func AgentChangeLens(roots []string, depth, maxNodes, maxEdges int) Spec {
	return Spec{
		Roots:     roots,
		NodeTypes: []string{canonical.AgentEvalNodeKind, "Behavior", "Proposal"},
		EdgeTypes: []string{canonical.EdgeTypeEVALUATED, canonical.EdgeTypeOBSERVED},
		MaxDepth:  depth,
		MaxNodes:  maxNodes,
		MaxEdges:  maxEdges,
		Evidence:  true,
	}
}

// RequirementTraceLens builds the spec that answers "which test cases and
// evidence back this requirement?" — Requirement → VERIFIES → TestCase,
// with attached Evidence nodes (REQ-018).
func RequirementTraceLens(roots []string, depth, maxNodes, maxEdges int) Spec {
	return Spec{
		Roots:     roots,
		NodeTypes: []string{"Requirement", "TestCase", "Evidence", "UATSession"},
		EdgeTypes: []string{"VERIFIES", "EVIDENCED_BY"},
		MaxDepth:  depth,
		MaxNodes:  maxNodes,
		MaxEdges:  maxEdges,
		Evidence:  true,
	}
}

// ArchitectureImpactLens builds the spec that answers "what is the blast radius
// of changing these architecture decisions?" — ADR → AFFECTED_BY → Component,
// with transitive containment (REQ-018).
func ArchitectureImpactLens(roots []string, depth, maxNodes, maxEdges int) Spec {
	return Spec{
		Roots:     roots,
		NodeTypes: []string{"ADR", "Component", "ServiceInstance", "Deployment", "Environment"},
		EdgeTypes: []string{"AFFECTED_BY", "CONTAINS", "DEPLOYED_TO", "BUILT_BY"},
		MaxDepth:  depth,
		MaxNodes:  maxNodes,
		MaxEdges:  maxEdges,
		Evidence:  true,
	}
}

// UATContextLens builds the spec that answers "what is the acceptance context
// for this requirement or feature?" — Requirement → VERIFIES → TestCase,
// UATSession, Evidence with a wider scope than RequirementTraceLens (REQ-018).
func UATContextLens(roots []string, depth, maxNodes, maxEdges int) Spec {
	return Spec{
		Roots:     roots,
		NodeTypes: []string{"Requirement", "Milestone", "Iteration", "TestCase", "TestRun", "UATSession", "Evidence"},
		EdgeTypes: []string{"VERIFIES", "CONTAINS", "EVIDENCED_BY", "PRODUCED"},
		MaxDepth:  depth,
		MaxNodes:  maxNodes,
		MaxEdges:  maxEdges,
		Evidence:  true,
	}
}
