// Package ci hosts the application handlers of the CI context.
package ci

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	appcmd "github.com/Rubentxu/golem/internal/application/command"
	domainci "github.com/Rubentxu/golem/internal/ci"
	"github.com/Rubentxu/golem/internal/application/scm"
	"github.com/Rubentxu/golem/internal/ports"
)

var (
	ErrCommitNotObserved = errors.New("ci: commit not observed yet")
	ErrInvalidDigest     = errors.New("ci: artifact digest must be sha256:<64 hex>")
	ErrInvalidStatus     = errors.New("ci: status must be success|failure|unstable")
	ErrEmptyPipeline     = errors.New("ci: pipeline is mandatory")
)

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// Command names of this context.
const (
	CmdCompleteBuild = "ci.complete-build"
)

// CompleteBuild is the payload of CmdCompleteBuild.
type CompleteBuild struct {
	Pipeline  string                 `json:"pipeline"`
	Commit    string                 `json:"commit"`
	Status    string                 `json:"status"`
	Artifacts []domainci.ArtifactOut `json:"artifacts"`
}

// CompleteBuildHandler returns the handler for CmdCompleteBuild. The
// built commit must already be observed (its journal stream exists — a
// synchronous check against the authoritative history, no projection
// dependency). Artifact digests must be content-addressed (ADR-022).
func CompleteBuildHandler(gen ports.IDGenerator, scmReader scm.SCMStreamReader) appcmd.Handler {
	return func(ctx context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
		p, ok := cmd.Payload.(CompleteBuild)
		if !ok {
			return nil, errors.New("ci: payload must be ci.CompleteBuild")
		}
		if strings.TrimSpace(p.Pipeline) == "" {
			return nil, ErrEmptyPipeline
		}
		sha := strings.ToLower(strings.TrimSpace(p.Commit))
		if sha == "" {
			return nil, ErrCommitNotObserved
		}
		switch strings.ToLower(strings.TrimSpace(p.Status)) {
		case "success", "failure", "unstable":
		default:
			return nil, fmt.Errorf("%w: %q", ErrInvalidStatus, p.Status)
		}

		// Check commit exists via SCMStreamReader.
		if _, err := scmReader.CommitObserved(ctx, string(cmd.TenantID), sha); err != nil {
			return nil, err
		}

		artifacts := make([]domainci.ArtifactOut, 0, len(p.Artifacts))
		for _, a := range p.Artifacts {
			digest := strings.ToLower(strings.TrimSpace(a.Digest))
			if !digestPattern.MatchString(digest) {
				return nil, fmt.Errorf("%w: %q", ErrInvalidDigest, a.Digest)
			}
			kind := strings.TrimSpace(a.Kind)
			if kind == "" {
				kind = "Artifact"
			}
			artifacts = append(artifacts, domainci.ArtifactOut{Digest: digest, Name: strings.TrimSpace(a.Name), Kind: kind})
		}

		buildID := gen.NewID()
		return []appcmd.EventDraft{{
			EventType:     domainci.EventBuildCompleted,
			StreamID:      "build:" + buildID,
			SchemaVersion: 1,
			Payload: domainci.BuildCompleted{
				BuildID: buildID, Pipeline: strings.TrimSpace(p.Pipeline),
				Commit: sha, Status: strings.ToLower(strings.TrimSpace(p.Status)),
				Artifacts: artifacts,
			},
		}}, nil
	}
}
