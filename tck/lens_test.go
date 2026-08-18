package tck

import (
	"context"
	"errors"
	"testing"

	graphmem "github.com/Rubentxu/golem/adapters/graph/memstore"
	"github.com/Rubentxu/golem/internal/lens"
	"github.com/Rubentxu/golem/internal/ports"
)

// seedLensGraph builds: a -[CONTAINS]-> b -[AFFECTED_BY]-> c (Component,
// Component, Vulnerability) — S10/S11/S12/S13 shape.
func seedLensGraph(t *testing.T) ports.GraphStore {
	t.Helper()
	g := graphmem.NewGraph()
	_, err := g.Apply(context.Background(), ports.GraphMutation{
		TenantID: "t-lens",
		Ops: []ports.GraphOp{
			{Kind: ports.OpUpsertNode, Target: "a", Data: map[string]any{"kind": "Component", "attributes": map[string]any{"name": "a"}}},
			{Kind: ports.OpUpsertNode, Target: "b", Data: map[string]any{"kind": "Component", "attributes": map[string]any{"name": "b"}}},
			{Kind: ports.OpUpsertNode, Target: "c", Data: map[string]any{"kind": "Vulnerability", "attributes": map[string]any{"cve": "CVE-1"}}},
			{Kind: ports.OpUpsertEdge, Target: "e1", Data: map[string]any{"type": "CONTAINS", "source": "a", "target": "b"}},
			{Kind: ports.OpUpsertEdge, Target: "e2", Data: map[string]any{"type": "AFFECTED_BY", "source": "b", "target": "c"}},
		},
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return g
}

func baseLensSpec() lens.Spec {
	return lens.Spec{Roots: []string{"a"}, MaxDepth: 2, MaxNodes: 10, MaxEdges: 10}
}

// S10 — basic lens materialises expected nodes and edges.
func TestLensExecute_Basic(t *testing.T) {
	g := seedLensGraph(t)
	res, err := lens.Execute(context.Background(), g, "t-lens", baseLensSpec())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(res.Nodes) != 3 || len(res.Edges) != 2 {
		t.Fatalf("nodes=%d edges=%d, want 3/2", len(res.Nodes), len(res.Edges))
	}
	if res.Nodes[0].ID != "a" || res.Nodes[2].ID != "c" {
		t.Errorf("nodes not sorted by ID: %v, %v, %v", res.Nodes[0].ID, res.Nodes[1].ID, res.Nodes[2].ID)
	}
}

// S11 — budget violation fails closed.
func TestLensExecute_BudgetExceeded(t *testing.T) {
	g := seedLensGraph(t)
	spec := baseLensSpec()
	spec.MaxNodes = 2
	if _, err := lens.Execute(context.Background(), g, "t-lens", spec); !errors.Is(err, lens.ErrLensBudgetExceeded) {
		t.Fatalf("err = %v, want ErrLensBudgetExceeded", err)
	}
}

// S12 — determinism byte-identical across runs.
func TestLensExecute_Deterministic(t *testing.T) {
	g := seedLensGraph(t)
	a, err := lens.Execute(context.Background(), g, "t-lens", baseLensSpec())
	if err != nil {
		t.Fatal(err)
	}
	b, err := lens.Execute(context.Background(), g, "t-lens", baseLensSpec())
	if err != nil {
		t.Fatal(err)
	}
	ja, _ := a.ToJSON()
	jb, _ := b.ToJSON()
	if string(ja) != string(jb) {
		t.Errorf("non-deterministic:\n%s\n%s", ja, jb)
	}
}

// S13 — evidence policy strips attributes when disabled.
func TestLensExecute_EvidencePolicy(t *testing.T) {
	g := seedLensGraph(t)
	spec := baseLensSpec()
	spec.Evidence = false
	res, err := lens.Execute(context.Background(), g, "t-lens", spec)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range res.Nodes {
		if n.Attributes != nil {
			t.Errorf("node %s kept attributes with evidence=false", n.ID)
		}
	}
	spec.Evidence = true
	res, err = lens.Execute(context.Background(), g, "t-lens", spec)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range res.Nodes {
		if n.ID == "a" && n.Attributes["name"] == "a" {
			found = true
		}
	}
	if !found {
		t.Error("evidence=true must include attributes")
	}
}

// seedFamilyGraph is the M4 blast-radius walk shape.
func seedFamilyGraph(t *testing.T) ports.GraphStore {
	t.Helper()
	g := graphmem.NewGraph()
	ops := []ports.GraphOp{
		{Kind: ports.OpUpsertNode, Target: "component-1", Data: map[string]any{"kind": "PackageComponent", "attributes": map[string]any{"purl": "pkg:npm/foo@1.0.0"}}},
		{Kind: ports.OpUpsertNode, Target: "sbom-1", Data: map[string]any{"kind": "SBOM", "attributes": map[string]any{"format": "spdx"}}},
		{Kind: ports.OpUpsertNode, Target: "artifact-1", Data: map[string]any{"kind": "Artifact", "attributes": map[string]any{"sha256": "a"}}},
		{Kind: ports.OpUpsertNode, Target: "release-1", Data: map[string]any{"kind": "Release", "attributes": map[string]any{"name": "v1.0"}}},
		{Kind: ports.OpUpsertNode, Target: "cve-1", Data: map[string]any{"kind": "Vulnerability", "attributes": map[string]any{"cve": "CVE-2026-1"}}},
		{Kind: ports.OpUpsertEdge, Target: "e-contains", Data: map[string]any{"type": "CONTAINS", "source": "sbom-1", "target": "component-1"}},
		{Kind: ports.OpUpsertEdge, Target: "e-has-sbom", Data: map[string]any{"type": "HAS_SBOM", "source": "artifact-1", "target": "sbom-1"}},
		{Kind: ports.OpUpsertEdge, Target: "e-released-as", Data: map[string]any{"type": "RELEASED_AS", "source": "artifact-1", "target": "release-1"}},
		{Kind: ports.OpUpsertEdge, Target: "e-affected", Data: map[string]any{"type": "AFFECTED_BY", "source": "component-1", "target": "cve-1"}},
	}
	if _, err := g.Apply(context.Background(), ports.GraphMutation{TenantID: "t-fam", Ops: ops}); err != nil {
		t.Fatal(err)
	}
	return g
}

// S14 — VulnerabilityImpactLens reaches the affected release.
func TestVulnerabilityImpactLens(t *testing.T) {
	g := seedFamilyGraph(t)
	res, err := lens.Execute(context.Background(), g, "t-fam",
		lens.VulnerabilityImpactLens([]string{"component-1"}, 5, 50, 50))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	ids := map[string]bool{}
	for _, n := range res.Nodes {
		ids[n.ID] = true
	}
	for _, want := range []string{"component-1", "sbom-1", "artifact-1", "release-1", "cve-1"} {
		if !ids[want] {
			t.Errorf("VulnerabilityImpactLens missing %s (got %v)", want, ids)
		}
	}
}

// S15 — ReleaseEvidenceLens from a Release root returns the full chain.
func TestReleaseEvidenceLens(t *testing.T) {
	g := seedFamilyGraph(t)
	res, err := lens.Execute(context.Background(), g, "t-fam",
		lens.ReleaseEvidenceLens([]string{"release-1"}, 5, 50, 50))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	kinds := map[string]int{}
	for _, n := range res.Nodes {
		kinds[n.Kind]++
	}
	if kinds["Release"] != 1 {
		t.Errorf("must include Release root, got %v", kinds)
	}
	if kinds["SBOM"] == 0 || kinds["PackageComponent"] == 0 {
		t.Errorf("evidence chain incomplete: %v", kinds)
	}
}
