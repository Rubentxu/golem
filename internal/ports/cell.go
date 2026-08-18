package ports

import (
	"context"
)

// CellID is the identifier for a cell (REQ-CELL-001).
type CellID string

// CellHealth describes the health status of a cell (REQ-CELL-002).
type CellHealth struct {
	LagSeconds  int64  `json:"lag_seconds"`
	JournalHead int64  `json:"journal_head"`
	Status      string `json:"status"` // "healthy", "degraded", "draining"
}

// CellRecord describes a cell in the catalog (REQ-CELL-001..004).
type CellRecord struct {
	ID     CellID `json:"cell_id"`
	Region string `json:"region"`
	Status string `json:"status"`
}

// MigrationPlan describes a tenant migration from one cell to another (REQ-MIG-001).
type MigrationPlan struct {
	TenantID    string `json:"tenant_id"`
	FromCell    CellID `json:"from_cell"`
	ToCell      CellID `json:"to_cell"`
	DryRun      bool   `json:"dry_run"`
	DiffDigest  string `json:"diff_digest,omitempty"`
	StartedAt   int64  `json:"started_at,omitempty"`
	CompletedAt int64  `json:"completed_at,omitempty"`
}

// CellRouter routes tenants to cells deterministically (REQ-CELL-002).
//
// Route returns the cell for a tenant. It must be deterministic: same tenantID
// always returns the same cell while no migration is in progress.
//
// Migrate executes a tenant migration. When dryRun=true, it performs shadow-reads
// and diff but does not modify the data plane.
//
// List returns all cells in the catalog.
//
// Health returns the health status of a cell.
type CellRouter interface {
	Route(ctx context.Context, tenantID string) (CellID, error)
	Migrate(ctx context.Context, plan MigrationPlan) error
	List(ctx context.Context) ([]CellRecord, error)
	Health(ctx context.Context, cellID CellID) (CellHealth, error)
}
