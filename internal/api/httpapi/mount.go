// Package httpapi is the API Edge of GOLEM.
package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/Rubentxu/golem/internal/obs"
	"github.com/Rubentxu/golem/internal/ports"
)

// HTTPMount is the interface for HTTP mountable components. Each bounded
// context implements this interface to register its routes on a shared
// *http.ServeMux. The mount system replaces the legacy Server.routes().
type HTTPMount interface {
	// Pattern returns the primary URL prefix for this mount (e.g., "/api/v1/work-items").
	Pattern() string
	// Mount registers the mount's routes on mux using deps. RegisterRoute
	// is the mandatory helper for route registration (not mux.HandleFunc directly).
	Mount(mux *http.ServeMux, deps MountDeps) error
}

// MultiMount extends HTTPMount for contexts that serve multiple URL prefixes
// from the same handler set (e.g., work-items + work-types).
type MultiMount interface {
	HTTPMount
	// AdditionalPatterns returns additional URL prefixes beyond the primary Pattern().
	// Each additional pattern is registered under the same Mount implementation.
	AdditionalPatterns() []string
}

// ErrPatternOverlap is returned by RegisterRoute when a route pattern
// overlaps with an already-registered pattern (segment-by-segment comparison,
// {param} wildcards allowed for sibling routes with shared prefix).
var ErrPatternOverlap = fmt.Errorf("httpapi: route pattern overlaps an existing registration")

// ParseETagVersion parses an RFC 7232 If-Match header value into a uint64 version.
// It accepts strong validators ("3") and weak validators (W/"3"), stripping
// the W/ prefix and surrounding quotes before parsing the integer.
func ParseETagVersion(ifMatch string) (uint64, error) {
	v := strings.TrimSpace(ifMatch)
	v = strings.TrimPrefix(v, `W/`)
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		v = v[1 : len(v)-1]
	}
	return strconv.ParseUint(v, 10, 64)
}

// registrationState holds the concurrency-safe bookkeeping for route registration.
type registrationState struct {
	mu          sync.Mutex
	registry    map[string]string // pattern → label for middleware
	routeLabels *map[string]string
}

// MountDeps holds all typed interface dependencies needed by mounted handlers.
// It has exactly 5 external kernel-port fields plus 2 internal bookkeeping fields.
// No sync.Mutex lives on MountDeps itself — composition is single-threaded at boot.
type MountDeps struct {
	Observability       ports.Observability       // kernel port: observability (optional)
	Bus                 CommandSubmitter          // kernel port: command dispatcher
	GraphStore          ports.GraphStore          // kernel port: graph read/write
	GraphNodeFetcher    ports.GraphNodeFetcher    // kernel port: point read on graph
	JournalStreamReader ports.JournalStreamReader // kernel port: stream read on journal

	Journal  ports.JournalStore // backing journal for JournalStreamReader creation
	regState *registrationState // internal concurrency-safe registration bookkeeping
}

// WorkItemReader is the narrow read port for work items.
type WorkItemReader interface {
	GetItem(ctx context.Context, tenant ports.TenantID, id string) (WorkItem, error)
	GetTypeDef(ctx context.Context, tenant ports.TenantID, name string) (TypeDef, error)
}

// WorkItemWriter is the narrow write port for work items (commands only, via Bus).
type WorkItemWriter interface {
	// No direct writes — all mutations go through Bus.Submit(command.Command)
}

// SCMStreamReader is the narrow port for SCM event ingestion.
type SCMStreamReader interface {
	ReadStream(ctx context.Context, tenant ports.TenantID, stream string, from uint64) ([]ports.RawEvent, error)
}

// ArtifactReader is the narrow port for artifact lookups.
type ArtifactReader interface {
	GetArtifact(ctx context.Context, tenant ports.TenantID, id string) (Artifact, error)
}

// ReleaseGraphReader is the narrow port for release graph queries.
type ReleaseGraphReader interface {
	GetRelease(ctx context.Context, tenant ports.TenantID, id string) (Release, error)
}

// SupplyChainEvidenceReader is the narrow port for supply chain evidence.
type SupplyChainEvidenceReader interface {
	GetEvidence(ctx context.Context, tenant ports.TenantID, id string) (SupplyChainEvidence, error)
}

// BlastRadiusQuery is the narrow port for blast radius analysis.
type BlastRadiusQuery interface {
	QueryBlastRadius(ctx context.Context, tenant ports.TenantID, componentPURL string) (BlastRadiusResult, error)
}

// TestRunReader is the narrow port for verification test runs.
type TestRunReader interface {
	GetTestRun(ctx context.Context, tenant ports.TenantID, id string) (TestRun, error)
}

// WorkItem is the read model for a projected work item node.
type WorkItem struct {
	ID       string
	Kind     string
	Revision uint64
	Fields   map[string]any
}

// TypeDef is the read model for a work type definition.
type TypeDef struct {
	Name        string
	Initial     string
	States      []string
	Transitions []Transition
	Fields      []FieldDef
}

// Transition is a declared state transition.
type Transition struct {
	From string
	To   string
}

// FieldDef is a declared custom field.
type FieldDef struct {
	Name     string
	Type     string
	Required bool
}

// Artifact is the read model for a projected artifact node.
type Artifact struct {
	ID       string
	Kind     string
	Revision uint64
	PURL     string
}

// Release is the read model for a projected release node.
type Release struct {
	ID        string
	Kind      string
	Revision  uint64
	Name      string
	CreatedAt string
}

// SupplyChainEvidence is the read model for supply chain evidence.
type SupplyChainEvidence struct {
	ID       string
	Kind     string
	Revision uint64
	Type     string
}

// BlastRadiusResult is the blast radius analysis result.
type BlastRadiusResult struct {
	Component string
	Impacted  []string
	MaxDepth  int
}

// TestRun is the read model for a test run.
type TestRun struct {
	ID        string
	Status    string
	StartedAt string
}

// registrationState returns the registration state, creating it if needed.
// This is concurrency-safe: RegisterRoute and RouteLabels lock on the inner mutex.
func (d *MountDeps) registrationState() *registrationState {
	if d.regState == nil {
		d.regState = &registrationState{}
	}
	return d.regState
}

// RegisterRoute registers a route on mux and records it in the pattern registry.
// It returns the effective pattern string and an error if overlap is detected.
// The prefix argument specifies the anchor prefix (primary Pattern() or an
// AdditionalPattern() from MultiMount). The registered path is <prefix><subpattern>
// (e.g., "/api/v1/work-items" + "/{id}" = "/api/v1/work-items/{id}").
// In Go 1.22+, the mux supports method-specific patterns like "GET /path".
// RegisterRoute is NOT concurrency-safe — call it only at composition time.
func (d *MountDeps) RegisterRoute(mux *http.ServeMux, method, subpattern, prefix string, h http.HandlerFunc) (effective string, err error) {
	effective = prefix + subpattern
	// Go 1.22+ mux uses method-specific patterns.
	pattern := method + " " + effective

	rs := d.registrationState()
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.registry == nil {
		rs.registry = make(map[string]string)
	}

	// Check for overlap with existing registrations.
	// Segments are compared; {param} wildcards allow sibling routes.
	for existing := range rs.registry {
		// Extract pattern from existing (skip method prefix).
		existingParts := strings.SplitN(existing, " ", 2)
		if len(existingParts) != 2 {
			continue
		}
		existingMethod, existingPattern := existingParts[0], existingParts[1]
		if existingMethod == method && segmentsOverlap(effective, existingPattern) {
			return "", fmt.Errorf("%w: %q conflicts with %q", ErrPatternOverlap, effective, existingPattern)
		}
	}

	// Use Handle (not HandleFunc) with method-specific pattern for Go 1.22+.
	mux.Handle(pattern, http.HandlerFunc(h))
	rs.registry[pattern] = effective

	// Also record in the Server's route labels map if set.
	if rs.routeLabels != nil {
		(*rs.routeLabels)[effective] = effective
	}

	return effective, nil
}

// segmentsOverlap checks if two URL patterns overlap (would match the same concrete paths).
// Returns true if they have the same number of segments and all static (non-{param})
// segments match, AND they don't diverge at a terminal where one is a param and the other is static.
// Two routes like "/work-items/{id}" and "/work-items/events" do NOT overlap
// because they diverge at the last segment (one is a param, the other is a static string).
func segmentsOverlap(a, b string) bool {
	pa := strings.Split(strings.Trim(a, "/"), "/")
	pb := strings.Split(strings.Trim(b, "/"), "/")
	if len(pa) != len(pb) {
		return false
	}
	// Check if any segment diverges: one is param, other is static.
	for i := range pa {
		aIsParam := strings.HasPrefix(pa[i], "{") && strings.HasSuffix(pa[i], "}")
		bIsParam := strings.HasPrefix(pb[i], "{") && strings.HasSuffix(pb[i], "}")
		if aIsParam != bIsParam {
			return false // One is param, other is static at this position.
		}
		if !aIsParam && !bIsParam && pa[i] != pb[i] {
			return false // Both static but different.
		}
	}
	return true
}

// RouteLabels returns a copy of the current route label registry.
// Used by the middleware to look up route labels for metrics.
func (d *MountDeps) RouteLabels() map[string]string {
	rs := d.registrationState()
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if rs.registry == nil {
		return nil
	}
	out := make(map[string]string, len(rs.registry))
	for k, v := range rs.registry {
		out[k] = v
	}
	return out
}

// NewWithMounts constructs a Server using the Mount system. The legacy
// New/Bus/Graph/streams constructor is preserved for backward compatibility;
// this constructor is for the mount-based wiring path.
func NewWithMounts(bus CommandSubmitter, deps MountDeps, mounts []HTTPMount) *Server {
	s := &Server{
		commands: bus,
		obs:      obs.Fill(deps.Observability),
		graph:    deps.GraphStore,
		streams:  deps.JournalStreamReader,
		journal:  deps.Journal,
		mounts:   mounts,
	}
	// Wire mounts into a temporary mux to capture the mount's routes.
	// The mounts are also stored in s.mounts so that Handler() uses routesWithMounts().
	mux := http.NewServeMux()
	for _, m := range mounts {
		if err := m.Mount(mux, deps); err != nil {
			// At construction time we expect no registration errors; fail fast.
			panic("mount " + m.Pattern() + ": " + err.Error())
		}
	}
	return s
}
