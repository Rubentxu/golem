package lens

import "testing"

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
