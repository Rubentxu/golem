// Package release hosts the application handlers of the Release context.
package release

import (
	"context"
	"errors"
	"fmt"
	"strings"

	appcmd "github.com/Rubentxu/golem/internal/application/command"
	"github.com/Rubentxu/golem/internal/application/ci"
	"github.com/Rubentxu/golem/internal/application/supplychain"
	"github.com/Rubentxu/golem/internal/ports"
	domainrelease "github.com/Rubentxu/golem/internal/release"
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
func CreateCandidateHandler(gen ports.IDGenerator, artifactReader ci.ArtifactReader) appcmd.Handler {
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
			exists, err := artifactReader.DigestExists(ctx, string(cmd.TenantID), d)
			if err != nil || !exists {
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
func EvaluateGateHandler(releaseReader ReleaseGraphReader, evidenceReader supplychain.SupplyChainEvidenceReader, artifactVerifier ArtifactVerifier) appcmd.Handler {
	return func(ctx context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
		p, ok := cmd.Payload.(EvaluateGate)
		if !ok {
			return nil, errors.New("release: payload must be release.EvaluateGate")
		}
		if strings.TrimSpace(p.ReleaseID) == "" {
			return nil, ErrReleaseNotFound
		}

		// Check release exists via ReleaseGraphReader.
		exists, err := releaseReader.NodeExists(ctx, string(cmd.TenantID), p.ReleaseID)
		if err != nil || !exists {
			return nil, ErrReleaseNotFound
		}

		// Get artifact digests via ReleaseGraphReader.
		artifacts, err := releaseReader.GetReleaseArtifactDigests(ctx, string(cmd.TenantID), p.ReleaseID)
		if err != nil {
			return nil, ErrReleaseNotFound
		}

		details := make([]domainrelease.GateDetail, 0, len(artifacts))
		evidence := make(map[string]domainrelease.ArtifactEvidence)
		reasons := []string{}

		for _, digest := range artifacts {
			// Collect supply-chain evidence via typed traversal.
			ev, err := evidenceReader.CollectEvidence(ctx, string(cmd.TenantID), digest)
			if err != nil {
				return nil, err
			}

			// Convert supplychain.Evidence to internal evidence type for ToArtifactEvidence.
			internalEv := supplyChainEvidence{
				SBOMIDs:              ev.SBOMIDs,
				TotalAttestations:     ev.TotalAttestations,
				VerifiedAttestations: ev.VerifiedAttestations,
				OpenVulns:            ev.OpenVulnIDs,
				MITigatedVulns:       ev.MitigatedVulnIDs,
			}

			// Check for supply-chain data presence.
			hasSBOM := len(ev.SBOMIDs) > 0
			hasAttestations := ev.TotalAttestations > 0

			// v2 evaluation when supply-chain data exists.
			if hasSBOM || hasAttestations {
				evidence[digest] = internalEv.ToArtifactEvidence(digest)

				if !hasSBOM {
					reasons = append(reasons, "sbom_missing")
				}
				if ev.VerifiedAttestations < ev.TotalAttestations {
					reasons = append(reasons, "attestation_unverified")
				}
				for _, vuln := range ev.OpenVulnIDs {
					reasons = append(reasons, "vuln_unmitigated:"+vuln)
				}

				details = append(details, domainrelease.GateDetail{Artifact: digest, Verified: true})
			} else {
				// Fall back to v1 semantics when no supply-chain data exists.
				// Bootstrap rule: v1 behavior preserved for existing artifacts without SBOM/attestation.
				v1Verified := artifactVerifier.CheckArtifactVerification(ctx, string(cmd.TenantID), digest)
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

// EvaluateGate is the payload of CmdEvaluateGate.
type EvaluateGate struct {
	ReleaseID string `json:"release_id"`
}
