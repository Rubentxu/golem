package tck

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	graphmem "github.com/Rubentxu/golem/adapters/graph/memstore"
	"github.com/Rubentxu/golem/internal/behavior"
	"github.com/Rubentxu/golem/internal/clock"
	idgen "github.com/Rubentxu/golem/internal/ids"
	"github.com/Rubentxu/golem/internal/lens"
	"github.com/Rubentxu/golem/internal/ports"
	"github.com/Rubentxu/golem/internal/scenario"
)

// seedScenarioBase builds a base graph: 2 components + 1 release.
func seedScenarioBase(t *testing.T, g ports.GraphStore, tenant string) {
	_, err := g.Apply(context.Background(), ports.GraphMutation{TenantID: ports.TenantID(tenant), Ops: []ports.GraphOp{
		{Kind: ports.OpUpsertNode, Target: "c1", Data: map[string]any{"kind": "PackageComponent", "attributes": map[string]any{"purl": "pkg:npm/a@1.0.0"}}},
		{Kind: ports.OpUpsertNode, Target: "c2", Data: map[string]any{"kind": "PackageComponent", "attributes": map[string]any{"purl": "pkg:npm/b@1.0.0"}}},
		{Kind: ports.OpUpsertNode, Target: "r1", Data: map[string]any{"kind": "Release", "attributes": map[string]any{"name": "v1"}}},
		{Kind: ports.OpUpsertEdge, Target: "e1", Data: map[string]any{"type": "RELEASED_AS", "source": "c1", "target": "r1"}},
	}})
	if err != nil {
		t.Fatalf("seed base: %v", err)
	}
}

// S16 — fork with empty overlay produces an identical graph.
func TestScenarioFork_EmptyOverlay(t *testing.T) {
	base := graphmem.NewGraph()
	seedScenarioBase(t, base, "t")
	target := graphmem.NewGraph()

	res, err := scenario.Fork(context.Background(), base, target, "t", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.NodesCopied != 3 || res.EdgesCopied != 1 {
		t.Fatalf("fork copied %d nodes %d edges, want 3/1", res.NodesCopied, res.EdgesCopied)
	}
	report, err := scenario.Diff(context.Background(), base, target, "t")
	if err != nil {
		t.Fatal(err)
	}
	if report.NodeDiffCount != 0 || report.EdgeDiffCount != 0 {
		t.Errorf("empty overlay must diff clean: %+v", report)
	}
}

// S17 — fork with a node overlay produces exactly that delta.
func TestScenarioFork_NodeOverlay(t *testing.T) {
	base := graphmem.NewGraph()
	seedScenarioBase(t, base, "t")
	target := graphmem.NewGraph()

	// Overlay event the projector handles: a new vulnerability node
	// (no component_purl → no edge).
	overlay := ports.RawEvent{
		EventID:       "evt-cve9",
		TenantID:      "t",
		StreamID:      "supplychain",
		EventType:     "supplychain.vulnerability.reported.v1",
		SchemaVersion: 1,
		OccurredAt:    time.Now(),
		Payload:       json.RawMessage(`{"vuln_id":"CVE-9","severity":"high","status":"open","provider":"demo"}`),
	}
	res, err := scenario.Fork(context.Background(), base, target, "t", []ports.RawEvent{overlay})
	if err != nil {
		t.Fatal(err)
	}
	if res.OverlayApplied != 1 {
		t.Fatalf("overlay applied = %d, want 1", res.OverlayApplied)
	}
	report, err := scenario.Diff(context.Background(), base, target, "t")
	if err != nil {
		t.Fatal(err)
	}
	if report.NodeDiffCount != 1 || len(report.NodeDiffs) != 1 {
		t.Fatalf("diff = %+v, want exactly one node diff", report)
	}
	if report.NodeDiffs[0].ID != "vuln-CVE-9" || report.NodeDiffs[0].Op != "added" {
		t.Errorf("node diff = %+v", report.NodeDiffs[0])
	}
}

// S18/S19 — diff determinism and clean case (already covered by S16/S17);
// here: changed node + affected release classification.
func TestScenarioDiff_ChangedNodeAffectsRelease(t *testing.T) {
	base := graphmem.NewGraph()
	seedScenarioBase(t, base, "t")
	target := graphmem.NewGraph()
	if _, err := scenario.Fork(context.Background(), base, target, "t", nil); err != nil {
		t.Fatal(err)
	}
	// Mutate r1 attributes directly on the fork.
	if _, err := target.Apply(context.Background(), ports.GraphMutation{TenantID: "t", Ops: []ports.GraphOp{
		{Kind: ports.OpUpsertNode, Target: "r1", Data: map[string]any{"kind": "Release", "attributes": map[string]any{"name": "v1", "note": "changed"}}},
	}}); err != nil {
		t.Fatal(err)
	}
	report, err := scenario.Diff(context.Background(), base, target, "t")
	if err != nil {
		t.Fatal(err)
	}
	if report.NodeDiffCount != 1 || report.NodeDiffs[0].Op != "changed" {
		t.Fatalf("diff = %+v", report)
	}
	if len(report.AffectedReleases) != 1 || report.AffectedReleases[0] != "r1" {
		t.Errorf("affected releases = %v, want [r1]", report.AffectedReleases)
	}
}

// S23/S24 — shadow report: identical versions diff clean; divergent
// versions (same lens over a seeded vuln graph) report the difference.
func TestScenarioShadow_V1V2Diff(t *testing.T) {
	graph := graphmem.NewGraph()
	_, err := graph.Apply(context.Background(), ports.GraphMutation{TenantID: "t", Ops: []ports.GraphOp{
		{Kind: ports.OpUpsertNode, Target: "vuln-1", Data: map[string]any{"kind": "Vulnerability", "attributes": map[string]any{"cve": "CVE-9"}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	eng := behavior.NewEngine(behavior.NewRegistry(), graph, clock.Fixed(ts))
	spec := &lens.Spec{Roots: []string{"vuln-1"}, MaxDepth: 1, MaxNodes: 10, MaxEdges: 10, Evidence: true}

	events := []ports.RawEvent{behaviorEvent(t, "evt.s", "t")}
	// v1 == v2 (same handler) → clean report (S23).
	report, err := scenario.Shadow(context.Background(), eng, graph, events,
		&behavior.Behavior{ID: "b", Version: "1", Subscriptions: []string{"evt.s"}, LensSpec: spec, Handler: behavior.CveGateV1},
		&behavior.Behavior{ID: "b", Version: "2", Subscriptions: []string{"evt.s"}, LensSpec: spec, Handler: behavior.CveGateV1})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Diffs) != 0 {
		t.Errorf("identical versions must diff clean: %+v", report.Diffs)
	}

	// v1 vs v2 divergence (S24): v2 adds the mitigation event + proposal.
	report, err = scenario.Shadow(context.Background(), eng, graph, events,
		&behavior.Behavior{ID: "b", Version: "1", Subscriptions: []string{"evt.s"}, LensSpec: spec, Handler: behavior.CveGateV1},
		&behavior.Behavior{ID: "b", Version: "2", Subscriptions: []string{"evt.s"}, LensSpec: spec, Handler: behavior.CveGateV2})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Diffs) != 1 {
		t.Fatalf("diffs = %d, want 1", len(report.Diffs))
	}
	if report.Diffs[0].EventID != "evt-1" {
		t.Errorf("diff event = %q", report.Diffs[0].EventID)
	}
	if report.Diffs[0].Difference == "" {
		t.Error("difference summary must be populated")
	}
}

// Lens-based shadow primer ensuring the demo handlers see a vuln node.
func TestScenarioShadow_WithLensGraph(t *testing.T) {
	g := graphmem.NewGraph()
	_, err := g.Apply(context.Background(), ports.GraphMutation{TenantID: "t", Ops: []ports.GraphOp{
		{Kind: ports.OpUpsertNode, Target: "vuln-1", Data: map[string]any{"kind": "Vulnerability", "attributes": map[string]any{"cve": "CVE-9"}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	spec := &lens.Spec{Roots: []string{"vuln-1"}, MaxDepth: 1, MaxNodes: 10, MaxEdges: 10, Evidence: true}
	eng := behavior.NewEngine(behavior.NewRegistry(), g, clock.Fixed(ts))
	events := []ports.RawEvent{behaviorEvent(t, "evt.s", "t")}

	report, err := scenario.Shadow(context.Background(), eng, g, events,
		&behavior.Behavior{ID: "b", Version: "1", Subscriptions: []string{"evt.s"}, LensSpec: spec, Handler: behavior.CveGateV1},
		&behavior.Behavior{ID: "b", Version: "2", Subscriptions: []string{"evt.s"}, LensSpec: spec, Handler: behavior.CveGateV2})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Diffs) != 1 {
		t.Fatalf("diffs = %d, want 1 (v2 adds mitigation)", len(report.Diffs))
	}
}

var _ = idgen.NewGenerator // keep import for future promote test wiring
