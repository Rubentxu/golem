package memstore

import (
	"context"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

func TestCatalog_Get(t *testing.T) {
	c := New()
	ctx := context.Background()

	// Assign a tenant first
	err := c.Assign(ctx, "tenant-1", "cell-A")
	if err != nil {
		t.Fatalf("Assign failed: %v", err)
	}

	// Get should return the tenant
	rec, err := c.Get(ctx, "tenant-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if rec.ID != "tenant-1" {
		t.Errorf("expected tenant-1, got %s", rec.ID)
	}
	if rec.CellID != "cell-A" {
		t.Errorf("expected cell-A, got %s", rec.CellID)
	}

	// Get non-existent tenant should fail
	_, err = c.Get(ctx, "tenant-nonexistent")
	if err == nil {
		t.Error("expected error for non-existent tenant")
	}
}

func TestCatalog_Assign(t *testing.T) {
	c := New()
	ctx := context.Background()

	// Assign new tenant
	err := c.Assign(ctx, "tenant-new", "cell-B")
	if err != nil {
		t.Fatalf("Assign new tenant failed: %v", err)
	}

	rec, err := c.Get(ctx, "tenant-new")
	if err != nil {
		t.Fatalf("Get after assign failed: %v", err)
	}
	if rec.CellID != "cell-B" {
		t.Errorf("expected cell-B, got %s", rec.CellID)
	}

	// Re-assign existing tenant
	err = c.Assign(ctx, "tenant-new", "cell-C")
	if err != nil {
		t.Fatalf("Re-assign failed: %v", err)
	}

	rec, err = c.Get(ctx, "tenant-new")
	if err != nil {
		t.Fatalf("Get after re-assign failed: %v", err)
	}
	if rec.CellID != "cell-C" {
		t.Errorf("expected cell-C after re-assign, got %s", rec.CellID)
	}
}

func TestCatalog_List(t *testing.T) {
	c := New()
	ctx := context.Background()

	// Create tenants across different cells
	c.Assign(ctx, "tenant-1", "cell-A")
	c.Assign(ctx, "tenant-2", "cell-A")
	c.Assign(ctx, "tenant-3", "cell-B")

	// List all
	all, err := c.List(ctx, ports.TenantFilter{})
	if err != nil {
		t.Fatalf("List all failed: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 tenants, got %d", len(all))
	}

	// List by cell
	cellA, err := c.List(ctx, ports.TenantFilter{CellID: "cell-A"})
	if err != nil {
		t.Fatalf("List by cell failed: %v", err)
	}
	if len(cellA) != 2 {
		t.Errorf("expected 2 tenants in cell-A, got %d", len(cellA))
	}

	// List by tier (none set, should return none)
	standard, err := c.List(ctx, ports.TenantFilter{Tier: ports.TenantTierStandard})
	if err != nil {
		t.Fatalf("List by tier failed: %v", err)
	}
	if len(standard) != 0 {
		t.Errorf("expected 0 tenants with standard tier, got %d", len(standard))
	}
}

func TestCatalog_Export(t *testing.T) {
	c := New()
	ctx := context.Background()

	// Export non-existent should fail
	_, err := c.Export(ctx, "tenant-nonexistent")
	if err == nil {
		t.Error("expected error for non-existent tenant export")
	}

	// Assign and export
	c.Assign(ctx, "tenant-export", "cell-X")
	data, err := c.Export(ctx, "tenant-export")
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}
	// Just verify it doesn't panic and returns something
	if len(data) == 0 {
		t.Error("expected non-empty export data")
	}
}

func TestCatalog_Concurrent(t *testing.T) {
	c := New()
	ctx := context.Background()

	// Run concurrent Assign and Get
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func(id int) {
			tenantID := "tenant-concurrent"
			cellID := ports.CellID("cell")
			_ = c.Assign(ctx, tenantID, cellID)
			_, _ = c.Get(ctx, tenantID)
			done <- true
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}
