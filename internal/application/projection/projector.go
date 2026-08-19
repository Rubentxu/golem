// Package projection translates accepted journal events into graph
// mutations: the Engineering Graph is the semantic projection of the Graph
// Journal (ARCHITECTURE — Fuente de verdad). It is pure and deterministic:
// the same event stream always yields the same graph, which the replay
// digest proves (M1 exit criterion).
package projection

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Rubentxu/golem/internal/ci"
	"github.com/Rubentxu/golem/internal/planning"
	"github.com/Rubentxu/golem/internal/ports"
	"github.com/Rubentxu/golem/internal/projects"
	"github.com/Rubentxu/golem/internal/release"
	"github.com/Rubentxu/golem/internal/requirements"
	"github.com/Rubentxu/golem/internal/scm"
	"github.com/Rubentxu/golem/internal/supplychain"
	"github.com/Rubentxu/golem/internal/verification"
	"github.com/Rubentxu/golem/internal/work"
)

// NodeKind constants used by the kernel projection.
const (
	KindWorkItem         = "WorkItem"
	KindRequirement      = "Requirement"
	KindWorkType         = "WorkType"
	KindProject          = "Project"
	KindIteration        = "Iteration"
	KindMilestone        = "Milestone"
	KindCommit           = "Commit"
	KindBuild            = "Build"
	KindArtifact         = "Artifact"
	KindTestRun          = "TestRun"
	KindRelease          = "Release"
	KindSBOM             = supplychain.KindSBOM
	KindPackageComponent = supplychain.KindPackageComponent
	KindVulnerability    = supplychain.KindVulnerability
	KindVEXStatement     = supplychain.KindVEXStatement
	KindAttestation      = supplychain.KindAttestation
)

// Projector maps journal events to graph mutations. Unknown event types
// yield an empty mutation (skipped by callers): projections must tolerate
// newer producers (forward compatibility, expand→migrate→contract).
type Projector struct{}

// Project interprets one event. The returned mutation has zero Ops when
// the event does not affect the graph. It consults the global Registry first;
// if no registered Projection claims the event type, it falls back to the
// legacy switch in projectSingle.
func (Projector) Project(env ports.RawEvent) (ports.GraphMutation, error) {
	// Consult the global registry first (strangler-fig pattern).
	if r := globalRegistry; r != nil {
		if m, handled, err := r.Handle(env); err != nil {
			return m, err
		} else if handled {
			return m, nil
		}
	}
	// Fall back to legacy switch.
	return projectSingle(env)
}

// Deprecated: use projection.Runner directly. This shim exists for the
// scenario.Fork overlay path and will be removed in a future cycle.
func ApplyIfHandled(p Projector, store ports.GraphStore, env ports.RawEvent) (bool, error) {
	// Single-event Runner invocation; preserves the (bool, error) semantics
	// that scenario.Fork relies on (Applied=false for unknown + no-op events
	// means OverlaySkipped++ continues to count correctly).
	r := &Runner{Projector: p, Graph: store}
	res := r.Run(context.Background(), env)
	return res.Applied, res.Err
}

// ApplyAll applies every chunk of an event's mutation. Used for events
// that may produce more than MaxOpsPerMutation (e.g. SBOM with many components).
func ApplyAll(p Projector, store ports.GraphStore, env ports.RawEvent) (bool, error) {
	chunks, err := p.ProjectAll(env)
	if err != nil {
		return false, err
	}
	if len(chunks) == 0 {
		return false, nil
	}
	for _, m := range chunks {
		if len(m.Ops) == 0 {
			continue
		}
		if _, err := store.Apply(mutationCtx(env), m); err != nil {
			return false, err
		}
	}
	return true, nil
}

// ProjectAll returns all chunks for an event. Single-chunk events return
// a single-element slice. Used for SBOM chunking (≤500 ops per chunk).
func (Projector) ProjectAll(env ports.RawEvent) ([]ports.GraphMutation, error) {
	m, err := projectSingle(env)
	if err != nil {
		return nil, err
	}
	if len(m.Ops) == 0 {
		return nil, nil
	}
	return chunkMutation(m), nil
}

// projectSingle returns the full (pre-chunking) mutation for one event.
// It is the shared implementation used by both Project and ProjectAll.
func projectSingle(env ports.RawEvent) (ports.GraphMutation, error) {
	m := ports.GraphMutation{TenantID: ports.TenantID(env.TenantID)}

	switch env.EventType {
	case work.EventItemCreated:
		var p work.ItemCreated
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return m, fmt.Errorf("projection %s: %w", env.EventType, err)
		}
		if p.ItemID == "" {
			return m, fmt.Errorf("projection %s: empty item_id", env.EventType)
		}
		attrs := map[string]any{
			"title":  p.Title,
			"type":   p.ItemType,
			"status": p.Status,
		}
		if p.TypeName != "" {
			attrs["type_name"] = p.TypeName
		}
		if p.External.Provider != "" {
			attrs["external_provider"] = p.External.Provider
			attrs["external_id"] = p.External.ExternalID
		}
		for k, v := range p.Fields {
			attrs["field_"+k] = v
		}
		m.Ops = append(m.Ops, nodeUpsert(p.ItemID, KindWorkItem, attrs))

	case work.EventItemUpdated:
		var p work.ItemUpdated
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return m, fmt.Errorf("projection %s: %w", env.EventType, err)
		}
		if p.ItemID == "" {
			return m, fmt.Errorf("projection %s: empty item_id", env.EventType)
		}
		attrs := map[string]any{}
		if p.Title != nil {
			attrs["title"] = *p.Title
		}
		if p.Status != nil {
			attrs["status"] = *p.Status
		}
		if len(attrs) > 0 {
			m.Ops = append(m.Ops, nodeUpsert(p.ItemID, KindWorkItem, attrs))
		}

	case requirements.EventRequirementCreated:
		var p requirements.RequirementCreated
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return m, fmt.Errorf("projection %s: %w", env.EventType, err)
		}
		if p.RequirementID == "" {
			return m, fmt.Errorf("projection %s: empty requirement_id", env.EventType)
		}
		m.Ops = append(m.Ops, nodeUpsert(p.RequirementID, KindRequirement, map[string]any{
			"title":     p.Title,
			"statement": p.Statement,
			"status":    p.Status,
		}))

	case projects.EventProjectCreated:
		var p projects.ProjectCreated
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return m, fmt.Errorf("projection %s: %w", env.EventType, err)
		}
		if p.ProjectID == "" {
			return m, fmt.Errorf("projection %s: empty project_id", env.EventType)
		}
		attrs := map[string]any{"name": p.Name, "description": p.Description}
		if p.External.Provider != "" {
			attrs["external_provider"] = p.External.Provider
			attrs["external_id"] = p.External.ExternalID
		}
		m.Ops = append(m.Ops, nodeUpsert(p.ProjectID, KindProject, attrs))

	case planning.EventIterationCreated:
		var p planning.IterationCreated
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return m, fmt.Errorf("projection %s: %w", env.EventType, err)
		}
		if p.IterationID == "" {
			return m, fmt.Errorf("projection %s: empty iteration_id", env.EventType)
		}
		m.Ops = append(m.Ops, nodeUpsert(p.IterationID, KindIteration, map[string]any{
			"name": p.Name, "start": p.Start.UTC().Format(time.RFC3339), "end": p.End.UTC().Format(time.RFC3339),
		}))

	case planning.EventMilestoneCreated:
		var p planning.MilestoneCreated
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return m, fmt.Errorf("projection %s: %w", env.EventType, err)
		}
		if p.MilestoneID == "" {
			return m, fmt.Errorf("projection %s: empty milestone_id", env.EventType)
		}
		m.Ops = append(m.Ops, nodeUpsert(p.MilestoneID, KindMilestone, map[string]any{
			"name": p.Name, "target_date": p.TargetDate.UTC().Format(time.RFC3339),
		}))

	case scm.EventCommitObserved:
		var p scm.CommitObserved
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return m, fmt.Errorf("projection %s: %w", env.EventType, err)
		}
		if p.SHA == "" {
			return m, fmt.Errorf("projection %s: empty sha", env.EventType)
		}
		m.Ops = append(m.Ops, nodeUpsert(p.SHA, KindCommit, map[string]any{
			"repository": p.Repository, "message": p.Message,
		}))
		for i, reqID := range p.Implements {
			m.Ops = append(m.Ops, edgeUpsert(edgeID(env.EventID, "impl", i), "IMPLEMENTS", p.SHA, reqID, causalAttrs(env)))
		}

	case ci.EventBuildCompleted:
		var p ci.BuildCompleted
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return m, fmt.Errorf("projection %s: %w", env.EventType, err)
		}
		if p.BuildID == "" || p.Commit == "" {
			return m, fmt.Errorf("projection %s: build_id and commit are mandatory", env.EventType)
		}
		m.Ops = append(m.Ops, nodeUpsert(p.BuildID, KindBuild, map[string]any{
			"pipeline": p.Pipeline, "status": p.Status, "commit": p.Commit,
		}))
		m.Ops = append(m.Ops, edgeUpsert(edgeID(env.EventID, "builtby", 0), "BUILT_BY", p.Commit, p.BuildID, causalAttrs(env)))
		for i, a := range p.Artifacts {
			m.Ops = append(m.Ops, nodeUpsert(a.Digest, a.Kind, map[string]any{"name": a.Name}))
			m.Ops = append(m.Ops, edgeUpsert(edgeID(env.EventID, "prod", i), "PRODUCED", p.BuildID, a.Digest, causalAttrs(env)))
		}

	case verification.EventTestRunReported:
		var p verification.TestRunReported
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return m, fmt.Errorf("projection %s: %w", env.EventType, err)
		}
		if p.RunID == "" || p.Verifies == "" {
			return m, fmt.Errorf("projection %s: run_id and verifies are mandatory", env.EventType)
		}
		m.Ops = append(m.Ops, nodeUpsert(p.RunID, KindTestRun, map[string]any{
			"case": p.TestCase, "status": p.Status,
		}))
		m.Ops = append(m.Ops, edgeUpsert(edgeID(env.EventID, "ver", 0), "VERIFIES", p.RunID, p.Verifies, causalAttrs(env)))

	case release.EventCandidateCreated:
		var p release.CandidateCreated
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return m, fmt.Errorf("projection %s: %w", env.EventType, err)
		}
		if p.ReleaseID == "" {
			return m, fmt.Errorf("projection %s: release_id is mandatory", env.EventType)
		}
		artifacts := make([]any, 0, len(p.Artifacts))
		for _, a := range p.Artifacts {
			artifacts = append(artifacts, a)
		}
		m.Ops = append(m.Ops, nodeUpsert(p.ReleaseID, KindRelease, map[string]any{
			"name": p.Name, "artifacts": artifacts, "gate_status": "pending",
		}))
		for i, digest := range p.Artifacts {
			m.Ops = append(m.Ops, edgeUpsert(edgeID(env.EventID, "rel", i), "RELEASED_AS", digest, p.ReleaseID, causalAttrs(env)))
		}

	case release.EventGateEvaluated:
		var p release.GateEvaluated
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return m, fmt.Errorf("projection %s: %w", env.EventType, err)
		}
		if p.ReleaseID == "" {
			return m, fmt.Errorf("projection %s: empty release_id", env.EventType)
		}
		m.Ops = append(m.Ops, nodeUpsert(p.ReleaseID, KindRelease, map[string]any{
			"gate_status": p.Result,
		}))

	case work.EventTypeRegistered:
		var p work.TypeRegistered
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return m, fmt.Errorf("projection %s: %w", env.EventType, err)
		}
		if p.Name == "" {
			return m, fmt.Errorf("projection %s: empty name", env.EventType)
		}
		attrs := map[string]any{
			"name":    p.Name,
			"initial": p.Initial,
		}
		if b, err := json.Marshal(p.States); err == nil {
			attrs["states"] = json.RawMessage(b)
		}
		if b, err := json.Marshal(p.Transitions); err == nil {
			attrs["transitions"] = json.RawMessage(b)
		}
		if b, err := json.Marshal(p.Fields); err == nil {
			attrs["fields"] = json.RawMessage(b)
		}
		m.Ops = append(m.Ops, nodeUpsert(p.Name, KindWorkType, attrs))

	case work.EventItemLinked:
		var p work.ItemLinked
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return m, fmt.Errorf("projection %s: %w", env.EventType, err)
		}
		rel := canonicalRelation(p.Relation)
		if p.FromID == "" || p.ToID == "" || rel == "" {
			return m, fmt.Errorf("projection %s: from_id, to_id and relation are mandatory", env.EventType)
		}
		m.Ops = append(m.Ops, edgeUpsert(env.EventID, rel, p.FromID, p.ToID, map[string]any{
			"source_event": env.EventID,
		}))

	case supplychain.EventSBOMIngested:
		return ProjectSBOMIngested(env)

	case supplychain.EventVulnerabilityReported:
		return ProjectVulnerabilityReported(env)

	case supplychain.EventVEXStatementRecorded:
		return ProjectVEXStatement(env)

	case supplychain.EventAttestationIngested:
		return ProjectAttestationIngested(env)
	}

	return m, nil
}

// MaxOpsPerMutation is the ceiling on ops per returned GraphMutation chunk.
// SBOM events with many components use this to split into multiple mutations.
const MaxOpsPerMutation = 500

// chunkMutation splits one mutation into multiple chunks of ≤MaxOpsPerMutation.
// All chunks share the same TenantID so they are all tenant-scoped.
func chunkMutation(m ports.GraphMutation) []ports.GraphMutation {
	if len(m.Ops) <= MaxOpsPerMutation {
		return []ports.GraphMutation{m}
	}
	var chunks []ports.GraphMutation
	for i := 0; i < len(m.Ops); i += MaxOpsPerMutation {
		end := i + MaxOpsPerMutation
		if end > len(m.Ops) {
			end = len(m.Ops)
		}
		chunks = append(chunks, ports.GraphMutation{
			TenantID: m.TenantID,
			Ops:      m.Ops[i:end],
		})
	}
	return chunks
}

func nodeUpsert(id, kind string, attrs map[string]any) ports.GraphOp {
	return ports.GraphOp{Kind: ports.OpUpsertNode, Target: id, Data: map[string]any{"kind": kind, "attributes": attrs}}
}

func edgeUpsert(id, typ, src, tgt string, attrs map[string]any) ports.GraphOp {
	return ports.GraphOp{
		Kind:   ports.OpUpsertEdge,
		Target: id,
		Data:   map[string]any{"type": typ, "source": src, "target": tgt, "attributes": attrs},
	}
}

func canonicalRelation(rel string) string {
	return strings.ToUpper(strings.TrimSpace(rel))
}

// edgeID derives a deterministic, causal edge identity from the causing
// event: replaying the journal reproduces identical edge ids.
func edgeID(eventID string, role string, i int) string {
	return fmt.Sprintf("%s#%s%d", eventID, role, i)
}

func causalAttrs(env ports.RawEvent) map[string]any {
	return map[string]any{"source_event": env.EventID}
}

// mutationCtx carries the tenant scope end-to-end (ADR-008) even for
// internal projection writes.
func mutationCtx(env ports.RawEvent) context.Context {
	return ports.WithTenant(context.Background(), ports.TenantID(env.TenantID))
}

// ---- Supply chain projection helpers ----

// ErrUnknownArtifact is returned by the attestation projector when the
// subject artifact does not exist in the graph.
var ErrUnknownArtifact = fmt.Errorf("projection: attestation subject artifact not found")

// ProjectSBOMIngested projects an SBOM ingested event into graph mutations.
func ProjectSBOMIngested(env ports.RawEvent) (ports.GraphMutation, error) {
	m := ports.GraphMutation{TenantID: ports.TenantID(env.TenantID)}
	var p supplychain.SBOMIngested
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return m, fmt.Errorf("projection %s: %w", env.EventType, err)
	}
	if p.SBOMID == "" {
		return m, fmt.Errorf("projection %s: sbom_id is mandatory", env.EventType)
	}
	if p.ArtifactDigest == "" {
		return m, fmt.Errorf("projection %s: artifact_digest is mandatory", env.EventType)
	}

	// SBOM node.
	sbomAttrs := map[string]any{
		"format":          p.Format,
		"spec_version":    p.SpecVersion,
		"source_provider": p.SourceProvider,
		"source_doc_id":   p.SourceDocID,
	}
	m.Ops = append(m.Ops, nodeUpsert(p.SBOMID, KindSBOM, sbomAttrs))

	// HAS_SBOM edge: Artifact → SBOM.
	// Use p.ArtifactDigest as-is (sha256:...) to match the Artifact node ID
	// used by the CI projector (artifact nodes are stored with digest as ID).
	m.Ops = append(m.Ops, edgeUpsert(
		edgeID(env.EventID, "hassbom", 0),
		supplychain.RelationHAS_SBOM,
		p.ArtifactDigest, p.SBOMID,
		causalAttrs(env),
	))

	// Per-component: PackageComponent node + CONTAINS edge.
	for i, comp := range p.Components {
		compID := comp.Purl
		if compID == "" {
			// Synthetic component: derive purl from name+version.
			compID = supplychain.Synthetic(comp.Name, comp.Version)
		}
		compAttrs := map[string]any{
			"name":      comp.Name,
			"version":   comp.Version,
			"synthetic": comp.Synthetic,
		}
		m.Ops = append(m.Ops, nodeUpsert(compID, KindPackageComponent, compAttrs))
		m.Ops = append(m.Ops, edgeUpsert(
			edgeID(env.EventID, "con", i),
			supplychain.RelationCONTAINS,
			p.SBOMID, compID,
			causalAttrs(env),
		))
	}

	return m, nil
}

// ProjectVulnerabilityReported projects a vulnerability reported event into graph mutations.
func ProjectVulnerabilityReported(env ports.RawEvent) (ports.GraphMutation, error) {
	m := ports.GraphMutation{TenantID: ports.TenantID(env.TenantID)}
	var p supplychain.VulnerabilityReported
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return m, fmt.Errorf("projection %s: %w", env.EventType, err)
	}
	if p.VulnID == "" {
		return m, fmt.Errorf("projection %s: vuln_id is mandatory", env.EventType)
	}

	vulnID := "vuln-" + p.VulnID
	vulnAttrs := map[string]any{
		"severity": p.Severity,
		"status":   p.Status,
		"provider": p.Provider,
	}
	m.Ops = append(m.Ops, nodeUpsert(vulnID, KindVulnerability, vulnAttrs))

	// AFFECTED_BY edge: PackageComponent → Vulnerability (only if purl is non-empty).
	if p.ComponentPurl != "" {
		m.Ops = append(m.Ops, edgeUpsert(
			edgeID(env.EventID, "aff", 0),
			supplychain.RelationAFFECTED_BY,
			p.ComponentPurl, vulnID,
			causalAttrs(env),
		))
	}

	return m, nil
}

// ProjectVEXStatement projects a VEX statement event into graph mutations.
func ProjectVEXStatement(env ports.RawEvent) (ports.GraphMutation, error) {
	m := ports.GraphMutation{TenantID: ports.TenantID(env.TenantID)}
	var p supplychain.VEXStatementRecorded
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return m, fmt.Errorf("projection %s: %w", env.EventType, err)
	}
	if p.StatementID == "" {
		return m, fmt.Errorf("projection %s: statement_id is mandatory", env.EventType)
	}
	if p.VulnID == "" {
		return m, fmt.Errorf("projection %s: vuln_id is mandatory", env.EventType)
	}

	vexAttrs := map[string]any{
		"vuln_id":        p.VulnID,
		"product_digest": p.ProductDigest,
		"product_purl":   p.ProductPurl,
		"status":         p.Status,
		"justification":  p.Justification,
		"provider":       p.Provider,
	}
	m.Ops = append(m.Ops, nodeUpsert(p.StatementID, KindVEXStatement, vexAttrs))

	// MITIGATED_BY edge: Vulnerability → VEXStatement only when status is
	// not_affected or fixed (per spec and ADR-055). in_remediation does NOT
	// suppress the vulnerability in the gate walk.
	vulnID := "vuln-" + p.VulnID
	switch p.Status {
	case supplychain.VEXStatusNotAffected, supplychain.VEXStatusFixed:
		m.Ops = append(m.Ops, edgeUpsert(
			edgeID(env.EventID, "mit", 0),
			supplychain.RelationMITIGATED_BY,
			vulnID, p.StatementID,
			causalAttrs(env),
		))
	}

	return m, nil
}

// ProjectAttestationIngested projects an attestation ingested event into graph mutations.
func ProjectAttestationIngested(env ports.RawEvent) (ports.GraphMutation, error) {
	m := ports.GraphMutation{TenantID: ports.TenantID(env.TenantID)}
	var p supplychain.AttestationIngested
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return m, fmt.Errorf("projection %s: %w", env.EventType, err)
	}
	if p.AttestationID == "" {
		return m, fmt.Errorf("projection %s: attestation_id is mandatory", env.EventType)
	}
	if p.ArtifactDigest == "" {
		return m, fmt.Errorf("projection %s: artifact_digest is mandatory", env.EventType)
	}

	attAttrs := map[string]any{
		"predicate_type":      p.PredicateType,
		"builder_id":          p.BuilderID,
		"verification_result": p.VerificationResult,
		"verification_reason": p.VerificationReason,
	}
	m.Ops = append(m.Ops, nodeUpsert(p.AttestationID, KindAttestation, attAttrs))

	// ATTESTED_BY edge: Artifact → Attestation with verification attrs.
	m.Ops = append(m.Ops, edgeUpsert(
		edgeID(env.EventID, "att", 0),
		supplychain.RelationATTESTED_BY,
		p.ArtifactDigest, p.AttestationID,
		map[string]any{
			"source_event":        env.EventID,
			"verification_result": p.VerificationResult,
			"verification_reason": p.VerificationReason,
		},
	))

	return m, nil
}
