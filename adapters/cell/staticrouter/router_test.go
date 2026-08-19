package staticrouter

import (
	"context"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

// TestStaticRouter_RouteDeterminism verifies that Route is deterministic:
// the same tenantID always returns the same cell (REQ-CELL-002).
func TestStaticRouter_RouteDeterminism(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	router := NewRouter([]ports.CellID{"cell-a", "cell-b", "cell-c"})

	tenantID := "tenant-deterministic-test"
	first, err := router.Route(ctx, tenantID)
	if err != nil {
		t.Fatalf("Route error: %v", err)
	}

	for i := 0; i < 100; i++ {
		cell, err := router.Route(ctx, tenantID)
		if err != nil {
			t.Fatalf("Route error at iteration %d: %v", i, err)
		}
		if cell != first {
			t.Errorf("Route non-deterministic: iteration 0 returned %s, iteration %d returned %s", first, i, cell)
		}
	}
}

// TestStaticRouter_ChurnReduction verifies that jump hash reduces churn
// vs modular hash. When adding a cell, ~1/(n+1) keys should remap, not 100%.
// This is the defining property of jump consistent hash (REQ-W4-Jump-Hash).
func TestStaticRouter_ChurnReduction(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// With 3 cells, add 1 → ~25% of keys should remap (not 100%).
	cells3 := []ports.CellID{"cell-a", "cell-b", "cell-c"}
	cells4 := []ports.CellID{"cell-a", "cell-b", "cell-c", "cell-d"}

	router3 := NewRouter(cells3)
	router4 := NewRouter(cells4)

	const numTenants = 1000
	var remapped int
	for i := 0; i < numTenants; i++ {
		tenantID := "tenant-" + string(rune('0'+i%10)) + string(rune('a'+i/10%26)) + string(rune('A'+i/100%26))
		cell3, _ := router3.Route(ctx, tenantID)
		cell4, _ := router4.Route(ctx, tenantID)
		if cell3 != cell4 {
			remapped++
		}
	}

	// Jump hash: expected remap ≈ 1/(n+1) = 1/4 = 25%.
	// Allow range 15-40% (generous for statistical variance with 1000 samples).
	// Modular hash would remap ~100% of keys (all tenantIDs have different hash distributions).
	churnRate := float64(remapped) / float64(numTenants)
	if churnRate > 0.40 {
		t.Errorf("Churn rate too high: %d/%d = %.1f%% (want ~25%%, modular hash would be ~100%%)",
			remapped, numTenants, churnRate*100)
	}
	if churnRate < 0.15 {
		t.Errorf("Churn rate suspiciously low: %d/%d = %.1f%% (want ~25%%)",
			remapped, numTenants, churnRate*100)
	}
}

// TestStaticRouter_SingleCell verifies edge case: router with 1 cell
// always returns that cell (REQ-W4-Single-Cell).
func TestStaticRouter_SingleCell(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	router := NewRouter([]ports.CellID{"cell-only"})

	for _, tenantID := range []string{"tenant-a", "tenant-b", "tenant-xyz"} {
		cell, err := router.Route(ctx, tenantID)
		if err != nil {
			t.Errorf("Route(%q) error: %v", tenantID, err)
		}
		if cell != "cell-only" {
			t.Errorf("Route(%q): got %s, want cell-only", tenantID, cell)
		}
	}
}

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
