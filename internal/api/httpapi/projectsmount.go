package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Rubentxu/golem/internal/application/command"
	appprojects "github.com/Rubentxu/golem/internal/application/projects"
	"github.com/Rubentxu/golem/internal/ports"
)

// ProjectsMount provides project creation endpoint.
type ProjectsMount struct{}

func (ProjectsMount) Pattern() string { return "/api/v1/projects" }

func (m *ProjectsMount) Mount(mux *http.ServeMux, deps MountDeps) error {
	if _, err := deps.RegisterRoute(mux, "POST", "", "/api/v1/projects", m.handleCreateProject(deps)); err != nil {
		return err
	}
	return nil
}

func (m *ProjectsMount) handleCreateProject(deps MountDeps) http.HandlerFunc {
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
			Name string `json:"name"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			m.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
			return
		}
		receipt, err := deps.Bus.Submit(r.Context(), command.Command{
			Name:          appprojects.CmdCreateProject,
			TenantID:      tenant,
			Actor:         actor,
			CommandID:     idemKey,
			CorrelationID: corr,
			Payload:       appprojects.CreateProject{Name: body.Name},
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

func (m *ProjectsMount) correlationOf(r *http.Request) string {
	if c, ok := ports.CorrelationFrom(r.Context()); ok {
		return c.CorrelationID
	}
	return ""
}

func (m *ProjectsMount) tenantActor(r *http.Request) (ports.TenantID, ports.Actor, bool) {
	return tenantActor(r)
}

func (m *ProjectsMount) problem(w http.ResponseWriter, status int, code, detail, corr string) {
	title := http.StatusText(status)
	if title == "" {
		title, status = "Error", http.StatusInternalServerError
	}
	writeJSON(w, status, Problem{Type: "about:blank", Title: title, Status: status, Code: code, Detail: detail, CorrelationID: corr})
}

var _ HTTPMount = (*ProjectsMount)(nil)
