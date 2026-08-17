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
	"sync"

	"github.com/Rubentxu/golem/internal/application/command"
	appwork "github.com/Rubentxu/golem/internal/application/work"
	"github.com/Rubentxu/golem/internal/clock"
	"github.com/Rubentxu/golem/internal/ids"
	"github.com/Rubentxu/golem/internal/obs"
	"github.com/Rubentxu/golem/internal/ports"
)

// Stable problem codes (public contract; never rename).
const (
	CodeInvalidArgument  = "golem/invalid-argument"
	CodeMissingTenant    = "golem/missing-tenant"
	CodeUnboundedQuery   = "golem/unbounded-query"
	CodeNotFound         = "golem/not-found"
	CodeRevisionConflict = "golem/revision-conflict"
	CodeDomainRejection  = "golem/domain-rejection"
	CodeInternal         = "golem/internal"
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

// GraphReader is the read-side dependency: tenant-scoped point reads and
// bounded neighborhood queries.
type GraphReader interface {
	Neighborhood(ctx context.Context, q ports.NeighborhoodQuery) (ports.Subgraph, error)
	GetNode(ctx context.Context, tenant ports.TenantID, nodeID string) (ports.Node, error)
}

// StreamVersionReader reads the authoritative version of a stream from
// the journal (ADR-021: optimistic concurrency is checked against the
// source of truth, not the eventual projection).
type StreamVersionReader interface {
	ReadStream(ctx context.Context, tenant ports.TenantID, streamID string, fromVersion uint64) ([]ports.RawEvent, error)
}

// Server builds the HTTP handler from the submitted dependencies.
type Server struct {
	commands CommandSubmitter
	graph    GraphReader
	streams  StreamVersionReader
	obs      ports.Observability

	idsOnce sync.Once
	idgen   ports.IDGenerator
}

// New creates the edge server. Observability is optional (zero value =
// no-ops).
func New(commands CommandSubmitter, graph GraphReader, streams StreamVersionReader) *Server {
	return &Server{commands: commands, graph: graph, streams: streams, obs: obs.Fill(ports.Observability{})}
}

// WithObservability sets the instrumentation bundle (chaining).
func (s *Server) WithObservability(o ports.Observability) *Server {
	s.obs = obs.Fill(o)
	return s
}

// Handler returns the routed handler wrapped with the observability
// middleware: correlation propagation (X-Correlation-Id, generated when
// absent), request spans and status metrics.
func (s *Server) Handler() http.Handler {
	return s.middleware(s.routes())
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/v1/work-items", s.handleCreateWorkItem)
	mux.HandleFunc("GET /api/v1/work-items/{id}", s.handleGetWorkItem)
	mux.HandleFunc("PATCH /api/v1/work-items/{id}", s.handleUpdateWorkItem)
	mux.HandleFunc("POST /api/v1/work-items/{id}/links", s.handleLinkWorkItem)
	mux.HandleFunc("POST /api/v1/requirements", s.handleCreateRequirement)
	mux.HandleFunc("GET /api/v1/requirements/{id}", s.handleGetRequirement)
	mux.HandleFunc("POST /api/v1/graph/neighborhood", s.handleNeighborhood)
	return mux
}

// middleware propagates correlation, traces each request and records
// http.server.request counters by method/route-template/status (tenant
// is deliberately absent from metric attributes: cardinality,
// OBSERVABILITY.md).
func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corr := strings.TrimSpace(r.Header.Get("X-Correlation-Id"))
		if corr == "" {
			corr = s.ids().NewID()
		}
		tenant := strings.TrimSpace(r.Header.Get("X-Golem-Tenant"))
		actorType, actorID := strings.TrimSpace(r.Header.Get("X-Golem-Actor-Type")), strings.TrimSpace(r.Header.Get("X-Golem-Actor-Id"))
		if actorType == "" || actorID == "" {
			actorType, actorID = "user", "anonymous"
		}

		r = r.WithContext(ports.WithCorrelation(r.Context(), ports.Correlation{
			CorrelationID: corr,
			TenantID:      tenant,
			ActorType:     actorType,
			ActorID:       actorID,
		}))
		// Handlers echo the correlation id in bodies and headers.
		w.Header().Set("X-Correlation-Id", corr)

		route := "unmatched"
		if match, pat := muxMatch(r); match {
			route = pat
		}
		ctx, span := s.obs.Tracer.Start(r.Context(), "golem.http.request",
			ports.A("http.method", r.Method), ports.A("http.route", route))
		defer span.End(nil)

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r.WithContext(ctx))

		s.obs.Meter.Counter("golem.http.requests").Add(ctx, 1,
			ports.A("method", r.Method), ports.A("route", route), ports.A("status", int64(rec.status)))
		if rec.status >= 500 {
			s.obs.Logger.Error(ctx, "request failed",
				ports.A("status", int64(rec.status)), ports.A("route", route))
		}
	})
}

// ids lazily builds an id generator for correlation fallbacks.
func (s *Server) ids() ports.IDGenerator {
	s.idsOnce.Do(func() {
		if s.idgen == nil {
			s.idgen = ids.NewGenerator(clock.SystemClock{})
		}
	})
	return s.idgen
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// muxMatch reports the ServeMux pattern the request would match. With
// Go 1.22+ net/http the mux does not expose pattern lookup, so the edge
// keeps its own route table.
func muxMatch(r *http.Request) (bool, string) {
	routes := []struct{ method, pattern string }{
		{http.MethodGet, "/healthz"},
		{http.MethodPost, "/api/v1/work-items"},
		{http.MethodGet, "/api/v1/work-items/{id}"},
		{http.MethodPatch, "/api/v1/work-items/{id}"},
		{http.MethodPost, "/api/v1/work-items/{id}/links"},
		{http.MethodPost, "/api/v1/requirements"},
		{http.MethodGet, "/api/v1/requirements/{id}"},
		{http.MethodPost, "/api/v1/graph/neighborhood"},
	}
	for _, rt := range routes {
		if r.Method == rt.method {
			if _, ok := routeMatches(rt.pattern, r.URL.Path); ok {
				return true, rt.pattern
			}
		}
	}
	return false, ""
}

// routeMatches checks a {param} pattern against a concrete path.
func routeMatches(pattern, path string) (map[string]string, bool) {
	pp := strings.Split(strings.Trim(pattern, "/"), "/")
	cp := strings.Split(strings.Trim(path, "/"), "/")
	if len(pp) != len(cp) {
		return nil, false
	}
	params := map[string]string{}
	for i := range pp {
		if strings.HasPrefix(pp[i], "{") && strings.HasSuffix(pp[i], "}") {
			params[pp[i][1:len(pp[i])-1]] = cp[i]
			continue
		}
		if pp[i] != cp[i] {
			return nil, false
		}
	}
	return params, true
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

// correlationOf returns the request correlation id: the ctx value set
// by the middleware (generated when the client sent none).
func (s *Server) correlationOf(r *http.Request) string {
	if c, ok := ports.CorrelationFrom(r.Context()); ok {
		return c.CorrelationID
	}
	return strings.TrimSpace(r.Header.Get("X-Correlation-Id"))
}

// decodeBody decodes a JSON request body with a 1 MiB guard.
func decodeBody(w http.ResponseWriter, r *http.Request, v any) error {
	return json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v)
}

// ---- handlers ----

type createWorkItemBody struct {
	Title string `json:"title"`
	Type  string `json:"type"`
}

func (s *Server) handleCreateWorkItem(w http.ResponseWriter, r *http.Request) {
	corr := s.correlationOf(r)

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
	corr := s.correlationOf(r)

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
