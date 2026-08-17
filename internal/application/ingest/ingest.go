// Package ingest implements the provider event sinks (M3 "event
// sinks"): webhook-style ingestion of external provider events
// translated into GOLEM commands.
//
// External idempotency (EVENT_MODEL: "Provider + external_event_id /
// content fingerprint + dedup key") is achieved by deriving CommandIDs
// deterministically from the provider identity: ingest.<provider>.
// <kind>.<external-id>. A redelivered webhook maps to the same
// CommandID, and the command registry dedups it — no extra dedup
// infrastructure, at-least-once delivery tolerated end to end.
package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	appci "github.com/Rubentxu/golem/internal/application/ci"
	appcmd "github.com/Rubentxu/golem/internal/application/command"
	appscm "github.com/Rubentxu/golem/internal/application/scm"
	domainci "github.com/Rubentxu/golem/internal/ci"
	"github.com/Rubentxu/golem/internal/ports"
)

// IngestedCommand is one translated provider event.
type IngestedCommand struct {
	Name      string
	CommandID string // deterministic, provider-derived
	Payload   any
}

// Translator maps one provider's webhook payload to commands.
type Translator interface {
	Provider() string
	Translate(payload []byte) ([]IngestedCommand, error)
}

// Submitter is the command sink (the bus).
type Submitter interface {
	Submit(ctx context.Context, cmd appcmd.Command) (ports.CommandReceipt, error)
}

// Report summarizes an ingestion call.
type Report struct {
	Accepted   int
	Duplicates int
	CommandIDs []string
	Errors     []error
}

// Service routes payloads to the translator of their provider.
type Service struct {
	translators map[string]Translator
	bus         Submitter
}

// New builds the service with the reference translators.
func New(bus Submitter) *Service {
	s := &Service{translators: map[string]Translator{}, bus: bus}
	s.Register(GitHubPush{})
	s.Register(GenericCI{})
	return s
}

// Register adds or replaces a translator.
func (s *Service) Register(t Translator) { s.translators[t.Provider()] = t }

// Ingest processes one provider payload under the tenant.
func (s *Service) Ingest(ctx context.Context, tenant ports.TenantID, provider string, payload []byte) (*Report, error) {
	t, ok := s.translators[strings.ToLower(strings.TrimSpace(provider))]
	if !ok {
		return nil, fmt.Errorf("ingest: unknown provider %q", provider)
	}
	cmds, err := t.Translate(payload)
	if err != nil {
		return nil, err
	}

	rep := &Report{}
	actor := ports.Actor{Type: "service", ID: "ingest:" + t.Provider()}
	for _, c := range cmds {
		receipt, err := s.bus.Submit(ctx, appcmd.Command{
			Name: c.Name, TenantID: tenant, Actor: actor,
			CommandID: c.CommandID, CorrelationID: "ingest:" + t.Provider(),
			Payload: c.Payload,
		})
		switch {
		case err != nil:
			rep.Errors = append(rep.Errors, fmt.Errorf("%s %s: %w", provider, c.CommandID, err))
		case receipt.Duplicate:
			rep.Duplicates++
			rep.CommandIDs = append(rep.CommandIDs, c.CommandID)
		default:
			rep.Accepted++
			rep.CommandIDs = append(rep.CommandIDs, c.CommandID)
		}
	}
	return rep, nil
}

// ---- GitHub push webhook (commit observation) ----

// GitHubPush translates push events: every commit of the push becomes
// one scm.observe-commit command, idempotent by sha.
type GitHubPush struct{}

// Provider identifies the translator.
func (GitHubPush) Provider() string { return "github" }

type ghPushPayload struct {
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Commits []struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	} `json:"commits"`
}

// Translate implements Translator.
func (GitHubPush) Translate(payload []byte) ([]IngestedCommand, error) {
	var p ghPushPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("ingest github: bad payload: %w", err)
	}
	if p.Repository.FullName == "" {
		return nil, errors.New("ingest github: repository.full_name is mandatory")
	}
	out := make([]IngestedCommand, 0, len(p.Commits))
	for _, c := range p.Commits {
		sha := strings.ToLower(strings.TrimSpace(c.ID))
		if sha == "" {
			continue
		}
		out = append(out, IngestedCommand{
			Name:      appscm.CmdObserveCommit,
			CommandID: "ingest.github.commit." + sha,
			Payload:   appscm.ObserveCommit{SHA: sha, Repository: p.Repository.FullName, Message: c.Message},
		})
	}
	if len(out) == 0 {
		return nil, errors.New("ingest github: payload carries no commits")
	}
	return out, nil
}

// ---- Generic CI webhook (build completion) ----

// GenericCI translates a provider-neutral CI completion event,
// idempotent by external build id.
type GenericCI struct{}

// Provider identifies the translator.
func (GenericCI) Provider() string { return "ci-generic" }

type genericCIPayload struct {
	ExternalBuildID string `json:"external_build_id"`
	Pipeline        string `json:"pipeline"`
	Commit          string `json:"commit"`
	Status          string `json:"status"`
	Artifacts       []struct {
		Digest string `json:"digest"`
		Name   string `json:"name"`
		Kind   string `json:"kind"`
	} `json:"artifacts"`
}

// Translate implements Translator.
func (GenericCI) Translate(payload []byte) ([]IngestedCommand, error) {
	var p genericCIPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("ingest ci-generic: bad payload: %w", err)
	}
	ext := strings.TrimSpace(p.ExternalBuildID)
	if ext == "" {
		return nil, errors.New("ingest ci-generic: external_build_id is mandatory")
	}
	artifacts := make([]domainci.ArtifactOut, 0, len(p.Artifacts))
	for _, a := range p.Artifacts {
		artifacts = append(artifacts, domainci.ArtifactOut{
			Digest: strings.ToLower(strings.TrimSpace(a.Digest)),
			Name:   strings.TrimSpace(a.Name), Kind: strings.TrimSpace(a.Kind),
		})
	}
	return []IngestedCommand{{
		Name:      appci.CmdCompleteBuild,
		CommandID: "ingest.ci-generic.build." + ext,
		Payload: appci.CompleteBuild{
			Pipeline: p.Pipeline, Commit: strings.ToLower(strings.TrimSpace(p.Commit)),
			Status: p.Status, Artifacts: artifacts,
		},
	}}, nil
}
