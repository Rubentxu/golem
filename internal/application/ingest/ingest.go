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
	appsupplychain "github.com/Rubentxu/golem/internal/application/supplychain"
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
	s.Register(SBOMSPDX{})
	s.Register(SBOMCycloneDX{})
	s.Register(AttestationInToto{})
	s.Register(VEXOpenVEX{})
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

// ---- SBOM SPDX translator ----

// SBOMSPDX translates SPDX SBOM webhook payloads.
type SBOMSPDX struct{}

// Provider identifies the translator.
func (SBOMSPDX) Provider() string { return "sbom-spdx" }

type sbomSpdxPayload struct {
	ExternalID string `json:"external_id"`
	Document   struct {
		Name string `json:"name"`
	} `json:"document"`
	RawB64 string `json:"raw_b64"` // base64-encoded SPDX document
}

// Translate implements Translator.
func (SBOMSPDX) Translate(payload []byte) ([]IngestedCommand, error) {
	var p sbomSpdxPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("ingest sbom-spdx: bad payload: %w", err)
	}
	if p.ExternalID == "" {
		return nil, errors.New("ingest sbom-spdx: external_id is mandatory")
	}
	return []IngestedCommand{{
		Name:      appsupplychain.CmdIngestSBOM,
		CommandID: "ingest.sbom-spdx.doc." + p.ExternalID,
		Payload: appsupplychain.IngestSBOM{
			Provider:      "sbom-spdx",
			ExternalDocID: p.ExternalID,
			FormatHint:    "spdx-2.3",
			RawB64:        p.RawB64,
		},
	}}, nil
}

// ---- SBOM CycloneDX translator ----

// SBOMCycloneDX translates CycloneDX SBOM webhook payloads.
type SBOMCycloneDX struct{}

// Provider identifies the translator.
func (SBOMCycloneDX) Provider() string { return "sbom-cyclonedx" }

type sbomCDXPayload struct {
	ExternalID string `json:"external_id"`
	Document   struct {
		Metadata struct {
			Component struct {
				Name string `json:"name"`
			} `json:"component"`
		} `json:"metadata"`
	} `json:"document"`
	RawB64 string `json:"raw_b64"`
}

// Translate implements Translator.
func (SBOMCycloneDX) Translate(payload []byte) ([]IngestedCommand, error) {
	var p sbomCDXPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("ingest sbom-cyclonedx: bad payload: %w", err)
	}
	if p.ExternalID == "" {
		return nil, errors.New("ingest sbom-cyclonedx: external_id is mandatory")
	}
	return []IngestedCommand{{
		Name:      appsupplychain.CmdIngestSBOM,
		CommandID: "ingest.sbom-cyclonedx.doc." + p.ExternalID,
		Payload: appsupplychain.IngestSBOM{
			Provider:      "sbom-cyclonedx",
			ExternalDocID: p.ExternalID,
			FormatHint:    "cyclonedx-1.5",
			RawB64:        p.RawB64,
		},
	}}, nil
}

// ---- Attestation in-toto translator ----

// AttestationInToto translates in-toto/SLSA attestation webhook payloads.
type AttestationInToto struct{}

// Provider identifies the translator.
func (AttestationInToto) Provider() string { return "attestation-intoto" }

type attestationPayload struct {
	SubjectDigest string `json:"subject_digest"` // sha256:...
	PredicateType string `json:"predicate_type"` // slsa-provenance|intoto-statement|intoto-link
	StatementB64  string `json:"statement_b64"`  // base64-encoded in-toto statement JSON
	Provider      string `json:"provider"`
}

// Translate implements Translator.
func (AttestationInToto) Translate(payload []byte) ([]IngestedCommand, error) {
	var p attestationPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("ingest attestation-intoto: bad payload: %w", err)
	}
	if p.SubjectDigest == "" {
		return nil, errors.New("ingest attestation-intoto: subject_digest is mandatory")
	}
	if p.StatementB64 == "" {
		return nil, errors.New("ingest attestation-intoto: statement_b64 is mandatory")
	}
	return []IngestedCommand{{
		Name:      appsupplychain.CmdIngestAttestation,
		CommandID: "ingest.attestation-intoto." + p.SubjectDigest,
		Payload: appsupplychain.IngestAttestation{
			ArtifactDigest: p.SubjectDigest,
			PredicateType:  p.PredicateType,
			StatementJSON:  p.StatementB64,
			Provider:       p.Provider,
		},
	}}, nil
}

// ---- VEX OpenVEX translator ----

// VEXOpenVEX translates OpenVEX webhook payloads. Each statement in the
// OpenVEX document becomes one command.
type VEXOpenVEX struct{}

// Provider identifies the translator.
func (VEXOpenVEX) Provider() string { return "vex-openvex" }

type vexOpenVEXPayload struct {
	DocID      string `json:"doc_id"`
	Statements []struct {
		ID            string `json:"id"`
		VulnID        string `json:"vuln_id"`
		Status        string `json:"status"`
		Justification string `json:"justification,omitempty"`
		Product       struct {
			Identifier string `json:"identifier"` // artifact digest or purl
			Type       string `json:"type"`       // "artifact" or "purl"
		} `json:"product"`
		Provider string `json:"provider"`
	} `json:"statements"`
}

// Translate implements Translator.
func (VEXOpenVEX) Translate(payload []byte) ([]IngestedCommand, error) {
	var p vexOpenVEXPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("ingest vex-openvex: bad payload: %w", err)
	}
	if p.DocID == "" {
		return nil, errors.New("ingest vex-openvex: doc_id is mandatory")
	}
	out := make([]IngestedCommand, 0, len(p.Statements))
	for i, stmt := range p.Statements {
		if stmt.ID == "" || stmt.VulnID == "" {
			continue
		}
		cmdID := fmt.Sprintf("ingest.vex-openvex.%s.%d", p.DocID, i)
		vex := appsupplychain.RecordVEX{
			StatementID:   stmt.ID,
			VulnID:        stmt.VulnID,
			Status:        stmt.Status,
			Justification: stmt.Justification,
			Provider:      stmt.Provider,
		}
		if stmt.Product.Type == "artifact" {
			vex.ProductDigest = stmt.Product.Identifier
		} else if stmt.Product.Type == "purl" {
			vex.ProductPurl = stmt.Product.Identifier
		}
		out = append(out, IngestedCommand{
			Name:      appsupplychain.CmdRecordVEX,
			CommandID: cmdID,
			Payload:   vex,
		})
	}
	if len(out) == 0 {
		return nil, errors.New("ingest vex-openvex: no valid statements found")
	}
	return out, nil
}
