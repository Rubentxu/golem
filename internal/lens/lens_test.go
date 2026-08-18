package lens

import (
	"context"
	"errors"
	"testing"

	graphmem "github.com/Rubentxu/golem/adapters/graph/memstore"
	"github.com/Rubentxu/golem/internal/ports"
)

// seedGraph builds: a -[CONTAINS]-> b -[AFFECTED_BY]-> c, all kind Component.
func seedGraph(t *testing.T) ports.GraphStore {
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

func baseSpec() Spec {
	return Spec{Roots: []string{"a"}, MaxDepth: 2, MaxNodes: 10, MaxEdges: 10}
}

// S10 — a basic lens materialises the expected nodes and edges.
func TestLensExecute_Basic(t *testing.T) {
	g := seedGraph(t)
	res, err := Execute(context.Background(), g, "t-lens", baseSpec())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(res.Nodes) != 3 || len(res.Edges) != 2 {
		t.Fatalf("nodes=%d edges=%d, want 3/2", len(res.Nodes), len(res.Edges))
	}
	// sorted by ID
	if res.Nodes[0].ID != "a" || res.Nodes[2].ID != "c" {
		t.Errorf("nodes not sorted: %v %v %v", res.Nodes[0].ID, res.Nodes[1].ID, res.Nodes[2].ID)
	}
}

// S11 — budget violation fails closed with no partial result.
func TestLensExecute_BudgetExceeded(t *testing.T) {
	g := seedGraph(t)
	spec := baseSpec()
	spec.MaxNodes = 2 // graph has 3
	_, err := Execute(context.Background(), g, "t-lens", spec)
	if !errors.Is(err, ErrLensBudgetExceeded) {
		t.Fatalf("err = %v, want ErrLensBudgetExceeded", err)
	}
}

// S12 — determinism: same input twice, byte-identical JSON.
func TestLensExecute_Deterministic(t *testing.T) {
	g := seedGraph(t)
	a, err := Execute(context.Background(), g, "t-lens", baseSpec())
	if err != nil {
		t.Fatal(err)
	}
	b, err := Execute(context.Background(), g, "t-lens", baseSpec())
	if err != nil {
		t.Fatal(err)
	}
	ja, _ := a.ToJSON()
	jb, _ := b.ToJSON()
	if string(ja) != string(jb) {
		t.Errorf("non-deterministic serialisation:\n%s\n%s", ja, jb)
	}
}

// S13 — evidence policy: without evidence attributes are stripped.
func TestLensExecute_EvidencePolicy(t *testing.T) {
	g := seedGraph(t)
	spec := baseSpec()
	spec.Evidence = false
	res, err := Execute(context.Background(), g, "t-lens", spec)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range res.Nodes {
		if n.Attributes != nil {
			t.Errorf("node %s kept attributes with evidence=false: %v", n.ID, n.Attributes)
		}
	}
	for _, e := range res.Edges {
		if e.Attributes != nil {
			t.Errorf("edge %s kept attributes with evidence=false", e.ID)
		}
	}

	spec.Evidence = true
	res, err = Execute(context.Background(), g, "t-lens", spec)
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

// Spec validation.
func TestSpecValidate(t *testing.T) {
	if err := (Spec{}).Validate(); err == nil {
		t.Error("empty spec must fail (no roots)")
	}
	if err := baseSpec().Validate(); err != nil {
		t.Errorf("base spec must validate: %v", err)
	}
	bad := baseSpec()
	bad.TimeWindow = "90D" // missing P
	if err := bad.Validate(); err == nil {
		t.Error("time_window without P must fail")
	}
}
