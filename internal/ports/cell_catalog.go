package ports

import "context"

// CellCatalog provides read access to the cell registry (REQ-CELL-001..004).
// It is a read-only view of cell metadata, separate from routing decisions.
type CellCatalog interface {
	// Get returns a cell record by ID.
	Get(ctx context.Context, cellID CellID) (CellRecord, error)
	// List returns all cell records.
	List(ctx context.Context) ([]CellRecord, error)
}
