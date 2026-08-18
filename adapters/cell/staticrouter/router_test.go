package staticrouter

import (
	"context"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

// TestStaticRouter_OverrideBeatsHash verifies that explicit overrides take precedence
// over jump hash routing (REQ-CELL-002).
func TestStaticRouter_OverrideBeatsHash(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	router := NewRouter([]ports.CellID{"cell-a", "cell-b", "cell-c"})

	tenantID := "tenant-123"

	// Get the hash-based cell.
	hashCell, err := router.Route(ctx, tenantID)
	if err != nil {
		t.Fatalf("Route error: %v", err)
	}

	// Set an override.
	router.SetOverride(tenantID, "cell-b")

	// Get the cell again - should return override.
	overrideCell, err := router.Route(ctx, tenantID)
	if err != nil {
		t.Fatalf("Route after override error: %v", err)
	}

	// Override should take precedence.
	if overrideCell != "cell-b" {
		t.Errorf("Route with override: got %s, want cell-b", overrideCell)
	}

	// Hash cell should be different from override.
	if hashCell == overrideCell {
		t.Logf("Note: hash happened to match override by chance")
	}
}
