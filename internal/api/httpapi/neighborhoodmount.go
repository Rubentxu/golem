package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Rubentxu/golem/internal/ports"
)

// NeighborhoodMount provides the graph neighborhood query endpoint.
type NeighborhoodMount struct{}

func (NeighborhoodMount) Pattern() string { return "/api/v1/graph" }

// correlationOf returns the request correlation id.
func (m *NeighborhoodMount) correlationOf(r *http.Request) string {
	if c, ok := ports.CorrelationFrom(r.Context()); ok {
		return c.CorrelationID
	}
	return ""
}

// tenantActor extracts tenant and actor from request headers.
func (m *NeighborhoodMount) tenantActor(r *http.Request) (ports.TenantID, ports.Actor, bool) {
	return tenantActor(r)
}

// problem writes a problem response.
func (m *NeighborhoodMount) problem(w http.ResponseWriter, status int, code, detail, corr string) {
	title := http.StatusText(status)
	if title == "" {
		title, status = "Error", http.StatusInternalServerError
	}
	writeJSON(w, status, Problem{Type: "about:blank", Title: title, Status: status, Code: code, Detail: detail, CorrelationID: corr})
}

func (m *NeighborhoodMount) Mount(mux *http.ServeMux, deps MountDeps) error {
	if _, err := deps.RegisterRoute(mux, "POST", "/neighborhood", "/api/v1/graph", m.handleNeighborhood(deps)); err != nil {
		return err
	}
	return nil
}

func (m *NeighborhoodMount) handleNeighborhood(deps MountDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		corr := m.correlationOf(r)
		tenant, _, ok := m.tenantActor(r)
		if !ok {
			m.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
			return
		}
		var req NeighborhoodRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			m.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
			return
		}
		if len(req.Roots) == 0 {
			m.problem(w, http.StatusBadRequest, CodeUnboundedQuery, "roots must not be empty", corr)
			return
		}
		if req.MaxDepth <= 0 || req.MaxNodes <= 0 || req.MaxEdges <= 0 {
			m.problem(w, http.StatusBadRequest, CodeUnboundedQuery, "max_depth, max_nodes and max_edges are mandatory and must be positive", corr)
			return
		}
		sub, err := deps.GraphStore.Neighborhood(r.Context(), ports.NeighborhoodQuery{
			TenantID: tenant,
			Roots:    req.Roots,
			MaxDepth: req.MaxDepth,
			MaxNodes: req.MaxNodes,
			MaxEdges: req.MaxEdges,
		})
		if err != nil {
			if errors.Is(err, ports.ErrUnboundedQuery) {
				m.problem(w, http.StatusBadRequest, CodeUnboundedQuery, "query exceeds safety bounds", corr)
				return
			}
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
		writeJSON(w, http.StatusOK, out)
	}
}
