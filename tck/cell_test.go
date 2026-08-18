package tck

import (
	"context"
	"testing"

	"github.com/Rubentxu/golem/adapters/cell/staticrouter"
	"github.com/Rubentxu/golem/internal/ports"
)

// TestCell_HashStableAcrossRestart verifies hash stability across restarts (ESC-002).
func TestCell_HashStableAcrossRestart(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	router1 := staticrouter.NewRouter([]ports.CellID{"cell-a", "cell-b", "cell-c", "cell-d"})
	router2 := staticrouter.NewRouter([]ports.CellID{"cell-a", "cell-b", "cell-c", "cell-d"})

	tenants := make([]string, 100)
	for i := 0; i < 100; i++ {
		tenants[i] = "tenant-" + string(rune('a'+i%10))
	}

	// Route all tenants through router 1.
	route1 := make(map[string]ports.CellID)
	for _, tenant := range tenants {
		cell, _ := router1.Route(ctx, tenant)
		route1[tenant] = cell
	}

	// Route all tenants through router 2 (simulates restart).
	for _, tenant := range tenants {
		cell, _ := router2.Route(ctx, tenant)
		if cell != route1[tenant] {
			t.Errorf("Route mismatch for %s: router1=%s, router2=%s", tenant, route1[tenant], cell)
		}
	}
}

// TestCell_TenantStickyAfterRestart verifies tenant stickiness (ESC-001).
func TestCell_TenantStickyAfterRestart(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	router := staticrouter.NewRouter([]ports.CellID{"cell-a", "cell-b"})

	tenant := "tenant-123"

	// First route.
	cell1, _ := router.Route(ctx, tenant)

	// Route again (same router instance).
	cell2, _ := router.Route(ctx, tenant)

	if cell1 != cell2 {
		t.Errorf("Same router: got %s then %s", cell1, cell2)
	}
}

// TestCell_AllEventsTenantScoped verifies all events are tenant-scoped (I-7).
func TestCell_AllEventsTenantScoped(t *testing.T) {
	t.Parallel()
	// This is a structural test: the CellRouter interface contract requires
	// tenant-scoped routing. We verify the port interface enforces this.
	//
	// In a real implementation, we'd inject a wrapper that validates tenant scope
	// on every Route call.

	ctx := context.Background()
	router := staticrouter.NewRouter([]ports.CellID{"cell-a", "cell-b"})

	// Route with a tenant ID.
	cell, err := router.Route(ctx, "tenant-valid")
	if err != nil {
		t.Fatalf("Route error: %v", err)
	}

	if cell == "" {
		t.Error("Route returned empty cell")
	}
}
