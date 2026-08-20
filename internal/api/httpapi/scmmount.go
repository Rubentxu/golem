package httpapi

import (
	"net/http"

	"github.com/Rubentxu/golem/internal/application/command"
	appscm "github.com/Rubentxu/golem/internal/application/scm"
	"github.com/Rubentxu/golem/internal/ports"
)

// SCMMount provides SCM commit observation endpoint.
type SCMMount struct{}

func (SCMMount) Pattern() string { return "/api/v1/scm" }

func (m *SCMMount) Mount(mux *http.ServeMux, deps MountDeps) error {
	if _, err := deps.RegisterRoute(mux, "POST", "/commits", "/api/v1/scm", m.handleObserveCommit(deps)); err != nil {
		return err
	}
	return nil
}

func (m *SCMMount) handleObserveCommit(deps MountDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		corr := m.correlationOf(r)
		tenant, actor, ok := m.tenantActor(r)
		if !ok {
			m.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
			return
		}
		var body appscm.ObserveCommit
		if err := decodeBody(w, r, &body); err != nil {
			m.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
			return
		}
		receipt, err := deps.Bus.Submit(r.Context(), command.Command{
			Name:          appscm.CmdObserveCommit,
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

func (m *SCMMount) correlationOf(r *http.Request) string {
	if c, ok := ports.CorrelationFrom(r.Context()); ok {
		return c.CorrelationID
	}
	return ""
}

func (m *SCMMount) tenantActor(r *http.Request) (ports.TenantID, ports.Actor, bool) {
	return tenantActor(r)
}

func (m *SCMMount) problem(w http.ResponseWriter, status int, code, detail, corr string) {
	title := http.StatusText(status)
	if title == "" {
		title, status = "Error", http.StatusInternalServerError
	}
	writeJSON(w, status, Problem{Type: "about:blank", Title: title, Status: status, Code: code, Detail: detail, CorrelationID: corr})
}

var _ HTTPMount = (*SCMMount)(nil)
