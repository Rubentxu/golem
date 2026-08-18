// Package cell implements the cell bounded context (REQ-CELL-001).
package cell

import "github.com/Rubentxu/golem/internal/ports"

// Cell represents a cell in the system (REQ-CELL-001).
type Cell struct {
	ID     ports.CellID
	Region string
	Status CellStatus
}

// CellStatus represents the operational status of a cell.
type CellStatus string

const (
	CellStatusHealthy  CellStatus = "healthy"
	CellStatusDegraded CellStatus = "degraded"
	CellStatusDraining CellStatus = "draining"
	CellStatusDown     CellStatus = "down"
)

// NewCell creates a new cell.
func NewCell(id ports.CellID, region string) *Cell {
	return &Cell{
		ID:     id,
		Region: region,
		Status: CellStatusHealthy,
	}
}

// CanAcceptAppend returns true if the cell can accept new journal appends.
func (c *Cell) CanAcceptAppend() bool {
	return c.Status == CellStatusHealthy
}

// Invariants validates the cell's invariants.
func (c *Cell) Invariants() error {
	if c.ID == "" {
		return ErrCellIDRequired
	}
	if c.Region == "" {
		return ErrCellRegionRequired
	}
	return nil
}

// Cell errors.
var (
	ErrCellIDRequired     = &cellError{"cell ID is required"}
	ErrCellRegionRequired = &cellError{"cell region is required"}
)

type cellError struct{ msg string }

func (e *cellError) Error() string { return e.msg }
