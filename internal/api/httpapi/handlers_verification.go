package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Rubentxu/golem/internal/application/command"
	appver "github.com/Rubentxu/golem/internal/application/verification"
	"github.com/Rubentxu/golem/internal/ports"
)

// VerificationMount provides verification test-run and lineage trace endpoints.
type VerificationMount struct{}

// NewVerificationMount creates a VerificationMount.
func NewVerificationMount() *VerificationMount { return &VerificationMount{} }

func (m *VerificationMount) Pattern() string              { return "/api/v1/test" }
func (m *VerificationMount) AdditionalPatterns() []string { return []string{"/api/v1/trace"} }

func (m *VerificationMount) Mount(mux *http.ServeMux, deps MountDeps) error {
	if _, err := deps.RegisterRoute(mux, "POST", "/runs", "/api/v1/test", m.handleReportTestRun(deps)); err != nil {
		return err
	}
	if _, err := deps.RegisterRoute(mux, "GET", "/{id}", "/api/v1/trace", m.handleTrace(deps)); err != nil {
		return err
	}
	return nil
}

func (m *VerificationMount) handleReportTestRun(deps MountDeps) http.HandlerFunc {
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
			Tenant       string `json:"tenant"`
			Target       string `json:"target"`
			Case         string `json:"case"`
			Status       string `json:"status"`
			ArtifactPURL string `json:"artifact_purl,omitempty"`
		}
		if err := decodeBody(w, r, &body); err != nil {
			m.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
			return
		}
		receipt, err := deps.Bus.Submit(r.Context(), command.Command{
			Name:          appver.CmdReportTestRun,
			TenantID:      tenant,
			Actor:         actor,
			CommandID:     idemKey,
			CorrelationID: corr,
			Payload:       body,
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

func (m *VerificationMount) handleTrace(deps MountDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		corr := m.correlationOf(r)
		tenant, _, ok := m.tenantActor(r)
		if !ok {
			m.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
			return
		}
		id := r.PathValue("id")
		depth := 5
		if v := r.URL.Query().Get("depth"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 || n > 10 {
				m.problem(w, http.StatusBadRequest, CodeInvalidArgument, "depth must be an integer in (0, 10]", corr)
				return
			}
			depth = n
		}
		if _, err := deps.GraphNodeFetcher.GetNode(r.Context(), tenant, id); err != nil {
			if errors.Is(err, ports.ErrNodeNotFound) {
				m.problem(w, http.StatusNotFound, CodeNotFound, "entity not found", corr)
				return
			}
			m.problem(w, http.StatusInternalServerError, CodeInternal, "graph query failed", corr)
			return
		}
		sub, err := deps.GraphStore.Neighborhood(r.Context(), ports.NeighborhoodQuery{
			TenantID: tenant, Roots: []string{id}, MaxDepth: depth, MaxNodes: 500, MaxEdges: 1000,
		})
		if err != nil {
			m.problem(w, http.StatusInternalServerError, CodeInternal, "graph query failed", corr)
			return
		}
		out := Subgraph{Nodes: make([]Node, 0, len(sub.Nodes)), Edges: make([]Edge, 0, len(sub.Edges))}
		for _, n := range sub.Nodes {
			out.Nodes = append(out.Nodes, Node{ID: n.ID, Kind: n.Kind, Revision: uint64(n.Revision), Attributes: n.Attributes})
		}
		for _, e := range sub.Edges {
			out.Edges = append(out.Edges, Edge{ID: e.ID, Type: e.Type, SourceID: e.SourceID, TargetID: e.TargetID, Revision: uint64(e.Revision), Attributes: e.Attributes})
		}
		writeJSON(w, http.StatusOK, map[string]any{"root": id, "depth": depth, "subgraph": out})
	}
}

func (m *VerificationMount) tenantActor(r *http.Request) (ports.TenantID, ports.Actor, bool) {
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

func (m *VerificationMount) correlationOf(r *http.Request) string {
	if c, ok := ports.CorrelationFrom(r.Context()); ok {
		return c.CorrelationID
	}
	return strings.TrimSpace(r.Header.Get("X-Correlation-Id"))
}

func (m *VerificationMount) problem(w http.ResponseWriter, status int, code, detail, corr string) {
	title := http.StatusText(status)
	if title == "" {
		title, status = "Error", http.StatusInternalServerError
	}
	writeJSON(w, status, Problem{Type: "about:blank", Title: title, Status: status, Code: code, Detail: detail, CorrelationID: corr})
}

func (m *VerificationMount) writeCommandError(w http.ResponseWriter, err error, corr string) {
	switch {
	case errors.Is(err, appver.ErrEmptyCase), errors.Is(err, appver.ErrInvalidRunStatus):
		m.problem(w, http.StatusUnprocessableEntity, CodeDomainRejection, err.Error(), corr)
	case errors.Is(err, appver.ErrUnknownTarget):
		m.problem(w, http.StatusNotFound, CodeNotFound, err.Error(), corr)
	default:
		m.problem(w, http.StatusInternalServerError, CodeInternal, "command failed", corr)
	}
}

var _ HTTPMount = (*VerificationMount)(nil)
var _ MultiMount = (*VerificationMount)(nil)
