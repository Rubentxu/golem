package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/Rubentxu/golem/internal/application/command"
	apprelease "github.com/Rubentxu/golem/internal/application/release"
	"github.com/Rubentxu/golem/internal/application/supplychain"
	"github.com/Rubentxu/golem/internal/ports"
)

// ReleaseMount provides release and supply-chain gate endpoints.
type ReleaseMount struct{}

// NewReleaseMount creates a ReleaseMount.
func NewReleaseMount() *ReleaseMount { return &ReleaseMount{} }

func (m *ReleaseMount) Pattern() string { return "/api/v1/releases" }

func (m *ReleaseMount) Mount(mux *http.ServeMux, deps MountDeps) error {
	if _, err := deps.RegisterRoute(mux, "POST", "", "/api/v1/releases", m.handleCreateRelease(deps)); err != nil {
		return err
	}
	if _, err := deps.RegisterRoute(mux, "GET", "/{id}", "/api/v1/releases", m.handleGetRelease(deps)); err != nil {
		return err
	}
	if _, err := deps.RegisterRoute(mux, "POST", "/{id}/gate", "/api/v1/releases", m.handleEvaluateGate(deps)); err != nil {
		return err
	}
	if _, err := deps.RegisterRoute(mux, "GET", "/{purl}/blast-radius", "/api/v1/components", m.handleBlastRadius(deps)); err != nil {
		return err
	}
	return nil
}

func (m *ReleaseMount) handleCreateRelease(deps MountDeps) http.HandlerFunc {
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
			Name      string   `json:"name"`
			Artifacts []string `json:"artifacts"`
		}
		if err := decodeBody(w, r, &body); err != nil {
			m.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
			return
		}
		receipt, err := deps.Bus.Submit(r.Context(), command.Command{
			Name:          apprelease.CmdCreateCandidate,
			TenantID:      tenant,
			Actor:         actor,
			CommandID:     idemKey,
			CorrelationID: corr,
			Payload:       apprelease.CreateCandidate{Name: body.Name, Artifacts: body.Artifacts},
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

func (m *ReleaseMount) handleGetRelease(deps MountDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		corr := m.correlationOf(r)
		tenant, _, ok := m.tenantActor(r)
		if !ok {
			m.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
			return
		}
		n, err := deps.GraphNodeFetcher.GetNode(r.Context(), tenant, r.PathValue("id"))
		if err != nil {
			if errors.Is(err, ports.ErrNodeNotFound) {
				m.problem(w, http.StatusNotFound, CodeNotFound, "release not found", corr)
				return
			}
			m.problem(w, http.StatusInternalServerError, CodeInternal, "graph query failed", corr)
			return
		}
		if n.Kind != "Release" {
			m.problem(w, http.StatusNotFound, CodeNotFound, "release not found", corr)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": n.ID, "kind": n.Kind, "attributes": n.Attributes})
	}
}

func (m *ReleaseMount) handleEvaluateGate(deps MountDeps) http.HandlerFunc {
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
		receipt, err := deps.Bus.Submit(r.Context(), command.Command{
			Name:          apprelease.CmdEvaluateGate,
			TenantID:      tenant,
			Actor:         actor,
			CommandID:     idemKey,
			CorrelationID: corr,
			Payload:       apprelease.EvaluateGate{ReleaseID: r.PathValue("id")},
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

func (m *ReleaseMount) handleBlastRadius(deps MountDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		corr := m.correlationOf(r)
		tenant, _, ok := m.tenantActor(r)
		if !ok {
			m.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
			return
		}
		purl := r.PathValue("purl")
		if purl == "" {
			m.problem(w, http.StatusBadRequest, CodeInvalidArgument, "purl path parameter is mandatory", corr)
			return
		}
		decoded, err := url.PathUnescape(purl)
		if err != nil {
			m.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid purl encoding", corr)
			return
		}
		result, err := supplychain.BlastRadius(r.Context(), deps.GraphStore, tenant, decoded)
		if err != nil {
			if errors.Is(err, supplychain.ErrInvalidPurlForBlast) {
				m.problem(w, http.StatusUnprocessableEntity, CodeDomainRejection, "unknown component: "+url.QueryEscape(decoded), corr)
				return
			}
			m.problem(w, http.StatusInternalServerError, CodeInternal, "blast radius query failed", corr)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func (m *ReleaseMount) tenantActor(r *http.Request) (ports.TenantID, ports.Actor, bool) {
	tenant := strings.TrimSpace(r.Header.Get("X-Golem-Tenant"))
	if tenant == "" {
		return "", ports.Actor{}, false
	}
	actorType := strings.TrimSpace(r.Header.Get("X-Golem-Actor-Type"))
	actorID := strings.TrimSpace(r.Header.Get("X-Golem-Actor-Id"))
	if actorType == "" || actorID == "" {
		actorType, actorID = "user", "anonymous"
	}
	return ports.TenantID(tenant), ports.Actor{Type: actorType, ID: actorID}, true
}

func (m *ReleaseMount) correlationOf(r *http.Request) string {
	if c, ok := ports.CorrelationFrom(r.Context()); ok {
		return c.CorrelationID
	}
	return strings.TrimSpace(r.Header.Get("X-Correlation-Id"))
}

func (m *ReleaseMount) problem(w http.ResponseWriter, status int, code, detail, corr string) {
	title := http.StatusText(status)
	if title == "" {
		title, status = "Error", http.StatusInternalServerError
	}
	writeJSON(w, status, Problem{Type: "about:blank", Title: title, Status: status, Code: code, Detail: detail, CorrelationID: corr})
}

func (m *ReleaseMount) writeCommandError(w http.ResponseWriter, err error, corr string) {
	switch {
	case errors.Is(err, apprelease.ErrEmptyName), errors.Is(err, apprelease.ErrNoArtifacts):
		m.problem(w, http.StatusUnprocessableEntity, CodeDomainRejection, err.Error(), corr)
	case errors.Is(err, apprelease.ErrUnknownArtifact), errors.Is(err, apprelease.ErrReleaseNotFound):
		m.problem(w, http.StatusNotFound, CodeNotFound, err.Error(), corr)
	default:
		m.problem(w, http.StatusInternalServerError, CodeInternal, "command failed", corr)
	}
}

var _ HTTPMount = (*ReleaseMount)(nil)
