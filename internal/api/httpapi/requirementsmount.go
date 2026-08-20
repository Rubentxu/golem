package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Rubentxu/golem/internal/application/command"
	appreq "github.com/Rubentxu/golem/internal/application/requirements"
	"github.com/Rubentxu/golem/internal/ports"
)

// RequirementsMount provides requirement create and get endpoints.
type RequirementsMount struct{}

func (RequirementsMount) Pattern() string { return "/api/v1/requirements" }

func (m *RequirementsMount) Mount(mux *http.ServeMux, deps MountDeps) error {
	if _, err := deps.RegisterRoute(mux, "POST", "", "/api/v1/requirements", m.handleCreateRequirement(deps)); err != nil {
		return err
	}
	if _, err := deps.RegisterRoute(mux, "GET", "/{id}", "/api/v1/requirements", m.handleGetRequirement(deps)); err != nil {
		return err
	}
	return nil
}

func (m *RequirementsMount) handleCreateRequirement(deps MountDeps) http.HandlerFunc {
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
		var body createRequirementBody
		if err := decodeBody(w, r, &body); err != nil {
			m.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
			return
		}
		receipt, err := deps.Bus.Submit(r.Context(), command.Command{
			Name:          appreq.CmdCreateRequirement,
			TenantID:      tenant,
			Actor:         actor,
			CommandID:     idemKey,
			CorrelationID: corr,
			Payload:       appreq.CreateRequirement{Title: body.Title, Statement: body.Statement},
		})
		if err != nil {
			m.writeCommandError(w, err, corr)
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

func (m *RequirementsMount) handleGetRequirement(deps MountDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		corr := m.correlationOf(r)
		tenant, _, ok := m.tenantActor(r)
		if !ok {
			m.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
			return
		}
		id := r.PathValue("id")
		node, err := deps.GraphStore.GetNode(r.Context(), tenant, id)
		if err != nil {
			if errors.Is(err, ports.ErrNodeNotFound) {
				m.problem(w, http.StatusNotFound, CodeNotFound, "entity not found", corr)
				return
			}
			m.problem(w, http.StatusInternalServerError, CodeInternal, "graph query failed", corr)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id":       node.ID,
			"kind":     node.Kind,
			"revision":  node.Revision,
			"attrs":    node.Attributes,
		})
	}
}

func (m *RequirementsMount) correlationOf(r *http.Request) string {
	if c, ok := ports.CorrelationFrom(r.Context()); ok {
		return c.CorrelationID
	}
	return ""
}

func (m *RequirementsMount) tenantActor(r *http.Request) (ports.TenantID, ports.Actor, bool) {
	return tenantActor(r)
}

func (m *RequirementsMount) problem(w http.ResponseWriter, status int, code, detail, corr string) {
	title := http.StatusText(status)
	if title == "" {
		title, status = "Error", http.StatusInternalServerError
	}
	writeJSON(w, status, Problem{Type: "about:blank", Title: title, Status: status, Code: code, Detail: detail, CorrelationID: corr})
}

func (m *RequirementsMount) writeCommandError(w http.ResponseWriter, err error, corr string) {
	switch {
	case errors.Is(err, appreq.ErrEmptyTitle):
		m.problem(w, http.StatusUnprocessableEntity, CodeDomainRejection, err.Error(), corr)
	default:
		m.problem(w, http.StatusInternalServerError, CodeInternal, "command failed", corr)
	}
}

var _ HTTPMount = (*RequirementsMount)(nil)
