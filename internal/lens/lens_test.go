package lens

import (
	"testing"

	"github.com/Rubentxu/golem/internal/canonical"
)

// TestSpecValidate is the only internal test: pure validation, no graph
// adapter imports (ADR-047 — graph-dependent tests live in tck/).
func TestSpecValidate(t *testing.T) {
	if err := (Spec{}).Validate(); err == nil {
		t.Error("empty spec must fail (no roots)")
	}
	base := Spec{Roots: []string{"a"}, MaxDepth: 2, MaxNodes: 10, MaxEdges: 10}
	if err := base.Validate(); err != nil {
		t.Errorf("base spec must validate: %v", err)
	}
	bad := base
	bad.TimeWindow = "90D" // missing P
	if err := bad.Validate(); err == nil {
		t.Error("time_window without P must fail")
	}
}

func TestAgentChangeLens_SpecShape(t *testing.T) {
	spec := AgentChangeLens([]string{"eval-001"}, 3, 50, 100)
	if len(spec.Roots) != 1 || spec.Roots[0] != "eval-001" {
		t.Errorf("unexpected roots: %v", spec.Roots)
	}
	if spec.MaxDepth != 3 || spec.MaxNodes != 50 || spec.MaxEdges != 100 {
		t.Errorf("unexpected bounds: depth=%d nodes=%d edges=%d", spec.MaxDepth, spec.MaxNodes, spec.MaxEdges)
	}
	if !spec.Evidence {
		t.Error("AgentChangeLens must have Evidence=true")
	}
	nodeTypes := map[string]bool{}
	for _, k := range spec.NodeTypes {
		nodeTypes[k] = true
	}
	if !nodeTypes[canonical.AgentEvalNodeKind] {
		t.Errorf("expected AgentEval node type in AgentChangeLens, got %v", spec.NodeTypes)
	}
	edgeTypes := map[string]bool{}
	for _, e := range spec.EdgeTypes {
		edgeTypes[e] = true
	}
	if !edgeTypes[canonical.EdgeTypeEVALUATED] || !edgeTypes[canonical.EdgeTypeOBSERVED] {
		t.Errorf("expected EVALUATED and OBSERVED edge types, got %v", spec.EdgeTypes)
	}
}

func TestRequirementTraceLens_SpecShape(t *testing.T) {
	spec := RequirementTraceLens([]string{"req-001"}, 2, 20, 40)
	if len(spec.Roots) != 1 {
		t.Errorf("unexpected roots: %v", spec.Roots)
	}
	if spec.MaxDepth != 2 || spec.MaxNodes != 20 || spec.MaxEdges != 40 {
		t.Errorf("unexpected bounds")
	}
}

func TestArchitectureImpactLens_SpecShape(t *testing.T) {
	spec := ArchitectureImpactLens([]string{"adr-042"}, 3, 30, 60)
	if spec.Roots[0] != "adr-042" {
		t.Errorf("unexpected roots: %v", spec.Roots)
	}
}

func TestUATContextLens_SpecShape(t *testing.T) {
	spec := UATContextLens([]string{"req-001"}, 4, 100, 200)
	if spec.MaxDepth != 4 || spec.MaxNodes != 100 || spec.MaxEdges != 200 {
		t.Errorf("unexpected bounds")
	}
}
