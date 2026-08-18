package tck

import (
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

// TestFrame_Validate_CatalogClosed verifies that permissions outside the
// closed catalog are rejected. Per ADR-058 and spec §6, the closed catalog
// contains exactly 5 permissions: graph.read, graph.read:lens, proposal.write,
// proposal.apply, evidence.write.
func TestFrame_Validate_CatalogClosed(t *testing.T) {
	// Valid permissions from the ADR-058 closed catalog
	validFrame := ports.Frame{
		ID:          "f-001",
		TenantID:    "t-test",
		Goal:        "Test goal",
		Permissions: []string{"graph.read", "graph.read:lens"},
	}
	if err := validFrame.Validate(); err != nil {
		t.Errorf("expected valid frame, got error: %v", err)
	}

	// Invalid permission (old catalog values are no longer valid)
	invalidFrame := ports.Frame{
		ID:          "f-002",
		TenantID:    "t-test",
		Permissions: []string{"read", "write"},
	}
	if err := invalidFrame.Validate(); err == nil {
		t.Error("expected error for invalid permission")
	}
}

// TestFrame_BudgetDefaults verifies that Frame without budget is valid.
func TestFrame_BudgetDefaults(t *testing.T) {
	frame := ports.Frame{
		ID:       "f-003",
		TenantID: "t-test",
		Goal:     "Test goal without budget",
		// No Permissions field = valid empty slice
	}
	if err := frame.Validate(); err != nil {
		t.Errorf("expected valid frame without permissions, got error: %v", err)
	}
}
