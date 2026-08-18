// Package lens implements the GOLEM lens engine (GRAPH_LENSES.md): a
// read-only, tenant-bound, budgeted, deterministic, serializable,
// inspectable view over the Engineering Graph, materialised through the
// typed TraversalQuery substrate (M4).
package lens

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/Rubentxu/golem/internal/ports"
)

// ErrLensBudgetExceeded is returned when the traversal underlying a lens
// hits any bound (depth/nodes/edges). Lenses fail closed: a budgeted view
// never returns a partial result.
var ErrLensBudgetExceeded = errors.New("lens: budget exceeded (traversal truncated)")

// Spec is the declarative lens definition of GRAPH_LENSES.md. It maps
// 1:1 onto ports.TraversalQuery plus evidence serialisation policy.
type Spec struct {
	Roots     []string `json:"roots"`
	NodeTypes []string `json:"node_types,omitempty"` // node kinds (any when empty)
	EdgeTypes []string `json:"edge_types,omitempty"` // edge types (any when empty)
	MaxDepth  int      `json:"max_depth"`
	MaxNodes  int      `json:"max_nodes"`
	MaxEdges  int      `json:"max_edges"`
	// TimeWindow is a window restriction ("P90D"). Validated and carried in
	// the result metadata; v1 does NOT filter by it (the graph projection
	// has no time index yet — M6.1 with the time-indexed reader).
	TimeWindow string `json:"time_window,omitempty"`
	// Evidence controls whether node/edge attributes are included in the
	// serialisable result (true) or stripped to ids/kinds/types (false).
	Evidence bool `json:"evidence"`
}

// Result is the materialised lens view. Serialisable via ToJSON in a
// deterministic byte form (sorted ids, sorted map keys — encoding/json
// sorts map keys).
type Result struct {
	Spec       Spec         `json:"spec"`
	Nodes      []ports.Node `json:"nodes"`
	Edges      []ports.Edge `json:"edges"`
	Truncated  bool         `json:"truncated"`
	TimeWindow string       `json:"time_window,omitempty"`
}

// Execute materialises a lens over the tenant graph. Read-only: it never
// mutates the store. Returns ErrLensBudgetExceeded when the traversal
// truncates.
func Execute(ctx context.Context, g ports.GraphStore, tenant ports.TenantID, spec Spec) (*Result, error) {
	if len(spec.Roots) == 0 {
		return nil, fmt.Errorf("lens: roots must not be empty")
	}
	if spec.MaxDepth <= 0 || spec.MaxNodes <= 0 || spec.MaxEdges <= 0 {
		return nil, fmt.Errorf("lens: max_depth/max_nodes/max_edges must be positive")
	}

	sub, err := g.Traversal(ctx, ports.TraversalQuery{
		TenantID:  tenant,
		Roots:     spec.Roots,
		EdgeTypes: spec.EdgeTypes,
		Kinds:     spec.NodeTypes,
		MaxDepth:  spec.MaxDepth,
		MaxNodes:  spec.MaxNodes,
		MaxEdges:  spec.MaxEdges,
	})
	if err != nil {
		return nil, fmt.Errorf("lens: traversal: %w", err)
	}
	if sub.Truncated {
		return nil, ErrLensBudgetExceeded
	}

	nodes := sub.Nodes
	edges := sub.Edges
	if !spec.Evidence {
		nodes = stripNodeAttrs(nodes)
		edges = stripEdgeAttrs(edges)
	}
	sortNodes(nodes)
	sortEdges(edges)

	return &Result{
		Spec:       spec,
		Nodes:      nodes,
		Edges:      edges,
		TimeWindow: spec.TimeWindow,
	}, nil
}

// ToJSON serialises the result deterministically: nodes and edges sorted
// by ID, attributes with map-key sorting (encoding/json contract).
func (r *Result) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// Validate checks the spec shape (bounds positive, roots non-empty,
// time window syntactic when present).
func (s Spec) Validate() error {
	if len(s.Roots) == 0 {
		return fmt.Errorf("lens: roots must not be empty")
	}
	if s.MaxDepth <= 0 || s.MaxNodes <= 0 || s.MaxEdges <= 0 {
		return fmt.Errorf("lens: bounds must be positive")
	}
	if s.TimeWindow != "" && !validTimeWindow(s.TimeWindow) {
		return fmt.Errorf("lens: invalid time_window %q", s.TimeWindow)
	}
	return nil
}

// validTimeWindow accepts ISO-8601 durations with a leading P (P90D, P1M).
// v1 is syntactic: no calendar arithmetic is performed.
func validTimeWindow(w string) bool {
	if len(w) < 2 || w[0] != 'P' {
		return false
	}
	for _, c := range w[1:] {
		if !((c >= '0' && c <= '9') || c == 'D' || c == 'W' || c == 'M' || c == 'Y') {
			return false
		}
	}
	return true
}

func stripNodeAttrs(nodes []ports.Node) []ports.Node {
	out := make([]ports.Node, len(nodes))
	for i, n := range nodes {
		n.Attributes = nil
		out[i] = n
	}
	return out
}

func stripEdgeAttrs(edges []ports.Edge) []ports.Edge {
	out := make([]ports.Edge, len(edges))
	for i, e := range edges {
		e.Attributes = nil
		out[i] = e
	}
	return out
}

func sortNodes(nodes []ports.Node) {
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
}

func sortEdges(edges []ports.Edge) {
	sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })
}
