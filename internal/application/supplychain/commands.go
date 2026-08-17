// Package supplychain hosts the application handlers of the Supply Chain
// bounded context: commands validated by domain rules and expressed as event
// drafts for the command bus.
package supplychain

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	appcmd "github.com/Rubentxu/golem/internal/application/command"
	"github.com/Rubentxu/golem/internal/ports"
	domainsupplychain "github.com/Rubentxu/golem/internal/supplychain"
)

// Command names of this context.
const (
	CmdIngestSBOM          = "supplychain.ingest-sbom"
	CmdReportVulnerability = "supplychain.report-vulnerability"
	CmdRecordVEX           = "supplychain.record-vex"
	CmdIngestAttestation   = "supplychain.ingest-attestation"
)

// Domain validation errors.
var (
	ErrInvalidPurl        = errors.New("supplychain: invalid purl")
	ErrInvalidVulnID      = errors.New("supplychain: invalid vulnerability id (must be CVE-YYYY-NNNNN or GHSA-xxxx-xxxx-xxxx)")
	ErrInvalidVEXStatus   = errors.New("supplychain: invalid vex status (must be not_affected|affected|fixed|in_remediation)")
	ErrInvalidProduct     = errors.New("supplychain: product not found in graph (must reference existing artifact digest or purl)")
	ErrInvalidAttestation = errors.New("supplychain: invalid attestation (subject digest must be provided)")
	ErrInvalidFormat      = errors.New("supplychain: invalid format (must be spdx-2.3|spdx-3.0|cyclonedx-1.5|cyclonedx-1.6)")
	ErrInvalidPredicate   = errors.New("supplychain: invalid predicate type (must be slsa-provenance|intoto-statement|intoto-link)")
)

// ---- Command payloads ----

// IngestSBOM is the payload of CmdIngestSBOM. The translator provides
// provider, external_doc_id and the raw document as base64.
// The handler decodes, parses via the SBOMParser port, validates the
// artifact digest, and emits supplychain.sbom.ingested.v1.
type IngestSBOM struct {
	Provider      string `json:"provider"`
	ExternalDocID string `json:"external_doc_id"`
	FormatHint    string `json:"format_hint"` // spdx-2.3 | spdx-3.0 | cyclonedx-1.5 | cyclonedx-1.6
	RawB64        string `json:"raw_b64"`     // base64-encoded SBOM document
}

// ReportVulnerability is the payload of CmdReportVulnerability.
type ReportVulnerability struct {
	VulnID        string `json:"vuln_id"`
	Severity      string `json:"severity"` // low|medium|high|critical
	Status        string `json:"status"`   // open|fixed|disputed
	ComponentPurl string `json:"component_purl"`
	Provider      string `json:"provider"`
}

// RecordVEX is the payload of CmdRecordVEX.
type RecordVEX struct {
	StatementID   string `json:"statement_id"`
	VulnID        string `json:"vuln_id"`
	ProductDigest string `json:"product_digest,omitempty"`
	ProductPurl   string `json:"product_purl,omitempty"`
	Status        string `json:"status"` // not_affected|affected|fixed|in_remediation
	Justification string `json:"justification"`
	Provider      string `json:"provider"`
}

// IngestAttestation is the payload of CmdIngestAttestation.
type IngestAttestation struct {
	ArtifactDigest string `json:"artifact_digest"`
	PredicateType  string `json:"predicate_type"` // slsa-provenance|intoto-statement|intoto-link
	StatementJSON  string `json:"statement_json"` // base64-encoded in-toto statement
	Provider       string `json:"provider"`
}

// ---- Validators ----

var (
	cvePattern     = regexp.MustCompile(`^CVE-\d{4}-\d{4,}$`)
	ghsaPattern    = regexp.MustCompile(`^GHSA-[a-z0-9]{4}-[a-z0-9]{4}-[a-z0-9]{4}$`)
	validFormats   = map[string]bool{"spdx-2.3": true, "spdx-3.0": true, "cyclonedx-1.5": true, "cyclonedx-1.6": true}
	validVEXStatus = map[string]bool{
		domainsupplychain.VEXStatusNotAffected:   true,
		domainsupplychain.VEXStatusAffected:      true,
		domainsupplychain.VEXStatusFixed:         true,
		domainsupplychain.VEXStatusInRemediation: true,
	}
	validPredicate = map[string]bool{
		"slsa-provenance":  true,
		"intoto-statement": true,
		"intoto-link":      true,
	}
)

func isValidVulnID(id string) bool {
	return cvePattern.MatchString(id) || ghsaPattern.MatchString(id)
}

func isValidArtifactDigest(d string) bool {
	d = strings.TrimSpace(d)
	return strings.HasPrefix(d, "sha256:") && len(d) > 7
}

func isValidSeverity(s string) bool {
	switch s {
	case domainsupplychain.SeverityLow, domainsupplychain.SeverityMedium,
		domainsupplychain.SeverityHigh, domainsupplychain.SeverityCritical:
		return true
	}
	return false
}

// ---- Handlers ----

// IngestSBOMHandler returns the handler for CmdIngestSBOM.
// It decodes the base64 document, parses it via the SBOMParser port,
// validates the artifact digest, and emits SBOMIngested.
// The SBOMID is deterministic: sbm- + first 12 hex chars of sha256 of the raw doc.
func IngestSBOMHandler(parser ports.SBOMParser) appcmd.Handler {
	return func(ctx context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
		p, ok := cmd.Payload.(IngestSBOM)
		if !ok {
			return nil, errors.New("supplychain: payload must be supplychain.IngestSBOM")
		}
		if !validFormats[p.FormatHint] {
			return nil, fmt.Errorf("%w: %q", ErrInvalidFormat, p.FormatHint)
		}
		raw, err := base64.StdEncoding.DecodeString(p.RawB64)
		if err != nil {
			return nil, fmt.Errorf("supplychain: base64 decode: %w", err)
		}

		// Parse via the SBOMParser port.
		parsed, err := parser.Parse(ctx, raw)
		if err != nil {
			return nil, fmt.Errorf("supplychain: SBOM parse: %w", err)
		}

		// Validate artifact digest format.
		if !isValidArtifactDigest(parsed.ArtifactDigest) {
			return nil, fmt.Errorf("supplychain: parsed artifact digest %q has invalid format", parsed.ArtifactDigest)
		}

		// Compute deterministic SBOMID from raw bytes.
		h := sha256.Sum256(raw)
		docDigest := hex.EncodeToString(h[:])
		sbomID := "sbm-" + docDigest[:12]

		// Map parsed components to domain components.
		components := make([]domainsupplychain.SBOMComponent, 0, len(parsed.Components))
		for _, c := range parsed.Components {
			components = append(components, domainsupplychain.SBOMComponent{
				Purl:      c.Purl,
				Name:      c.Name,
				Version:   c.Version,
				Synthetic: c.Synthetic,
			})
		}

		streamID := "artifact:" + parsed.ArtifactDigest
		return []appcmd.EventDraft{{
			EventType:     domainsupplychain.EventSBOMIngested,
			StreamID:      streamID,
			SchemaVersion: 1,
			Payload: domainsupplychain.SBOMIngested{
				SBOMID:         sbomID,
				Format:         p.FormatHint,
				SpecVersion:    parsed.SpecVersion,
				ArtifactDigest: parsed.ArtifactDigest,
				Components:     components,
				SourceProvider: p.Provider,
				SourceDocID:    p.ExternalDocID,
			},
		}}, nil
	}
}

// ReportVulnerabilityHandler returns the handler for CmdReportVulnerability.
// It validates the vulnerability ID (CVE/GHSA format) and the component purl,
// then emits VulnerabilityReported.
func ReportVulnerabilityHandler() appcmd.Handler {
	return func(ctx context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
		p, ok := cmd.Payload.(ReportVulnerability)
		if !ok {
			return nil, errors.New("supplychain: payload must be supplychain.ReportVulnerability")
		}
		p.VulnID = strings.TrimSpace(p.VulnID)
		if p.VulnID == "" || !isValidVulnID(p.VulnID) {
			return nil, fmt.Errorf("%w: %q", ErrInvalidVulnID, p.VulnID)
		}
		p.ComponentPurl = strings.TrimSpace(p.ComponentPurl)
		if p.ComponentPurl != "" {
			normalized, err := domainsupplychain.Normalize(p.ComponentPurl)
			if err != nil {
				return nil, fmt.Errorf("%w: %q", ErrInvalidPurl, p.ComponentPurl)
			}
			p.ComponentPurl = normalized
		}
		if !isValidSeverity(p.Severity) {
			return nil, fmt.Errorf("supplychain: invalid severity %q (must be low|medium|high|critical)", p.Severity)
		}
		switch p.Status {
		case domainsupplychain.StatusOpen, domainsupplychain.StatusFixed, domainsupplychain.StatusDisputed:
		default:
			return nil, fmt.Errorf("supplychain: invalid status %q (must be open|fixed|disputed)", p.Status)
		}

		streamID := "vuln:" + p.VulnID
		return []appcmd.EventDraft{{
			EventType:     domainsupplychain.EventVulnerabilityReported,
			StreamID:      streamID,
			SchemaVersion: 1,
			Payload: domainsupplychain.VulnerabilityReported{
				VulnID:        p.VulnID,
				Severity:      p.Severity,
				Status:        p.Status,
				ComponentPurl: p.ComponentPurl,
				Provider:      p.Provider,
			},
		}}, nil
	}
}

// RecordVEXHandler returns the handler for CmdRecordVEX.
// It validates the VEX status and that the referenced product exists in the graph.
func RecordVEXHandler(graph ports.GraphStore) appcmd.Handler {
	return func(ctx context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
		p, ok := cmd.Payload.(RecordVEX)
		if !ok {
			return nil, errors.New("supplychain: payload must be supplychain.RecordVEX")
		}
		p.StatementID = strings.TrimSpace(p.StatementID)
		if p.StatementID == "" {
			return nil, errors.New("supplychain: statement_id is mandatory")
		}
		p.VulnID = strings.TrimSpace(p.VulnID)
		if p.VulnID == "" || !isValidVulnID(p.VulnID) {
			return nil, fmt.Errorf("%w: %q", ErrInvalidVulnID, p.VulnID)
		}
		if !validVEXStatus[p.Status] {
			return nil, fmt.Errorf("%w: %q", ErrInvalidVEXStatus, p.Status)
		}

		// Validate product reference exists in graph.
		if p.ProductDigest == "" && p.ProductPurl == "" {
			return nil, ErrInvalidProduct
		}
		if p.ProductDigest != "" {
			// Validate by artifact digest.
			_, err := graph.GetNode(ctx, cmd.TenantID, p.ProductDigest)
			if err != nil {
				if errors.Is(err, ports.ErrNodeNotFound) {
					return nil, fmt.Errorf("%w: artifact %q not found", ErrInvalidProduct, p.ProductDigest)
				}
				return nil, err
			}
		} else if p.ProductPurl != "" {
			// Validate by component purl.
			normalized, err := domainsupplychain.Normalize(p.ProductPurl)
			if err != nil {
				return nil, fmt.Errorf("%w: %q", ErrInvalidPurl, p.ProductPurl)
			}
			_, err = graph.GetNode(ctx, cmd.TenantID, normalized)
			if err != nil {
				if errors.Is(err, ports.ErrNodeNotFound) {
					return nil, fmt.Errorf("%w: component %q not found", ErrInvalidProduct, normalized)
				}
				return nil, err
			}
		}

		streamID := "vex:" + p.StatementID
		return []appcmd.EventDraft{{
			EventType:     domainsupplychain.EventVEXStatementRecorded,
			StreamID:      streamID,
			SchemaVersion: 1,
			Payload: domainsupplychain.VEXStatementRecorded{
				StatementID:   p.StatementID,
				VulnID:        p.VulnID,
				ProductDigest: p.ProductDigest,
				ProductPurl:   p.ProductPurl,
				Status:        p.Status,
				Justification: p.Justification,
				Provider:      p.Provider,
			},
		}}, nil
	}
}

// IngestAttestationHandler returns the handler for CmdIngestAttestation.
// It decodes the statement JSON, verifies the subject digest and builder identity
// via the ProvenanceVerifier port, and emits AttestationIngested with
// verification results stored in the event.
func IngestAttestationHandler(provVerifier ports.ProvenanceVerifier) appcmd.Handler {
	return func(ctx context.Context, cmd appcmd.Command) ([]appcmd.EventDraft, error) {
		p, ok := cmd.Payload.(IngestAttestation)
		if !ok {
			return nil, errors.New("supplychain: payload must be supplychain.IngestAttestation")
		}
		p.ArtifactDigest = strings.TrimSpace(p.ArtifactDigest)
		if p.ArtifactDigest == "" || !isValidArtifactDigest(p.ArtifactDigest) {
			return nil, fmt.Errorf("%w: %q", ErrInvalidAttestation, p.ArtifactDigest)
		}
		if !validPredicate[p.PredicateType] {
			return nil, fmt.Errorf("%w: %q", ErrInvalidPredicate, p.PredicateType)
		}

		statementJSON, err := base64.StdEncoding.DecodeString(p.StatementJSON)
		if err != nil {
			return nil, fmt.Errorf("supplychain: base64 decode statement: %w", err)
		}

		// Run provenance verification.
		subjResult, err := provVerifier.VerifySubject(ctx, statementJSON, p.ArtifactDigest)
		if err != nil {
			return nil, fmt.Errorf("supplychain: VerifySubject: %w", err)
		}
		builderResult, err := provVerifier.VerifyBuilder(ctx, statementJSON)
		if err != nil {
			return nil, fmt.Errorf("supplychain: VerifyBuilder: %w", err)
		}

		// Determine verification outcome.
		verResult := domainsupplychain.VerificationFailed
		verReason := "subject_mismatch"
		if subjResult.OK && builderResult.OK {
			verResult = domainsupplychain.VerificationVerified
			verReason = "ok"
		} else if !subjResult.OK {
			verReason = "subject_mismatch"
		} else if !builderResult.OK {
			verReason = "builder_missing_or_invalid"
		}

		// Extract builder ID if present.
		builderID := ""
		var stmt struct {
			Predicate struct {
				Builder struct {
					ID string `json:"id"`
				} `json:"builder"`
			} `json:"predicate"`
		}
		if json.Unmarshal(statementJSON, &stmt) == nil {
			builderID = stmt.Predicate.Builder.ID
		}

		// Compute attestation ID from statement digest.
		h := sha256.Sum256(statementJSON)
		attID := "att-" + hex.EncodeToString(h[:])[:16]

		rawMsg := json.RawMessage(statementJSON)
		streamID := "artifact:" + p.ArtifactDigest
		return []appcmd.EventDraft{{
			EventType:     domainsupplychain.EventAttestationIngested,
			StreamID:      streamID,
			SchemaVersion: 1,
			Payload: domainsupplychain.AttestationIngested{
				AttestationID:      attID,
				ArtifactDigest:     p.ArtifactDigest,
				PredicateType:      p.PredicateType,
				BuilderID:          builderID,
				VerificationResult: verResult,
				VerificationReason: verReason,
				Statement:          rawMsg,
			},
		}}, nil
	}
}
