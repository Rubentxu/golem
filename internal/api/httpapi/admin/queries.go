// Package admin provides operator-only HTTP endpoints for cell and tenant
// management (ADR-081).
package admin

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// SLOTracker defines SLO evaluation operations.
type SLOTracker interface {
	Record(ctx context.Context, sloName string, value float64) error
	Evaluate(ctx context.Context) ([]ports.SLOViolation, error)
}

// UsageMeter defines metering query operations.
type UsageMeter interface {
	Record(ctx context.Context, event ports.MeteringEvent) error
	Rollup(ctx context.Context, hour time.Time) ([]ports.MeteringRollup, error)
	UptimeGauge(ctx context.Context, capability string) (float64, error)
	ErrorBudgetGauge(ctx context.Context, capability string) (float64, error)
}

// SLOQueryResponse is the response for GET /admin/slo/{sli}.
type SLOQueryResponse struct {
	SLIName        string  `json:"sli_name"`
	CurrentValue   float64 `json:"current_value"`
	Target         float64 `json:"target"`
	BudgetConsumed float64 `json:"budget_consumed"`
	BurnRate       float64 `json:"burn_rate"`
	WindowHours    int     `json:"window_hours"`
}

// QueriesHandler handles admin query endpoints for SLO and metering.
type QueriesHandler struct {
	sloTracker ports.SLOTracker
	meter      ports.UsageMeter
}

// NewQueriesHandler creates a QueriesHandler.
func NewQueriesHandler(st ports.SLOTracker, m ports.UsageMeter) *QueriesHandler {
	return &QueriesHandler{sloTracker: st, meter: m}
}

// HandleSLOQuery handles GET /admin/slo/{sli}.
func (h *QueriesHandler) HandleSLOQuery(w http.ResponseWriter, r *http.Request) {
	corr := r.Header.Get("X-Correlation-Id")
	if corr == "" {
		corr = strconv.FormatInt(int64(time.Now().UnixNano()), 10)
	}

	sliName := extractSLI(r.URL.Path)
	if sliName == "" {
		writeProblem(w, http.StatusBadRequest, "invalid-argument", "sli name is required", corr)
		return
	}

	ctx := r.Context()
	violations, err := h.sloTracker.Evaluate(ctx)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", "failed to evaluate SLOs: "+err.Error(), corr)
		return
	}

	// Find the specific SLO violation.
	for _, v := range violations {
		if v.SLOName == sliName {
			writeJSON(w, http.StatusOK, SLOQueryResponse{
				SLIName:        v.SLOName,
				BudgetConsumed: v.BudgetConsumed,
				BurnRate:       v.BurnRate,
			})
			return
		}
	}

	// No violation found — return budget status.
	budget, _ := h.meter.ErrorBudgetGauge(ctx, sliName)
	writeJSON(w, http.StatusOK, SLOQueryResponse{
		SLIName:        sliName,
		BudgetConsumed: budget,
		BurnRate:       0,
	})
}

// HandleMeteringQuery handles GET /admin/metering.
func (h *QueriesHandler) HandleMeteringQuery(w http.ResponseWriter, r *http.Request) {
	corr := r.Header.Get("X-Correlation-Id")
	if corr == "" {
		corr = strconv.FormatInt(int64(time.Now().UnixNano()), 10)
	}

	tenantID := r.URL.Query().Get("tenant")
	if tenantID == "" {
		writeProblem(w, http.StatusBadRequest, "invalid-argument", "tenant query parameter is required", corr)
		return
	}

	ctx := r.Context()

	// Query metering data for the last 24 hours.
	now := time.Now()
	hour := now.Truncate(time.Hour)
	var rollups []ports.MeteringRollup
	for i := 0; i < 24; i++ {
		ts := hour.Add(-time.Duration(i) * time.Hour)
		rls, err := h.meter.Rollup(ctx, ts)
		if err != nil {
			continue
		}
		for _, r := range rls {
			if r.TenantID == tenantID {
				rollups = append(rollups, r)
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"tenant_id": tenantID,
		"rollups":   rollups,
		"count":     len(rollups),
	})
}

// extractSLI extracts the SLI name from a path like /admin/slo/{sli}.
func extractSLI(path string) string {
	parts := splitPath(path)
	for i, p := range parts {
		if p == "slo" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// splitPath splits a URL path into segments.
func splitPath(path string) []string {
	parts := make([]string, 0)
	for _, p := range split(path, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func split(s, sep string) []string {
	if s == "" {
		return nil
	}
	result := make([]string, 0)
	start := 0
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}
