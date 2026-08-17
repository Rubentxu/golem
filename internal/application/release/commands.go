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
		if len(artifacts) == 0 {
			return nil, ErrNoArtifacts
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

// EvaluateGateHandler computes the evidence gate from the projection:
// green iff every composed artifact has at least one TestRun with a
// passed status connected through VERIFIES. The evaluation is journaled
// as evidence (evidence first, PRINCIPLES).
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
		green := true
		for _, ra := range rawArtifacts {
			digest, _ := ra.(string)
			verified := artifactVerified(ctx, graph, cmd.TenantID, digest)
			details = append(details, domainrelease.GateDetail{Artifact: digest, Verified: verified})
			if !verified {
				green = false
			}
		}
		result := "red"
		if green {
			result = "green"
		}

		return []appcmd.EventDraft{{
			EventType:     domainrelease.EventGateEvaluated,
			StreamID:      "release:" + p.ReleaseID,
			SchemaVersion: 1,
			Payload:       domainrelease.GateEvaluated{ReleaseID: p.ReleaseID, Result: result, Details: details},
		}}, nil
	}
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
