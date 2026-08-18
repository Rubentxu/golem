package tck

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	graphmem "github.com/Rubentxu/golem/adapters/graph/memstore"
	journalmem "github.com/Rubentxu/golem/adapters/journal/memstore"
	"github.com/Rubentxu/golem/internal/behavior"
	"github.com/Rubentxu/golem/internal/clock"
	idgen "github.com/Rubentxu/golem/internal/ids"
	"github.com/Rubentxu/golem/internal/lens"
	"github.com/Rubentxu/golem/internal/ports"
	"github.com/Rubentxu/golem/internal/scenario"
)

// S25/S26 — the M6 exit criterion demo, end to end:
//
//	supply-chain snapshot + CVE overlay → cve-gate v1 vs v2 shadow →
//	diff report → promote (v2 override) → scenario.promoted.v1 →
//	what-if reproducible (×2 byte-identical)
func TestM6ExitCriterionDemo(t *testing.T) {
	ts := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	clk := clock.Fixed(ts)
	idsGen := idgen.NewGenerator(clk)

	// 1. Base graph: a PackageComponent with an SBOM and a Release.
	base := graphmem.NewGraph()
	_, err := base.Apply(context.Background(), ports.GraphMutation{TenantID: "t", Ops: []ports.GraphOp{
		{Kind: ports.OpUpsertNode, Target: "pkg:npm/acme@1.0.0", Data: map[string]any{"kind": "PackageComponent", "attributes": map[string]any{"purl": "pkg:npm/acme@1.0.0"}}},
		{Kind: ports.OpUpsertNode, Target: "rel-1", Data: map[string]any{"kind": "Release", "attributes": map[string]any{"name": "acme-v1"}}},
		{Kind: ports.OpUpsertEdge, Target: "e-rel", Data: map[string]any{"type": "RELEASED_AS", "source": "pkg:npm/acme@1.0.0", "target": "rel-1"}},
	}})
	if err != nil {
		t.Fatal(err)
	}

	// 2. Overlay: a new CVE affecting the component.
	overlayEvent := ports.RawEvent{
		EventID:       "ovl-1",
		TenantID:      "t",
		StreamID:      "supplychain",
		EventType:     "supplychain.vulnerability.reported.v1",
		SchemaVersion: 1,
		OccurredAt:    ts,
		Actor:         ports.Actor{Type: "service", ID: "scenario-demo"},
		Payload: json.RawMessage(`{"vuln_id":"CVE-2026-999","severity":"critical",` +
			`"status":"open","component_purl":"pkg:npm/acme@1.0.0","provider":"demo"}`),
	}

	// 3. Fork: snapshot + overlay into a scenario graph (what-if #1).
	forked := graphmem.NewGraph()
	forkRes, err := scenario.Fork(context.Background(), base, forked, "t", []ports.RawEvent{overlayEvent})
	if err != nil {
		t.Fatal(err)
	}
	if forkRes.OverlayApplied != 1 {
		t.Fatalf("overlay applied = %d, want 1", forkRes.OverlayApplied)
	}

	// 4. Shadow: cve-gate v1 vs v2 over the overlay event, lens rooted at
	//    the new vuln node.
	spec := &lens.Spec{Roots: []string{"vuln-CVE-2026-999"}, MaxDepth: 2, MaxNodes: 50, MaxEdges: 50, Evidence: true}
	eng := behavior.NewEngine(behavior.NewRegistry(), forked, clk)
	shadowReport, err := scenario.Shadow(context.Background(), eng, forked,
		[]ports.RawEvent{overlayEvent},
		&behavior.Behavior{ID: "cve-gate", Version: "1", Subscriptions: []string{overlayEvent.EventType}, LensSpec: spec, Handler: behavior.CveGateV1},
		&behavior.Behavior{ID: "cve-gate", Version: "2", Subscriptions: []string{overlayEvent.EventType}, LensSpec: spec, Handler: behavior.CveGateV2})
	if err != nil {
		t.Fatal(err)
	}
	if len(shadowReport.Diffs) != 1 {
		t.Fatalf("shadow diffs = %d, want 1 (v2 adds mitigation)", len(shadowReport.Diffs))
	}

	// 5. Diff report for the fork.
	diffReport, err := scenario.Diff(context.Background(), base, forked, "t")
	if err != nil {
		t.Fatal(err)
	}
	if diffReport.NodeDiffCount != 1 {
		t.Fatalf("node diffs = %d, want 1 (the vuln node)", diffReport.NodeDiffCount)
	}

	// 6. Promote the scenario with the v2 override.
	journal := journalmem.NewJournal()
	overlayJSON, _ := json.Marshal(overlayEvent)
	scn := &ports.Scenario{
		ID:           "scn-cve999",
		TenantID:     "t",
		BasePosition: 0,
		Overlay:      []json.RawMessage{overlayJSON},
		Overrides:    map[string]string{"cve-gate": "2"},
		Approved:     true,
		CreatedAt:    ts,
	}
	promRes, err := scenario.Promote(context.Background(), journal, idsGen, clk, scn)
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if promRes.EventsApplied != 1 {
		t.Fatalf("events applied = %d, want 1", promRes.EventsApplied)
	}
	events, _, err := journal.Replay(context.Background(), 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	var promoted bool
	for _, e := range events {
		if e.EventType == ports.EventScenarioPromoted {
			promoted = true
			var p map[string]any
			if err := json.Unmarshal(e.Payload, &p); err != nil {
				t.Fatal(err)
			}
			if p["scenario_id"] != "scn-cve999" {
				t.Errorf("promoted payload = %v", p)
			}
		}
	}
	if !promoted {
		t.Fatal("scenario.promoted.v1 missing from journal")
	}

	// 7. What-if reproducible: second fork + diff must be byte-identical.
	forked2 := graphmem.NewGraph()
	if _, err := scenario.Fork(context.Background(), base, forked2, "t", []ports.RawEvent{overlayEvent}); err != nil {
		t.Fatal(err)
	}
	diff2, err := scenario.Diff(context.Background(), base, forked2, "t")
	if err != nil {
		t.Fatal(err)
	}
	j1, _ := json.Marshal(diffReport)
	j2, _ := json.Marshal(diff2)
	if string(j1) != string(j2) {
		t.Errorf("what-if NOT reproducible:\n%s\n%s", j1, j2)
	}
}
