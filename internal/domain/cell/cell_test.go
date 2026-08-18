package cell

import "testing"

// TestCell_Invariants verifies cell invariant checks.
func TestCell_Invariants(t *testing.T) {
	t.Parallel()

	// Valid cell.
	c := NewCell("cell-a", "us-east-1")
	if err := c.Invariants(); err != nil {
		t.Errorf("valid cell: expected nil error, got %v", err)
	}

	// Empty ID.
	c2 := NewCell("", "us-east-1")
	if err := c2.Invariants(); err == nil {
		t.Error("empty ID: expected error")
	}

	// Empty region.
	c3 := NewCell("cell-b", "")
	if err := c3.Invariants(); err == nil {
		t.Error("empty region: expected error")
	}
}

// TestCell_CanAcceptAppend verifies append eligibility.
func TestCell_CanAcceptAppend(t *testing.T) {
	t.Parallel()
	c := NewCell("cell-a", "us-east-1")

	if !c.CanAcceptAppend() {
		t.Error("healthy cell should accept appends")
	}

	c.Status = CellStatusDraining
	if c.CanAcceptAppend() {
		t.Error("draining cell should not accept appends")
	}
}
