// Package release hosts the application handlers of the Release context.
package release

import (
	"context"
	"errors"
	"fmt"
	"strings"

	appcmd "github.com/Rubentxu/golem/internal/application/command"
	"github.com/Rubentxu/golem/internal/ports"
	domainrelease "github.com/Rubentxu/golem/internal/release"
	domainsupplychain "github.com/Rubentxu/golem/internal/supplychain"
)

var (
	ErrEmptyName       = errors.New("release: name is mandatory")
	ErrNoArtifacts     = errors.New("release: at least one artifact is mandatory")
	ErrUnknownArtifact = errors.New("release: artifact not found")
	ErrReleaseNotFound = errors.New("release: release not found")
)

// Command names of this context.
const (
	CmdCreateCandidate = "release.create-candidate"
	CmdEvaluateGate    = "release.evaluate-gate"
)

// CreateCandidate is the payload of CmdCreateCandidate.
type CreateCandidate struct {
	Name      string   `json:"name"`
	Artifacts []string `json:"artifacts"`
}

// CreateCandidateHandler validates that every artifact digest exists in
// the tenant graph (they materialize from ci.build.completed events)
// and journals the release candidate.
func CreateCandidateHandler(gen ports.IDGenerator, graph ports.GraphStore) appcmd.Handler {
	return func(ctx context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
		p, ok := cmd.Payload.(CreateCandidate)
		if !ok {
			return nil, errors.New("release: payload must be release.CreateCandidate")
		}
		name := strings.TrimSpace(p.Name)
		if name == "" {
			return nil, ErrEmptyName
		}
		artifacts := make([]string, 0, len(p.Artifacts))
		seen := map[string]bool{}
		for _, a := range p.Artifacts {
			d := strings.ToLower(strings.TrimSpace(a))
			if d == "" || seen[d] {
				continue
			}
			seen[d] = true
			if _, err := graph.GetNode(ctx, cmd.TenantID, d); err != nil {
				return nil, fmt.Errorf("%w: %s", ErrUnknownArtifact, d)
			}
			artifacts = append(artifacts, d)
		}
		id := gen.NewID()
		return []appcmd.EventDraft{{
			EventType:     domainrelease.EventCandidateCreated,
			StreamID:      "release:" + id,
			SchemaVersion: 1,
			Payload:       domainrelease.CandidateCreated{ReleaseID: id, Name: name, Artifacts: artifacts},
		}}, nil
	}
}

// EvaluateGateHandler computes the evidence gate. It uses the supply-chain-gate-v1
// policy when an artifact carries supply-chain data (HAS_SBOM or ATTESTED_BY edges);
// otherwise it falls back to the original v1 VERIFIES walk for that artifact.
// Green iff no reasons. The evaluation is journaled as evidence (evidence first).
func EvaluateGateHandler(graph ports.GraphStore) appcmd.Handler {
	return func(ctx context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
		p, ok := cmd.Payload.(EvaluateGate)
		if !ok {
			return nil, errors.New("release: payload must be release.EvaluateGate")
		}
		if strings.TrimSpace(p.ReleaseID) == "" {
			return nil, ErrReleaseNotFound
		}

		node, err := graph.GetNode(ctx, cmd.TenantID, p.ReleaseID)
		if err != nil {
			return nil, ErrReleaseNotFound
		}
		rawArtifacts, _ := node.Attributes["artifacts"].([]any)

		details := make([]domainrelease.GateDetail, 0, len(rawArtifacts))
		evidence := make(map[string]domainrelease.ArtifactEvidence)
		reasons := []string{}

		for _, ra := range rawArtifacts {
			digest, _ := ra.(string)

			// Collect supply-chain evidence via typed traversal.
			ev := collectSupplyChainEvidence(ctx, graph, cmd.TenantID, digest)

			// Check for supply-chain data presence.
			hasSBOM := len(ev.SBOMIDs) > 0
			hasAttestations := ev.TotalAttestations > 0

			// v2 evaluation when supply-chain data exists.
			if hasSBOM || hasAttestations {
				evidence[digest] = ev.ToArtifactEvidence(digest)

				if !hasSBOM {
					reasons = append(reasons, "sbom_missing")
				}
				if ev.VerifiedAttestations < ev.TotalAttestations {
					reasons = append(reasons, "attestation_unverified")
				}
				for _, vuln := range ev.OpenVulns {
					reasons = append(reasons, "vuln_unmitigated:"+vuln)
				}

				details = append(details, domainrelease.GateDetail{Artifact: digest, Verified: true})
			} else {
				// Fall back to v1 semantics when no supply-chain data exists.
				// Bootstrap rule: v1 behavior preserved for existing artifacts without SBOM/attestation.
				v1Verified := artifactVerified(ctx, graph, cmd.TenantID, digest)
				details = append(details, domainrelease.GateDetail{Artifact: digest, Verified: v1Verified})
				if !v1Verified {
					reasons = append(reasons, "vuln_unmitigated:v1_fallback") // sentinel for v1 red
				}
			}
		}

		// v2 gate: green iff no reasons; v1 artifacts contribute reasons only when red.
		result := "red"
		if len(reasons) == 0 {
			result = "green"
		}

		// Only set v2 fields when at least one artifact has supply-chain data.
		var policyVersion string
		var evMap map[string]domainrelease.ArtifactEvidence
		if len(evidence) > 0 {
			policyVersion = domainrelease.PolicyVersionSupplyChainGateV1
			evMap = evidence
		}

		return []appcmd.EventDraft{{
			EventType:     domainrelease.EventGateEvaluated,
			StreamID:      "release:" + p.ReleaseID,
			SchemaVersion: 1,
			Payload: domainrelease.GateEvaluated{
				ReleaseID:     p.ReleaseID,
				Result:        result,
				Details:       details,
				PolicyVersion: policyVersion,
				Evidence:      evMap,
				Reasons:       reasons,
			},
		}}, nil
	}
}

// supplyChainEvidence holds intermediate evidence collected during gate evaluation.
type supplyChainEvidence struct {
	SBOMIDs              []string // IDs of SBOMs attached to this artifact
	TotalAttestations    int
	VerifiedAttestations int
	SBOMComponents       []string // PackageComponent purls reachable via HAS_SBOM→CONTAINS
	VulnIDs              []string // Vulnerability IDs reachable via components
	OpenVulns            []string // Unmitigated high/critical vulnerability IDs
	MITigatedVulns       []string
}

// ToArtifactEvidence converts intermediate evidence to the domain artifact evidence struct.
func (e supplyChainEvidence) ToArtifactEvidence(digest string) domainrelease.ArtifactEvidence {
	ev := domainrelease.ArtifactEvidence{ArtifactDigest: digest}
	ev.SBOMPresent = len(e.SBOMIDs) > 0
	ev.Attestations.Verified = e.VerifiedAttestations
	ev.Attestations.Total = e.TotalAttestations
	ev.Vulnerabilities.Open = len(e.OpenVulns)
	ev.Vulnerabilities.Mitigated = len(e.MITigatedVulns)
	return ev
}

// collectSupplyChainEvidence walks supply-chain edges from an artifact and returns
// typed evidence under the supply-chain-gate-v1 policy.
//
// Walk path: artifact → HAS_SBOM → SBOM → CONTAINS → PackageComponent → AFFECTED_BY → Vulnerability
// Separate walk: artifact → ATTESTED_BY → Attestation
// Mitigation check: vulnerability → MITIGATED_BY → VEXStatement
func collectSupplyChainEvidence(ctx context.Context, graph ports.GraphStore, tenant ports.TenantID, artifactID string) supplyChainEvidence {
	ev := supplyChainEvidence{}

	// Walk 1: Find SBOMs attached to this artifact.
	sbomWalk, _ := graph.Traversal(ctx, ports.TraversalQuery{
		TenantID:  tenant,
		Roots:     []string{artifactID},
		EdgeTypes: []string{domainsupplychain.RelationHAS_SBOM},
		Kinds:     []string{domainsupplychain.KindSBOM},
		MaxDepth:  1,
		MaxNodes:  100,
		MaxEdges:  200,
	})
	for _, n := range sbomWalk.Nodes {
		if n.ID != artifactID {
			ev.SBOMIDs = append(ev.SBOMIDs, n.ID)
		}
	}

	// Walk 2: Walk artifact → ATTESTED_BY → Attestation.
	attWalk, _ := graph.Traversal(ctx, ports.TraversalQuery{
		TenantID:  tenant,
		Roots:     []string{artifactID},
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
		if n.ID == artifactID {
			continue
		}
		if ver, _ := n.Attributes["verification"].(string); ver == domainsupplychain.VerificationVerified {
			ev.VerifiedAttestations++
		}
	}

	if len(ev.SBOMIDs) == 0 {
		return ev // no SBOM means no vulnerability walk needed
	}

	// Walk 3: For each SBOM, walk SBOM → CONTAINS → PackageComponent.
	componentIDs := []string{}
	for _, sbomID := range ev.SBOMIDs {
		compWalk, _ := graph.Traversal(ctx, ports.TraversalQuery{
			TenantID:  tenant,
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
		vulnWalk, _ := graph.Traversal(ctx, ports.TraversalQuery{
			TenantID:  tenant,
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
		mitWalk, _ := graph.Traversal(ctx, ports.TraversalQuery{
			TenantID:  tenant,
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
			vulnNode, err := graph.GetNode(ctx, tenant, vulnID)
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

	return ev
}

// artifactVerified walks the artifact's incident VERIFIES edges and
// checks whether any source TestRun passed.
func artifactVerified(ctx context.Context, graph ports.GraphStore, tenant ports.TenantID, digest string) bool {
	sub, err := graph.Neighborhood(ctx, ports.NeighborhoodQuery{
		TenantID: tenant, Roots: []string{digest}, MaxDepth: 1, MaxNodes: 50, MaxEdges: 100,
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
		run, err := graph.GetNode(ctx, tenant, runID)
		if err != nil {
			continue
		}
		if status, _ := run.Attributes["status"].(string); status == "passed" {
			return true
		}
	}
	return false
}

// EvaluateGate is the payload of CmdEvaluateGate.
type EvaluateGate struct {
	ReleaseID string `json:"release_id"`
}
