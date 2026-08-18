package ports

import (
	"context"
	"testing"
)

// TestCellRouter_RouteDeterministic verifies that Route returns the same cell
// for the same tenantID across multiple calls (REQ-CELL-002).
func TestCellRouter_RouteDeterministic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create a CellRouter implementation for testing.
	// In RED phase, we define the interface behavior before implementation.
	router := &mockCellRouter{cells: []CellID{"cell-a", "cell-b", "cell-c", "cell-d"}}

	tenantID := "tenant-123"

	// First call
	cell1, err := router.Route(ctx, tenantID)
	if err != nil {
		t.Fatalf("Route[1] error: %v", err)
	}

	// Second call - must return same cell
	cell2, err := router.Route(ctx, tenantID)
	if err != nil {
		t.Fatalf("Route[2] error: %v", err)
	}

	if cell1 != cell2 {
		t.Errorf("Route deterministic: got %v on call 1, %v on call 2", cell1, cell2)
	}

	// Multiple tenants should distribute across cells (hash stability).
	seen := make(map[CellID]bool)
	for i := 0; i < 100; i++ {
		tid := "tenant-" + string(rune('a'+i%10))
		cell, err := router.Route(ctx, tid)
		if err != nil {
			t.Fatalf("Route error for %s: %v", tid, err)
		}
		seen[cell] = true
	}

	// With 4 cells and 100 tenants, we expect distribution across multiple cells.
	if len(seen) < 2 {
		t.Errorf("expected distribution across multiple cells, got %d cell(s)", len(seen))
	}
}

// mockCellRouter is a minimal implementation for testing the interface contract.
type mockCellRouter struct {
	cells []CellID
}

func (m *mockCellRouter) Route(ctx context.Context, tenantID string) (CellID, error) {
	// Simple hash-based routing for testing.
	hash := 0
	for _, c := range tenantID {
		hash += int(c)
	}
	idx := hash % len(m.cells)
	return m.cells[idx], nil
}

func (m *mockCellRouter) Migrate(ctx context.Context, plan MigrationPlan) error {
	return nil
}

func (m *mockCellRouter) List(ctx context.Context) ([]CellRecord, error) {
	records := make([]CellRecord, len(m.cells))
	for i, id := range m.cells {
		records[i] = CellRecord{ID: id, Region: "us-east-1", Status: "healthy"}
	}
	return records, nil
}

func (m *mockCellRouter) Health(ctx context.Context, cellID CellID) (CellHealth, error) {
	return CellHealth{LagSeconds: 0, JournalHead: 100, Status: "healthy"}, nil
}
