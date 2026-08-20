package httpapi

import (
	"net/http"

	"github.com/Rubentxu/golem/internal/application/command"
	appci "github.com/Rubentxu/golem/internal/application/ci"
	"github.com/Rubentxu/golem/internal/ports"
)

// CIMount provides CI build completion endpoint.
type CIMount struct{}

func (CIMount) Pattern() string { return "/api/v1/ci" }

func (m *CIMount) Mount(mux *http.ServeMux, deps MountDeps) error {
	if _, err := deps.RegisterRoute(mux, "POST", "/builds", "/api/v1/ci", m.handleCompleteBuild(deps)); err != nil {
		return err
	}
	return nil
}

func (m *CIMount) handleCompleteBuild(deps MountDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		corr := m.correlationOf(r)
		tenant, actor, ok := m.tenantActor(r)
		if !ok {
			m.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
			return
		}
		var body appci.CompleteBuild
		if err := decodeBody(w, r, &body); err != nil {
			m.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
			return
		}
		receipt, err := deps.Bus.Submit(r.Context(), command.Command{
			Name:          appci.CmdCompleteBuild,
			TenantID:      tenant,
			Actor:         actor,
			CommandID:     r.Header.Get("X-Idempotency-Key"),
			CorrelationID: corr,
			Payload:       body,
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

func (m *CIMount) correlationOf(r *http.Request) string {
	if c, ok := ports.CorrelationFrom(r.Context()); ok {
		return c.CorrelationID
	}
	return ""
}

func (m *CIMount) tenantActor(r *http.Request) (ports.TenantID, ports.Actor, bool) {
	return tenantActor(r)
}

func (m *CIMount) problem(w http.ResponseWriter, status int, code, detail, corr string) {
	title := http.StatusText(status)
	if title == "" {
		title, status = "Error", http.StatusInternalServerError
	}
	writeJSON(w, status, Problem{Type: "about:blank", Title: title, Status: status, Code: code, Detail: detail, CorrelationID: corr})
}

var _ HTTPMount = (*CIMount)(nil)
