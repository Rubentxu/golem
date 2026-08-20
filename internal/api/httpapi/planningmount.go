package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Rubentxu/golem/internal/application/command"
	appplanning "github.com/Rubentxu/golem/internal/application/planning"
	"github.com/Rubentxu/golem/internal/ports"
)

// PlanningMount provides iteration, milestone, and board endpoints.
type PlanningMount struct{}

func (PlanningMount) Pattern() string { return "/api/v1/planning" }

func (m *PlanningMount) Mount(mux *http.ServeMux, deps MountDeps) error {
	if _, err := deps.RegisterRoute(mux, "POST", "/iterations", "/api/v1/planning", m.handleCreateIteration(deps)); err != nil {
		return err
	}
	if _, err := deps.RegisterRoute(mux, "POST", "/milestones", "/api/v1/planning", m.handleCreateMilestone(deps)); err != nil {
		return err
	}
	if _, err := deps.RegisterRoute(mux, "GET", "/iterations/{id}/board", "/api/v1/planning", m.handleIterationBoard(deps)); err != nil {
		return err
	}
	return nil
}

func (m *PlanningMount) handleCreateIteration(deps MountDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		corr := m.correlationOf(r)
		tenant, actor, ok := m.tenantActor(r)
		if !ok {
			m.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
			return
		}
		idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if len(idemKey) < 8 {
			m.problem(w, http.StatusBadRequest, CodeInvalidArgument, "Idempotency-Key header is required (min 8 chars)", corr)
			return
		}
		var body struct {
			Name  string `json:"name"`
			Start string `json:"start"`
			End   string `json:"end"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			m.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
			return
		}
		start, _ := time.Parse(time.RFC3339, body.Start)
		end, _ := time.Parse(time.RFC3339, body.End)
		receipt, err := deps.Bus.Submit(r.Context(), command.Command{
			Name:          appplanning.CmdCreateIteration,
			TenantID:      tenant,
			Actor:         actor,
			CommandID:     idemKey,
			CorrelationID: corr,
			Payload:       appplanning.CreateIteration{Name: body.Name, Start: start, End: end},
		})
		if err != nil {
			m.problem(w, http.StatusInternalServerError, CodeInternal, "command failed: "+err.Error(), corr)
			return
		}
		status := http.StatusAccepted
		if receipt.Duplicate {
			status = http.StatusOK
		}
		writeJSON(w, status, Receipt{
			CommandID: receipt.CommandID, EventIDs: receipt.EventIDs,
			Position: uint64(receipt.Position), Duplicate: receipt.Duplicate,
		})
	}
}

func (m *PlanningMount) handleCreateMilestone(deps MountDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		corr := m.correlationOf(r)
		tenant, actor, ok := m.tenantActor(r)
		if !ok {
			m.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
			return
		}
		idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if len(idemKey) < 8 {
			m.problem(w, http.StatusBadRequest, CodeInvalidArgument, "Idempotency-Key header is required (min 8 chars)", corr)
			return
		}
		var body struct {
			Name       string `json:"name"`
			TargetDate string `json:"target_date"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			m.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
			return
		}
		targetDate, _ := time.Parse(time.RFC3339, body.TargetDate)
		receipt, err := deps.Bus.Submit(r.Context(), command.Command{
			Name:          appplanning.CmdCreateMilestone,
			TenantID:      tenant,
			Actor:         actor,
			CommandID:     idemKey,
			CorrelationID: corr,
			Payload:       appplanning.CreateMilestone{Name: body.Name, TargetDate: targetDate},
		})
		if err != nil {
			m.problem(w, http.StatusInternalServerError, CodeInternal, "command failed: "+err.Error(), corr)
			return
		}
		status := http.StatusAccepted
		if receipt.Duplicate {
			status = http.StatusOK
		}
		writeJSON(w, status, Receipt{
			CommandID: receipt.CommandID, EventIDs: receipt.EventIDs,
			Position: uint64(receipt.Position), Duplicate: receipt.Duplicate,
		})
	}
}

func (m *PlanningMount) handleIterationBoard(deps MountDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		corr := m.correlationOf(r)
		tenant, _, ok := m.tenantActor(r)
		if !ok {
			m.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
			return
		}
		id := r.PathValue("id")
		// Board is a graph neighborhood query scoped to the iteration.
		sub, err := deps.GraphStore.Neighborhood(r.Context(), ports.NeighborhoodQuery{
			TenantID: tenant,
			Roots:   []string{id},
			MaxDepth: 3, MaxNodes: 100, MaxEdges: 200,
		})
		if err != nil {
			m.problem(w, http.StatusInternalServerError, CodeInternal, "graph query failed: "+err.Error(), corr)
			return
		}
		writeJSON(w, http.StatusOK, sub)
	}
}

func (m *PlanningMount) correlationOf(r *http.Request) string {
	if c, ok := ports.CorrelationFrom(r.Context()); ok {
		return c.CorrelationID
	}
	return ""
}

func (m *PlanningMount) tenantActor(r *http.Request) (ports.TenantID, ports.Actor, bool) {
	return tenantActor(r)
}

func (m *PlanningMount) problem(w http.ResponseWriter, status int, code, detail, corr string) {
	title := http.StatusText(status)
	if title == "" {
		title, status = "Error", http.StatusInternalServerError
	}
	writeJSON(w, status, Problem{Type: "about:blank", Title: title, Status: status, Code: code, Detail: detail, CorrelationID: corr})
}

var _ HTTPMount = (*PlanningMount)(nil)
