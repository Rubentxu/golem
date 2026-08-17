package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	appci "github.com/Rubentxu/golem/internal/application/ci"
	appscm "github.com/Rubentxu/golem/internal/application/scm"
	appver "github.com/Rubentxu/golem/internal/application/verification"
	"github.com/Rubentxu/golem/internal/ports"
)

// ---- POST /api/v1/scm/commits ----

func (s *Server) handleObserveCommit(w http.ResponseWriter, r *http.Request) {
	var body appscm.ObserveCommit
	if err := decodeBody(w, r, &body); err != nil {
		corr := s.correlationOf(r)
		s.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
		return
	}
	s.submitCommand(w, r, appscm.CmdObserveCommit, body)
}

// ---- POST /api/v1/ci/builds ----

func (s *Server) handleCompleteBuild(w http.ResponseWriter, r *http.Request) {
	var body appci.CompleteBuild
	if err := decodeBody(w, r, &body); err != nil {
		corr := s.correlationOf(r)
		s.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
		return
	}
	s.submitCommand(w, r, appci.CmdCompleteBuild, body)
}

// ---- POST /api/v1/test/runs ----

func (s *Server) handleReportTestRun(w http.ResponseWriter, r *http.Request) {
	var body appver.ReportTestRun
	if err := decodeBody(w, r, &body); err != nil {
		corr := s.correlationOf(r)
		s.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
		return
	}
	s.submitCommand(w, r, appver.CmdReportTestRun, body)
}

// ---- GET /api/v1/trace/{id} (openapi: traceEntity) ----
//
// A bounded, bidirectional lineage walk from any node: the causal trace
// explorer of the M3 exit criterion (Requirement→Commit→Build→Artifact
// →Test). depth defaults to 5, caps at 10; node/edge budgets fixed.

func (s *Server) handleTrace(w http.ResponseWriter, r *http.Request) {
	corr := s.correlationOf(r)
	tenant, _, ok := tenantActor(r)
	if !ok {
		s.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
		return
	}
	id := r.PathValue("id")

	depth := 5
	if v := r.URL.Query().Get("depth"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 || n > 10 {
			s.problem(w, http.StatusBadRequest, CodeInvalidArgument, "depth must be an integer in (0, 10]", corr)
			return
		}
		depth = n
	}

	if _, err := s.graph.GetNode(r.Context(), tenant, id); err != nil {
		if errors.Is(err, ports.ErrNodeNotFound) {
			s.problem(w, http.StatusNotFound, CodeNotFound, "entity not found", corr)
			return
		}
		s.problem(w, http.StatusInternalServerError, CodeInternal, "graph query failed", corr)
		return
	}

	sub, err := s.graph.Neighborhood(r.Context(), ports.NeighborhoodQuery{
		TenantID: tenant, Roots: []string{id}, MaxDepth: depth, MaxNodes: 500, MaxEdges: 1000,
	})
	if err != nil {
		s.problem(w, http.StatusInternalServerError, CodeInternal, "graph query failed", corr)
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
