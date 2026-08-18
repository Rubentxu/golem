package tck

import (
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

// TestFrame_Validate_CatalogClosed verifies that permissions outside the
// closed catalog are rejected.
func TestFrame_Validate_CatalogClosed(t *testing.T) {
	// Valid permissions
	validFrame := ports.Frame{
		ID:          "f-001",
		TenantID:    "t-test",
		Goal:        "Test goal",
		Permissions: []string{ports.PermissionRead, ports.PermissionWrite},
	}
	if err := validFrame.Validate(); err != nil {
		t.Errorf("expected valid frame, got error: %v", err)
	}

	// Invalid permission
	invalidFrame := ports.Frame{
		ID:          "f-002",
		TenantID:    "t-test",
		Permissions: []string{"invalid-permission"},
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
