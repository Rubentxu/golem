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

	"github.com/Rubentxu/golem/internal/api/httpapi/admin"
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
	Traversal(ctx context.Context, q ports.TraversalQuery) (ports.Subgraph, error)
	GetNode(ctx context.Context, tenant ports.TenantID, nodeID string) (ports.Node, error)
}

// SearchReader runs tenant-scoped search queries (ADR-015 derived
// projection).
type SearchReader interface {
	Query(ctx context.Context, q ports.SearchQuery) (ports.SearchPage, error)
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
	graph    ports.GraphStore
	streams  StreamVersionReader
	journal  ports.JournalStore // underlying journal for JournalStreamReader creation in routesWithMounts
	search   SearchReader
	ingest   IngestService
	packs    PackRegistry
	obs      ports.Observability

	// MetricsHandler serves the /metrics endpoint (e.g., Prometheus scrape).
	// Set via WithMetrics before building the handler.
	MetricsHandler http.Handler

	// OperatorAuth enforces RBAC for admin endpoints.
	OperatorAuth *OperatorAuth

	// AuditLogger emits ops.console.action events for admin endpoints.
	AuditLogger AuditLogger

	// AdminHandlers provides the /admin/* endpoints (ADR-081).
	AdminHandlers *admin.AdminMux

	// mounts is the list of HTTPMounts to register (from WithMounts).
	mounts []HTTPMount

	// routeLabels maps registered route patterns to their labels (for middleware).
	routeLabels map[string]string

	idsOnce sync.Once
	idgen   ports.IDGenerator
}

// New creates the edge server. Observability is optional (zero value =
// no-ops).
func New(commands CommandSubmitter, graph ports.GraphStore, streams StreamVersionReader) *Server {
	return &Server{commands: commands, graph: graph, streams: streams, obs: obs.Fill(ports.Observability{})}
}

// WithSearch sets the search reader (enables GET /api/v1/search).
func (s *Server) WithSearch(r SearchReader) *Server {
	s.search = r
	return s
}

// WithIngest sets the provider event-sink service (enables
// POST /api/v1/ingest/{provider}).
func (s *Server) WithIngest(svc IngestService) *Server {
	s.ingest = svc
	return s
}

// WithObservability sets the instrumentation bundle (chaining).
func (s *Server) WithObservability(o ports.Observability) *Server {
	s.obs = obs.Fill(o)
	return s
}

// WithMetrics sets the Prometheus /metrics handler (REQ-OPS-001).
func (s *Server) WithMetrics(handler http.Handler) *Server {
	s.MetricsHandler = handler
	return s
}

// WithAuditLogger sets the audit logger for admin endpoints (REQ-OPS-003).
func (s *Server) WithAuditLogger(l AuditLogger) *Server {
	s.AuditLogger = l
	return s
}

// WithAdminHandlers sets the admin mux for /admin/* endpoints (ADR-081).
func (s *Server) WithAdminHandlers(h *admin.AdminMux) *Server {
	s.AdminHandlers = h
	return s
}

// WithMounts sets the HTTPMount list and returns the server for chaining.
// During T07/T08, both legacy routes() and mounts coexist; T09 removes legacy.
func (s *Server) WithMounts(mounts []HTTPMount) *Server {
	s.mounts = mounts
	return s
}

// Handler returns the routed handler wrapped with the observability
// middleware: correlation propagation (X-Correlation-Id, generated when
// absent), request spans and status metrics.
func (s *Server) Handler() http.Handler {
	// Use mount-based routing if mounts are set; otherwise fall back to legacy.
	if len(s.mounts) > 0 {
		return s.middleware(s.routesWithMounts())
	}
	return s.middleware(s.routes())
}

// routesWithMounts registers routes from all HTTPMounts and returns the mux.
// It also registers legacy routes not covered by any mount (requirements, search,
// projects, planning, scm, ci, ingest) so that tests using NewWithMounts can still
// access legacy endpoints. Routes already covered by mounts (work-items,
// work-types, test/runs, releases, trace, healthz, etc.) are NOT registered here
// to avoid conflicts with mount-based routes.
func (s *Server) routesWithMounts() http.Handler {
	mux := http.NewServeMux()
	s.routeLabels = make(map[string]string)

	// Build MountDeps with a pointer to s.routeLabels so RegisterRoute
	// can record labels for middleware.
	deps := MountDeps{
		Observability: s.obs,
		Bus:          s.commands,
		GraphNodeFetcher: ports.NewGraphNodeFetcherOverGraphStore(s.graph),
		routeLabels:  &s.routeLabels,
		// Other deps fields are nil for now; T10 wires them fully.
	}

	// If s.journal is available, wrap it as JournalStreamReader.
	if s.journal != nil {
		deps.JournalStreamReader = ports.NewJournalStreamReaderOverJournalStore(s.journal)
	}

	for _, m := range s.mounts {
		if err := m.Mount(mux, deps); err != nil {
			// At construction time we expect no registration errors; fail fast.
			panic("mount " + m.Pattern() + ": " + err.Error())
		}
	}

	// Register legacy routes not covered by any mount.
	// Routes covered by mounts: PlatformMount (healthz, readyz, status, metrics),
	// WorkMount (work-items, work-types), VerificationMount (test/runs, trace),
	// ReleaseMount (releases, blast-radius), AdminMount (admin/*).
	mux.HandleFunc("POST /api/v1/packs/activate", s.handleActivatePack)
	mux.HandleFunc("POST /api/v1/requirements", s.handleCreateRequirement)
	mux.HandleFunc("GET /api/v1/requirements/{id}", s.handleGetRequirement)
	mux.HandleFunc("POST /api/v1/graph/neighborhood", s.handleNeighborhood)
	mux.HandleFunc("GET /api/v1/search", s.handleSearch)
	mux.HandleFunc("POST /api/v1/projects", s.handleCreateProject)
	mux.HandleFunc("POST /api/v1/planning/iterations", s.handleCreateIteration)
	mux.HandleFunc("POST /api/v1/planning/milestones", s.handleCreateMilestone)
	mux.HandleFunc("GET /api/v1/planning/iterations/{id}/board", s.handleIterationBoard)
	mux.HandleFunc("POST /api/v1/scm/commits", s.handleObserveCommit)
	mux.HandleFunc("POST /api/v1/ci/builds", s.handleCompleteBuild)
	mux.HandleFunc("POST /api/v1/ingest/{provider}", s.handleIngest)

	return mux
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/packs/activate", s.handleActivatePack)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	// Deep readiness check (REQ-OPS-001)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	// System status (operator role required)
	if s.OperatorAuth != nil {
		mux.Handle("/status", s.OperatorAuth.RequireOperator(http.HandlerFunc(s.handleStatus)))
	} else {
		mux.HandleFunc("GET /status", s.handleStatus)
	}
	// Prometheus /metrics endpoint (REQ-OPS-001)
	if s.MetricsHandler != nil {
		mux.Handle("/metrics", s.MetricsHandler)
	}
	// Admin endpoints (operator-only, ADR-081).
	if s.AdminHandlers != nil {
		adminMux := s.AdminHandlers.Handler()
		if s.OperatorAuth != nil {
			mux.Handle("/admin/", s.OperatorAuth.RequireOperator(adminMux))
		} else {
			mux.Handle("/admin/", adminMux)
		}
	}
	mux.HandleFunc("POST /api/v1/work-items", s.handleCreateWorkItem)
	mux.HandleFunc("GET /api/v1/work-items/{id}", s.handleGetWorkItem)
	mux.HandleFunc("PATCH /api/v1/work-items/{id}", s.handleUpdateWorkItem)
	mux.HandleFunc("POST /api/v1/work-items/{id}/links", s.handleLinkWorkItem)
	mux.HandleFunc("POST /api/v1/requirements", s.handleCreateRequirement)
	mux.HandleFunc("GET /api/v1/requirements/{id}", s.handleGetRequirement)
	mux.HandleFunc("POST /api/v1/graph/neighborhood", s.handleNeighborhood)
	mux.HandleFunc("GET /api/v1/search", s.handleSearch)
	mux.HandleFunc("POST /api/v1/work-types", s.handleRegisterWorkType)
	mux.HandleFunc("GET /api/v1/work-types/{name}", s.handleGetWorkType)
	mux.HandleFunc("POST /api/v1/projects", s.handleCreateProject)
	mux.HandleFunc("POST /api/v1/planning/iterations", s.handleCreateIteration)
	mux.HandleFunc("POST /api/v1/planning/milestones", s.handleCreateMilestone)
	mux.HandleFunc("GET /api/v1/planning/iterations/{id}/board", s.handleIterationBoard)
	mux.HandleFunc("POST /api/v1/work-items/{id}/comments", s.handleAddComment)
	mux.HandleFunc("GET /api/v1/work-items/{id}/events", s.handleItemEvents)
	mux.HandleFunc("POST /api/v1/scm/commits", s.handleObserveCommit)
	mux.HandleFunc("POST /api/v1/ci/builds", s.handleCompleteBuild)
	mux.HandleFunc("POST /api/v1/test/runs", s.handleReportTestRun)
	mux.HandleFunc("GET /api/v1/trace/{id}", s.handleTrace)
	mux.HandleFunc("POST /api/v1/ingest/{provider}", s.handleIngest)
	mux.HandleFunc("POST /api/v1/releases", s.handleCreateRelease)
	mux.HandleFunc("POST /api/v1/releases/{id}/gate", s.handleEvaluateGate)
	mux.HandleFunc("GET /api/v1/releases/{id}", s.handleGetRelease)
	mux.HandleFunc("GET /api/v1/components/{purl}/blast-radius", s.handleBlastRadius)
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

		route := s.routeLabel(r)
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

// routeLabel returns the route label for a request by looking up
// s.routeLabels (populated by mount-based registration).
func (s *Server) routeLabel(r *http.Request) string {
	if s.routeLabels == nil {
		return "unmatched"
	}
	// Linear search through registered patterns.
	for pattern := range s.routeLabels {
		if _, ok := matchPattern(pattern, r.URL.Path); ok {
			return pattern
		}
	}
	return "unmatched"
}

// matchPattern checks a {param} pattern against a concrete path.
func matchPattern(pattern, path string) (map[string]string, bool) {
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
	Title    string         `json:"title"`
	Type     string         `json:"type"`
	TypeName string         `json:"type_name,omitempty"`
	Fields   map[string]any `json:"fields,omitempty"`
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
		Payload:       appwork.CreateWorkItem{Title: body.Title, ItemType: body.Type, TypeName: body.TypeName, Fields: body.Fields},
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
