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
	"github.com/Rubentxu/golem/internal/requirements"
	"github.com/Rubentxu/golem/internal/scm"
	"github.com/Rubentxu/golem/internal/verification"
	"github.com/Rubentxu/golem/internal/work"
)

// NodeKind constants used by the kernel projection.
const (
	KindWorkItem    = "WorkItem"
	KindRequirement = "Requirement"
	KindWorkType    = "WorkType"
	KindProject     = "Project"
	KindIteration   = "Iteration"
	KindMilestone   = "Milestone"
	KindCommit      = "Commit"
	KindBuild       = "Build"
	KindArtifact    = "Artifact"
	KindTestRun     = "TestRun"
)

// Projector maps journal events to graph mutations. Unknown event types
// yield an empty mutation (skipped by callers): projections must tolerate
// newer producers (forward compatibility, expand→migrate→contract).
type Projector struct{}

// Project interprets one event. The returned mutation has zero Ops when
// the event does not affect the graph.
func (Projector) Project(env ports.RawEvent) (ports.GraphMutation, error) {
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
			// Namespace custom fields to avoid collisions with canonical
			// attributes.
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
		// Commit ──BUILT_BY──> Build (VISION lineage direction).
		m.Ops = append(m.Ops, edgeUpsert(edgeID(env.EventID, "builtby", 0), "BUILT_BY", p.Commit, p.BuildID, causalAttrs(env)))
		for i, a := range p.Artifacts {
			// ADR-022: artifact identity is the digest itself.
			m.Ops = append(m.Ops, nodeUpsert(a.Digest, a.Kind, map[string]any{
				"name": a.Name,
			}))
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
		// Slices are JSON-roundtrippable attributes (digest-stable).
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
		// Edge identity is the causing event: deterministic under replay
		// and auditable back to the journal (causality everywhere).
		m.Ops = append(m.Ops, edgeUpsert(env.EventID, rel, p.FromID, p.ToID, map[string]any{
			"source_event": env.EventID,
		}))
	}

	return m, nil
}

// ApplyIfHandled is a convenience for projection loops: it applies the
// mutation when the event produced ops.
func ApplyIfHandled(p Projector, store ports.GraphStore, env ports.RawEvent) (bool, error) {
	m, err := p.Project(env)
	if err != nil {
		return false, err
	}
	if len(m.Ops) == 0 {
		return false, nil
	}
	if _, err := store.Apply(mutationCtx(env), m); err != nil {
		return false, err
	}
	return true, nil
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
