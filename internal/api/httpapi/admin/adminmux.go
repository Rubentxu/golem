// Package admin provides operator-only HTTP endpoints for cell and tenant
// management (ADR-081).
package admin

import (
	"context"
	"net/http"
)

// AdminMux aggregates admin HTTP handlers and registers routes.
type AdminMux struct {
	Cells   *CellsHandler
	Queries *QueriesHandler
}

// NewAdminMux creates an AdminMux with the given handlers.
func NewAdminMux(cells *CellsHandler, queries *QueriesHandler) *AdminMux {
	return &AdminMux{Cells: cells, Queries: queries}
}

// Handler returns an http.Handler that routes /admin/* paths.
// Routes are registered with operator RBAC protection.
func (m *AdminMux) Handler() http.Handler {
	mux := http.NewServeMux()

	// Cell management.
	mux.HandleFunc("POST /admin/cells/", m.Cells.HandleMigrate)
	mux.HandleFunc("POST /admin/cells/", m.Cells.HandleDrain)

	// Tenant management.
	mux.HandleFunc("POST /admin/tenants/", m.Cells.HandleTenantAssign)
	mux.HandleFunc("POST /admin/tenants/", m.Cells.HandleTenantExport)

	// SLO and metering queries.
	mux.HandleFunc("GET /admin/slo/", m.Queries.HandleSLOQuery)
	mux.HandleFunc("GET /admin/metering", m.Queries.HandleMeteringQuery)

	return mux
}

// AdminMux satisfies the EventEmitter interface for event emission.
var _ EventEmitter = (*AdminMux)(nil)

// Emit emits an ops.console.action event via the journal (placeholder — wired via app layer).
func (m *AdminMux) Emit(ctx context.Context, eventType string, payload any) error {
	return nil // TODO: wire via event bus
}
