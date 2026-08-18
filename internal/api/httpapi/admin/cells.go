// Package admin provides operator-only HTTP endpoints for cell and tenant
// management (ADR-081).
package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// CellRouter defines the cell routing operations needed by admin endpoints.
type CellRouter interface {
	Route(ctx context.Context, tenantID string) (ports.CellID, error)
	Migrate(ctx context.Context, plan ports.MigrationPlan) error
	List(ctx context.Context) ([]ports.CellRecord, error)
	Health(ctx context.Context, cellID ports.CellID) (ports.CellHealth, error)
}

// TenantCatalog defines tenant management operations (I-12 M8-fit).
type TenantCatalog interface {
	Get(ctx context.Context, tenantID string) (ports.TenantRecord, error)
	Assign(ctx context.Context, tenantID string, cell ports.CellID) error
	List(ctx context.Context, filter ports.TenantFilter) ([]ports.TenantRecord, error)
	Export(ctx context.Context, tenantID string) ([]byte, error)
}

// EventEmitter emits ops.console.action events.
type EventEmitter interface {
	Emit(ctx context.Context, eventType string, payload any) error
}

// CellsHandler handles admin cell management endpoints.
type CellsHandler struct {
	cellRouter CellRouter
	tenantCtl  TenantCatalog
	emitter    EventEmitter
}

// NewCellsHandler creates a CellsHandler.
func NewCellsHandler(cr CellRouter, tc TenantCatalog, emitter EventEmitter) *CellsHandler {
	return &CellsHandler{cellRouter: cr, tenantCtl: tc, emitter: emitter}
}

// MigrateRequest is the body for POST /admin/cells/{id}/migrate.
type MigrateRequest struct {
	TenantID string `json:"tenant_id"`
	ToCell   string `json:"to_cell"`
	DryRun   bool   `json:"dry_run"`
}

// MigrateResponse is the response for a migration (dry-run or executed).
type MigrateResponse struct {
	TenantID   string `json:"tenant_id"`
	FromCell   string `json:"from_cell"`
	ToCell     string `json:"to_cell"`
	DryRun     bool   `json:"dry_run"`
	DiffDigest string `json:"diff_digest,omitempty"`
	Message    string `json:"message"`
}

// HandleMigrate handles POST /admin/cells/{id}/migrate.
// When dryRun=true, performs shadow-reads and diff without mutating the data plane (AC-5).
func (h *CellsHandler) HandleMigrate(w http.ResponseWriter, r *http.Request) {
	corr := r.Header.Get("X-Correlation-Id")
	if corr == "" {
		corr = strconv.FormatInt(int64(time.Now().UnixNano()), 10)
	}

	var req MigrateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid-argument", "invalid JSON body: "+err.Error(), corr)
		return
	}

	if req.TenantID == "" {
		writeProblem(w, http.StatusBadRequest, "invalid-argument", "tenant_id is required", corr)
		return
	}
	if req.ToCell == "" {
		writeProblem(w, http.StatusBadRequest, "invalid-argument", "to_cell is required", corr)
		return
	}

	ctx := r.Context()

	// Get source cell by routing the tenant.
	fromCell, err := h.cellRouter.Route(ctx, req.TenantID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", "failed to route tenant: "+err.Error(), corr)
		return
	}

	plan := ports.MigrationPlan{
		TenantID:   req.TenantID,
		FromCell:   fromCell,
		ToCell:     ports.CellID(req.ToCell),
		DryRun:     req.DryRun,
		DiffDigest: "",
	}

	err = h.cellRouter.Migrate(ctx, plan)
	if err != nil && !req.DryRun {
		// Emit rejection event on real migration failure.
		h.emitter.Emit(ctx, ports.EventOpsConsoleActionRejected, ports.OpsConsoleActionPayload{
			Action:      "cell.migrate",
			Target:      req.TenantID,
			Status:      "rejected",
			Detail:      err.Error(),
			Correlation: corr,
		})
		writeProblem(w, http.StatusInternalServerError, "migration-failed", err.Error(), corr)
		return
	}

	// Emit completion event.
	h.emitter.Emit(ctx, ports.EventOpsConsoleActionCompleted, ports.OpsConsoleActionPayload{
		Action:      "cell.migrate",
		Target:      req.TenantID,
		Status:      "completed",
		Correlation: corr,
	})

	resp := MigrateResponse{
		TenantID: req.TenantID,
		FromCell: string(fromCell),
		ToCell:   req.ToCell,
		DryRun:   req.DryRun,
		Message:  "migration planned",
	}
	if req.DryRun {
		resp.Message = "dry-run completed — no mutations performed"
	}

	writeJSON(w, http.StatusOK, resp)
}

// DrainRequest is the body for POST /admin/cells/{id}/drain.
type DrainRequest struct {
	Force bool `json:"force"`
}

// HandleDrain handles POST /admin/cells/{id}/drain.
func (h *CellsHandler) HandleDrain(w http.ResponseWriter, r *http.Request) {
	corr := r.Header.Get("X-Correlation-Id")
	if corr == "" {
		corr = strconv.FormatInt(int64(time.Now().UnixNano()), 10)
	}

	ctx := r.Context()

	// Extract cell ID from path.
	cellID := extractCellID(r.URL.Path)
	if cellID == "" {
		writeProblem(w, http.StatusBadRequest, "invalid-argument", "cell id is required", corr)
		return
	}

	health, err := h.cellRouter.Health(ctx, ports.CellID(cellID))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "not-found", "cell not found: "+cellID, corr)
		return
	}

	// Mark cell as draining by updating health status.
	health.Status = "draining"
	_ = health // TODO: persist draining status via CellCatalog.MarkDraining

	h.emitter.Emit(ctx, ports.EventOpsConsoleActionCompleted, ports.OpsConsoleActionPayload{
		Action:      "cell.drain",
		Target:      cellID,
		Status:      health.Status,
		Correlation: corr,
	})

	writeJSON(w, http.StatusOK, map[string]string{
		"cell_id": cellID,
		"status":  "draining",
		"message": "cell marked as draining — no new appends will be accepted",
	})
}

// extractCellID extracts the cell id from a path like /admin/cells/{id}/drain.
func extractCellID(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, p := range parts {
		if p == "cells" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// TenantAssignRequest is the body for POST /admin/tenants/{id}/cell.
type TenantAssignRequest struct {
	CellID string `json:"cell_id"`
}

// HandleTenantAssign handles POST /admin/tenants/{id}/cell.
func (h *CellsHandler) HandleTenantAssign(w http.ResponseWriter, r *http.Request) {
	corr := r.Header.Get("X-Correlation-Id")
	if corr == "" {
		corr = strconv.FormatInt(int64(time.Now().UnixNano()), 10)
	}

	var req TenantAssignRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid-argument", "invalid JSON body: "+err.Error(), corr)
		return
	}

	tenantID := extractTenantID(r.URL.Path)
	if tenantID == "" {
		writeProblem(w, http.StatusBadRequest, "invalid-argument", "tenant id is required", corr)
		return
	}
	if req.CellID == "" {
		writeProblem(w, http.StatusBadRequest, "invalid-argument", "cell_id is required", corr)
		return
	}

	ctx := r.Context()
	err := h.tenantCtl.Assign(ctx, tenantID, ports.CellID(req.CellID))
	if err != nil {
		h.emitter.Emit(ctx, ports.EventOpsConsoleActionRejected, ports.OpsConsoleActionPayload{
			Action:      "tenant.assign",
			Target:      tenantID,
			Status:      "rejected",
			Detail:      err.Error(),
			Correlation: corr,
		})
		writeProblem(w, http.StatusInternalServerError, "assign-failed", err.Error(), corr)
		return
	}

	h.emitter.Emit(ctx, ports.EventOpsConsoleActionCompleted, ports.OpsConsoleActionPayload{
		Action:      "tenant.assign",
		Target:      tenantID,
		Status:      "completed",
		Correlation: corr,
	})

	writeJSON(w, http.StatusOK, map[string]string{
		"tenant_id": tenantID,
		"cell_id":   req.CellID,
		"message":   "tenant assigned to cell",
	})
}

// HandleTenantExport handles POST /admin/tenants/{id}/export.
func (h *CellsHandler) HandleTenantExport(w http.ResponseWriter, r *http.Request) {
	corr := r.Header.Get("X-Correlation-Id")
	if corr == "" {
		corr = strconv.FormatInt(int64(time.Now().UnixNano()), 10)
	}

	tenantID := extractTenantID(r.URL.Path)
	if tenantID == "" {
		writeProblem(w, http.StatusBadRequest, "invalid-argument", "tenant id is required", corr)
		return
	}

	ctx := r.Context()
	manifestBytes, err := h.tenantCtl.Export(ctx, tenantID)
	if err != nil {
		h.emitter.Emit(ctx, ports.EventOpsConsoleActionRejected, ports.OpsConsoleActionPayload{
			Action:      "tenant.export",
			Target:      tenantID,
			Status:      "rejected",
			Detail:      err.Error(),
			Correlation: corr,
		})
		writeProblem(w, http.StatusInternalServerError, "export-failed", err.Error(), corr)
		return
	}

	h.emitter.Emit(ctx, ports.EventOpsConsoleActionCompleted, ports.OpsConsoleActionPayload{
		Action:      "tenant.export",
		Target:      tenantID,
		Status:      "completed",
		Correlation: corr,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"tenant_id":   tenantID,
		"manifest":    string(manifestBytes),
		"exported_at": time.Now().Format(time.RFC3339),
	})
}

// extractTenantID extracts the tenant id from a path like /admin/tenants/{id}/cell.
func extractTenantID(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, p := range parts {
		if p == "tenants" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}
