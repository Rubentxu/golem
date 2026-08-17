// Package httpapi is the API Edge of GOLEM: the HTTP/JSON driving adapter
// of the application layer (API_GUIDELINES: resource-oriented,
// OpenAPI-first, stable opaque IDs; errors as Problem Details with stable
// codes and correlation ids).
//
// Slice scope (IMPLEMENTATION_SEQUENCE weeks 7–8):
//
//	POST /api/v1/work-items          — command, 202 + receipt
//	POST /api/v1/graph/neighborhood  — bounded graph query
//	GET  /healthz
//
// Tenancy: the mandatory X-Golem-Tenant header populates TenantContext
// (ADR-008). Actor headers default to an anonymous user placeholder until
// the OIDC identity boundary lands (ADR-017); commands stay attributed.
//
// Consistency: reads hit the graph projection, which the runtime tails
// asynchronously — read-your-write is available via the receipt's journal
// position, and clients may poll until the projection catches up.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Rubentxu/golem/internal/application/command"
	appwork "github.com/Rubentxu/golem/internal/application/work"
	"github.com/Rubentxu/golem/internal/ports"
)

// Stable problem codes (public contract; never rename).
const (
	CodeInvalidArgument = "golem/invalid-argument"
	CodeMissingTenant   = "golem/missing-tenant"
	CodeUnboundedQuery  = "golem/unbounded-query"
	CodeNotFound        = "golem/not-found"
	CodeDomainRejection = "golem/domain-rejection"
	CodeInternal        = "golem/internal"
)

// Problem is the RFC 7807-style error body (API_GUIDELINES: stable code
// and correlation_id).
type Problem struct {
	Type          string `json:"type"`
	Title         string `json:"title"`
	Status        int    `json:"status"`
	Code          string `json:"code"`
	Detail        string `json:"detail,omitempty"`
	CorrelationID string `json:"correlation_id"`
}

// Receipt is the command acceptance body (202).
type Receipt struct {
	CommandID string   `json:"command_id"`
	EventIDs  []string `json:"event_ids"`
	Position  uint64   `json:"position"`
	Duplicate bool     `json:"duplicate"`
}

// NeighborhoodRequest is the bounded traversal body. Every limit is
// mandatory and positive (GRAPH_MODEL query safety).
type NeighborhoodRequest struct {
	Roots    []string `json:"roots"`
	MaxDepth int      `json:"max_depth"`
	MaxNodes int      `json:"max_nodes"`
	MaxEdges int      `json:"max_edges"`
}

// Node/Edge DTOs of the graph read path (stable DTOs, not internal types).
type Node struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	Revision   uint64         `json:"revision"`
	Attributes map[string]any `json:"attributes"`
}

type Edge struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	SourceID   string         `json:"source_id"`
	TargetID   string         `json:"target_id"`
	Revision   uint64         `json:"revision"`
	Attributes map[string]any `json:"attributes"`
}

type Subgraph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// CommandSubmitter decouples the edge from runtime.Runtime: any command
// dispatcher satisfying this signature can serve HTTP (tests use the real
// bus; nothing else is exposed).
type CommandSubmitter interface {
	Submit(ctx context.Context, cmd command.Command) (ports.CommandReceipt, error)
}

// GraphReader is the read-side dependency: a tenant-scoped bounded
// neighborhood query.
type GraphReader interface {
	Neighborhood(ctx context.Context, q ports.NeighborhoodQuery) (ports.Subgraph, error)
}

// Server builds the HTTP handler from the submitted dependencies.
type Server struct {
	commands CommandSubmitter
	graph    GraphReader
}

// New creates the edge server.
func New(commands CommandSubmitter, graph GraphReader) *Server {
	return &Server{commands: commands, graph: graph}
}

// Handler returns the routed handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/v1/work-items", s.handleCreateWorkItem)
	mux.HandleFunc("POST /api/v1/graph/neighborhood", s.handleNeighborhood)
	return mux
}

// ---- helpers ----

func (s *Server) problem(w http.ResponseWriter, status int, code, detail string, corr string) {
	title := http.StatusText(status)
	if title == "" {
		title, status = "Error", http.StatusInternalServerError
	}
	writeJSON(w, status, Problem{
		Type: "about:blank", Title: title, Status: status,
		Code: code, Detail: detail, CorrelationID: corr,
	})
}

// tenantActor extracts the mandatory tenant and attributed actor from
// headers. Defaults keep the slice attributed; OIDC replaces them later.
func tenantActor(r *http.Request) (ports.TenantID, ports.Actor, bool) {
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Header already sent; nothing else to do but log-worthy.
		_ = err
	}
}

// ---- handlers ----

type createWorkItemBody struct {
	Title string `json:"title"`
	Type  string `json:"type"`
}

func (s *Server) handleCreateWorkItem(w http.ResponseWriter, r *http.Request) {
	corr := r.Header.Get("X-Correlation-Id")

	tenant, actor, ok := tenantActor(r)
	if !ok {
		s.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
		return
	}

	idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(idemKey) < 8 {
		s.problem(w, http.StatusBadRequest, CodeInvalidArgument, "Idempotency-Key header is required (min 8 chars)", corr)
		return
	}

	var body createWorkItemBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		s.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
		return
	}

	receipt, err := s.commands.Submit(r.Context(), command.Command{
		Name:          appwork.CmdCreateWorkItem,
		TenantID:      tenant,
		Actor:         actor,
		CommandID:     idemKey,
		CorrelationID: corr,
		Payload:       appwork.CreateWorkItem{Title: body.Title, ItemType: body.Type},
	})
	if err != nil {
		s.writeCommandError(w, err, corr)
		return
	}

	status := http.StatusAccepted
	if receipt.Duplicate {
		status = http.StatusOK
	}
	writeJSON(w, status, Receipt{
		CommandID: receipt.CommandID,
		EventIDs:  receipt.EventIDs,
		Position:  uint64(receipt.Position),
		Duplicate: receipt.Duplicate,
	})
}

func (s *Server) handleNeighborhood(w http.ResponseWriter, r *http.Request) {
	corr := r.Header.Get("X-Correlation-Id")

	tenant, _, ok := tenantActor(r)
	if !ok {
		s.problem(w, http.StatusBadRequest, CodeMissingTenant, "X-Golem-Tenant header is mandatory", corr)
		return
	}

	var req NeighborhoodRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		s.problem(w, http.StatusBadRequest, CodeInvalidArgument, "invalid JSON body: "+err.Error(), corr)
		return
	}
	if len(req.Roots) == 0 {
		s.problem(w, http.StatusBadRequest, CodeUnboundedQuery, "roots must not be empty", corr)
		return
	}
	if req.MaxDepth <= 0 || req.MaxNodes <= 0 || req.MaxEdges <= 0 {
		s.problem(w, http.StatusBadRequest, CodeUnboundedQuery, "max_depth, max_nodes and max_edges are mandatory and must be positive", corr)
		return
	}

	sub, err := s.graph.Neighborhood(r.Context(), ports.NeighborhoodQuery{
		TenantID: tenant,
		Roots:    req.Roots,
		MaxDepth: req.MaxDepth,
		MaxNodes: req.MaxNodes,
		MaxEdges: req.MaxEdges,
	})
	if err != nil {
		if errors.Is(err, ports.ErrUnboundedQuery) {
			s.problem(w, http.StatusBadRequest, CodeUnboundedQuery, "query exceeds safety bounds", corr)
			return
		}
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
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) writeCommandError(w http.ResponseWriter, err error, corr string) {
	switch {
	case errors.Is(err, appwork.ErrEmptyTitle), errors.Is(err, appwork.ErrEmptyType):
		s.problem(w, http.StatusUnprocessableEntity, CodeDomainRejection, err.Error(), corr)
	case errors.Is(err, ports.ErrEmptyTenant), errors.Is(err, ports.ErrEmptyActor):
		s.problem(w, http.StatusBadRequest, CodeInvalidArgument, err.Error(), corr)
	default:
		s.problem(w, http.StatusInternalServerError, CodeInternal, "command failed", corr)
	}
}
