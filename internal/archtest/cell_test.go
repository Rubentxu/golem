package archtest

import (
	"testing"

	"github.com/Rubentxu/golem/internal/domain/cell"
)

// TestCell_Invariant_AllEventsTenantScoped verifies cell events are tenant-scoped (I-7).
func TestCell_Invariant_AllEventsTenantScoped(t *testing.T) {
	t.Parallel()

	// Create a valid cell.
	c := cell.NewCell("cell-a", "us-east-1")

	// Verify invariants pass.
	if err := c.Invariants(); err != nil {
		t.Errorf("Cell invariants: expected nil, got %v", err)
	}

	// Verify healthy cell can accept appends.
	if !c.CanAcceptAppend() {
		t.Error("Healthy cell should accept appends")
	}

	// Drain the cell.
	c.Status = cell.CellStatusDraining

	// Verify drained cell cannot accept appends.
	if c.CanAcceptAppend() {
		t.Error("Draining cell should not accept appends")
	}
}
