// Package scenario implements the GOLEM scenario model of
// SCENARIOS_FORK_DIFF_PROMOTE.md: fork a base journal position with an
// overlay event delta, diff the forked graph against the base, and
// promote the delta atomically. All graph work is store-agnostic — the
// caller wires the stores (live graph = source, scenario graph = target).
package scenario

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/Rubentxu/golem/internal/application/projection"
	"github.com/Rubentxu/golem/internal/ports"
)

// ErrScenarioConflict is returned when promotion violates lineage: the
// fork base position is ahead of the journal head (forked from the
// future) or the scenario is not approved.
var ErrScenarioConflict = errors.New("scenario: lineage conflict or missing approval")

// ForkResult summarises a fork execution.
type ForkResult struct {
	NodesCopied    int
	EdgesCopied    int
	OverlayApplied int // events the projector handled
	OverlaySkipped int // events the projector did not handle
}

// Fork builds the scenario graph in target: a full snapshot of source
// (copied node/edge by node/edge — no full-graph serialisation in v1,
// the canonical export path arrives with file-backed stores in M6.1)
// plus the overlay events projected on top.
//
// Overlay events that the projector does not handle are skipped — the
// same graceful-unhandled semantics as the migration harness replay.
func Fork(ctx context.Context, source, target ports.GraphStore, tenant ports.TenantID, overlay []ports.RawEvent) (ForkResult, error) {
	nodes, err := source.ListNodes(ctx, tenant)
	if err != nil {
		return ForkResult{}, fmt.Errorf("scenario: list nodes: %w", err)
	}
	edges, err := source.ListEdges(ctx, tenant)
	if err != nil {
		return ForkResult{}, fmt.Errorf("scenario: list edges: %w", err)
	}

	ops := make([]ports.GraphOp, 0, len(nodes)+len(edges))
	for _, n := range nodes {
		ops = append(ops, ports.GraphOp{
			Kind:   ports.OpUpsertNode,
			Target: n.ID,
			Data:   map[string]any{"kind": n.Kind, "attributes": n.Attributes},
		})
	}
	for _, e := range edges {
		ops = append(ops, ports.GraphOp{
			Kind:   ports.OpUpsertEdge,
			Target: e.ID,
			Data:   map[string]any{"type": e.Type, "source": e.SourceID, "target": e.TargetID, "attributes": e.Attributes},
		})
	}
	if len(ops) > 0 {
		if _, err := target.Apply(ctx, ports.GraphMutation{TenantID: tenant, Ops: ops}); err != nil {
			return ForkResult{}, fmt.Errorf("scenario: snapshot apply: %w", err)
		}
	}

	res := ForkResult{NodesCopied: len(nodes), EdgesCopied: len(edges)}
	projector := projection.Projector{}
	for _, env := range overlay {
		applied, err := projection.ApplyIfHandled(projector, target, env)
		if err != nil {
			return res, fmt.Errorf("scenario: overlay event %s: %w", env.EventID, err)
		}
		if applied {
			res.OverlayApplied++
		} else {
			res.OverlaySkipped++
		}
	}
	return res, nil
}

// NodeDiff is one structural difference between base and forked graphs.
type NodeDiff struct {
	ID   string `json:"id"`
	Op   string `json:"op"` // added | removed | changed
	Kind string `json:"kind"`
}

// EdgeDiff is one structural edge difference.
type EdgeDiff struct {
	ID   string `json:"id"`
	Op   string `json:"op"`
	Type string `json:"type"`
}

// PolicyDecision is a v1 placeholder capturing gate decisions observed in
// the forked graph (M6.1: full policy decision extraction).
type PolicyDecision struct {
	Gate   string `json:"gate"`
	Reason string `json:"reason"`
}

// DiffReport is the scenario diff output of
// SCENARIOS_FORK_DIFF_PROMOTE.md §Diff: nodes/edges, policy decisions and
// affected releases. Deterministic: diffs sorted by ID.
type DiffReport struct {
	NodeDiffs        []NodeDiff       `json:"node_diffs"`
	EdgeDiffs        []EdgeDiff       `json:"edge_diffs"`
	PolicyDecisions  []PolicyDecision `json:"policy_decisions"`
	AffectedReleases []string         `json:"affected_releases"`
	NodeDiffCount    int              `json:"node_diff_count"`
	EdgeDiffCount    int              `json:"edge_diff_count"`
}

// Diff compares base and forked graphs structurally. Deterministic.
func Diff(ctx context.Context, base, forked ports.GraphStore, tenant ports.TenantID) (*DiffReport, error) {
	baseNodes, err := base.ListNodes(ctx, tenant)
	if err != nil {
		return nil, fmt.Errorf("scenario: base nodes: %w", err)
	}
	forkedNodes, err := forked.ListNodes(ctx, tenant)
	if err != nil {
		return nil, fmt.Errorf("scenario: forked nodes: %w", err)
	}
	baseEdges, err := base.ListEdges(ctx, tenant)
	if err != nil {
		return nil, fmt.Errorf("scenario: base edges: %w", err)
	}
	forkedEdges, err := forked.ListEdges(ctx, tenant)
	if err != nil {
		return nil, fmt.Errorf("scenario: forked edges: %w", err)
	}

	report := &DiffReport{PolicyDecisions: []PolicyDecision{}}
	baseByID := map[string]ports.Node{}
	for _, n := range baseNodes {
		baseByID[n.ID] = n
	}
	forkedByID := map[string]ports.Node{}
	for _, n := range forkedNodes {
		forkedByID[n.ID] = n
	}

	for id, n := range forkedByID {
		if _, ok := baseByID[id]; !ok {
			report.NodeDiffs = append(report.NodeDiffs, NodeDiff{ID: id, Op: "added", Kind: n.Kind})
			if n.Kind == "Release" {
				report.AffectedReleases = append(report.AffectedReleases, id)
			}
			continue
		}
		if !nodesEqual(baseByID[id], n) {
			report.NodeDiffs = append(report.NodeDiffs, NodeDiff{ID: id, Op: "changed", Kind: n.Kind})
			if n.Kind == "Release" {
				report.AffectedReleases = append(report.AffectedReleases, id)
			}
		}
	}
	for id, n := range baseByID {
		if _, ok := forkedByID[id]; !ok {
			report.NodeDiffs = append(report.NodeDiffs, NodeDiff{ID: id, Op: "removed", Kind: n.Kind})
		}
	}

	baseE := map[string]ports.Edge{}
	for _, e := range baseEdges {
		baseE[e.ID] = e
	}
	forkedE := map[string]ports.Edge{}
	for _, e := range forkedEdges {
		forkedE[e.ID] = e
	}
	for id, e := range forkedE {
		if _, ok := baseE[id]; !ok {
			report.EdgeDiffs = append(report.EdgeDiffs, EdgeDiff{ID: id, Op: "added", Type: e.Type})
			continue
		}
		if !edgesEqual(baseE[id], e) {
			report.EdgeDiffs = append(report.EdgeDiffs, EdgeDiff{ID: id, Op: "changed", Type: e.Type})
		}
	}
	for id, e := range baseE {
		if _, ok := forkedE[id]; !ok {
			report.EdgeDiffs = append(report.EdgeDiffs, EdgeDiff{ID: id, Op: "removed", Type: e.Type})
		}
	}

	sort.Slice(report.NodeDiffs, func(i, j int) bool { return report.NodeDiffs[i].ID < report.NodeDiffs[j].ID })
	sort.Slice(report.EdgeDiffs, func(i, j int) bool { return report.EdgeDiffs[i].ID < report.EdgeDiffs[j].ID })
	sort.Strings(report.AffectedReleases)
	report.NodeDiffCount = len(report.NodeDiffs)
	report.EdgeDiffCount = len(report.EdgeDiffs)
	return report, nil
}

func nodesEqual(a, b ports.Node) bool {
	if a.ID != b.ID || a.Kind != b.Kind {
		return false
	}
	ja, _ := json.Marshal(a.Attributes)
	jb, _ := json.Marshal(b.Attributes)
	return string(ja) == string(jb)
}

func edgesEqual(a, b ports.Edge) bool {
	if a.ID != b.ID || a.Type != b.Type || a.SourceID != b.SourceID || a.TargetID != b.TargetID {
		return false
	}
	ja, _ := json.Marshal(a.Attributes)
	jb, _ := json.Marshal(b.Attributes)
	return string(ja) == string(jb)
}
