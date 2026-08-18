// Package memstore provides the in-memory reference adapter of the
// GraphStore port: the Engineering Graph projection used by the kernel,
// tests and the GraphStoreTCK baseline. The physical graph database is
// spike-gated (ADR-013); this adapter defines the semantics any candidate
// must reproduce.
//
// Semantics (also asserted by tck.RunGraphStoreTCK):
//   - Apply is atomic per mutation and processes node ops before edge ops.
//   - upsert merges attributes; node kind and edge type are immutable;
//     revisions increment per touched entity (optimistic concurrency).
//   - remove_node removes its incident edges; edges require existing
//     endpoints.
//   - Neighborhood is an undirected bounded BFS: every query declares max
//     depth, nodes and edges (query safety, GRAPH_MODEL), honors context
//     cancellation, and never crosses tenants.
package memstore

import (
	"context"
	"sort"
	"sync"

	"github.com/Rubentxu/golem/internal/ports"
)

type tenantGraph struct {
	rev   ports.Revision
	nodes map[string]*ports.Node
	edges map[string]*ports.Edge
	adj   map[string]map[string]struct{} // nodeID -> incident edge IDs
}

func newTenantGraph() *tenantGraph {
	return &tenantGraph{
		nodes: map[string]*ports.Node{},
		edges: map[string]*ports.Edge{},
		adj:   map[string]map[string]struct{}{},
	}
}

// Store is an in-memory multi-tenant GraphStore. Safe for concurrent use.
type Store struct {
	mu      sync.RWMutex
	tenants map[ports.TenantID]*tenantGraph
}

// NewGraph builds an empty graph store.
func NewGraph() *Store {
	return &Store{tenants: map[ports.TenantID]*tenantGraph{}}
}

// Capabilities reports what this adapter honors.
func (s *Store) Capabilities(ctx context.Context) ports.GraphCapabilities {
	_ = ctx
	return ports.GraphCapabilities{Transactions: true, EdgeProperties: true}
}

// Apply validates the whole mutation, then applies node ops (in order) and
// edge ops (in order). Returns the new per-tenant graph revision.
func (s *Store) Apply(ctx context.Context, tx ports.GraphMutation) (ports.Revision, error) {
	_ = ctx
	if tx.TenantID == "" {
		return 0, ports.ErrEmptyTenant
	}
	if len(tx.Ops) == 0 {
		return 0, ports.ErrEmptyMutation
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	g, ok := s.tenants[tx.TenantID]
	if !ok {
		g = newTenantGraph()
		s.tenants[tx.TenantID] = g
	}

	// Validation pass — no mutation until every op checks out. Node ops
	// are simulated on nodeIDs so an edge may reference a node created in
	// the same mutation.
	nodeIDs := make(map[string]struct{}, len(g.nodes))
	for id := range g.nodes {
		nodeIDs[id] = struct{}{}
	}
	for _, op := range tx.Ops {
		switch op.Kind {
		case ports.OpUpsertNode:
			if dataString(op.Data, "kind") == "" {
				return 0, ports.ErrInvalidOp
			}
			if existing, ok := g.nodes[op.Target]; ok && existing.Kind != dataString(op.Data, "kind") {
				return 0, ports.ErrKindMismatch
			}
			nodeIDs[op.Target] = struct{}{}
		case ports.OpRemoveNode:
			if _, ok := g.nodes[op.Target]; !ok {
				return 0, ports.ErrNodeNotFound
			}
			delete(nodeIDs, op.Target)
		case ports.OpUpsertEdge:
			typ := dataString(op.Data, "type")
			src := dataString(op.Data, "source")
			tgt := dataString(op.Data, "target")
			if typ == "" || src == "" || tgt == "" {
				return 0, ports.ErrInvalidOp
			}
			if _, ok := nodeIDs[src]; !ok {
				return 0, ports.ErrEndpointNotFound
			}
			if _, ok := nodeIDs[tgt]; !ok {
				return 0, ports.ErrEndpointNotFound
			}
			if existing, ok := g.edges[op.Target]; ok && existing.Type != typ {
				return 0, ports.ErrTypeMismatch
			}
		case ports.OpRemoveEdge:
			if _, ok := g.edges[op.Target]; !ok {
				return 0, ports.ErrEdgeNotFound
			}
		default:
			return 0, ports.ErrInvalidOp
		}
	}

	// Apply pass.
	for _, op := range tx.Ops {
		switch op.Kind {
		case ports.OpUpsertNode:
			n, ok := g.nodes[op.Target]
			if !ok {
				n = &ports.Node{
					ID:         op.Target,
					Kind:       dataString(op.Data, "kind"),
					Attributes: map[string]any{},
				}
				g.nodes[op.Target] = n
				g.adj[op.Target] = map[string]struct{}{}
			}
			n.Revision++
			mergeAttrs(n.Attributes, dataAttrs(op.Data))
		case ports.OpRemoveNode:
			g.removeNode(op.Target)
		case ports.OpUpsertEdge:
			e, ok := g.edges[op.Target]
			if !ok {
				e = &ports.Edge{
					ID:         op.Target,
					Type:       dataString(op.Data, "type"),
					SourceID:   dataString(op.Data, "source"),
					TargetID:   dataString(op.Data, "target"),
					Attributes: map[string]any{},
				}
				g.edges[op.Target] = e
				g.adj[e.SourceID][op.Target] = struct{}{}
				g.adj[e.TargetID][op.Target] = struct{}{}
			}
			e.Revision++
			mergeAttrs(e.Attributes, dataAttrs(op.Data))
		case ports.OpRemoveEdge:
			g.removeEdge(op.Target)
		}
	}

	g.rev++
	return g.rev, nil
}

// Neighborhood runs a bounded undirected BFS from the existing roots.
// Results are deterministic for a given graph state and query.
func (s *Store) Neighborhood(ctx context.Context, q ports.NeighborhoodQuery) (ports.Subgraph, error) {
	if q.TenantID == "" {
		return ports.Subgraph{}, ports.ErrEmptyTenant
	}
	if q.MaxDepth <= 0 || q.MaxNodes <= 0 || q.MaxEdges <= 0 {
		return ports.Subgraph{}, ports.ErrUnboundedQuery
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	g, ok := s.tenants[q.TenantID]
	if !ok {
		return ports.Subgraph{}, nil
	}

	visitedN := map[string]bool{}
	visitedE := map[string]bool{}
	resNodes := []ports.Node{}
	resEdges := []ports.Edge{}

	frontier := []string{}
	for _, r := range q.Roots {
		if n, ok := g.nodes[r]; ok && !visitedN[r] {
			visitedN[r] = true
			resNodes = append(resNodes, copyNode(n))
			frontier = append(frontier, r)
		}
	}
	sort.Strings(frontier)

loop:
	for depth := 0; depth < q.MaxDepth && len(frontier) > 0; depth++ {
		if err := ctx.Err(); err != nil {
			return ports.Subgraph{}, err
		}
		next := []string{}
		for _, id := range frontier {
			edgeIDs := make([]string, 0, len(g.adj[id]))
			for eid := range g.adj[id] {
				edgeIDs = append(edgeIDs, eid)
			}
			sort.Strings(edgeIDs)
			for _, eid := range edgeIDs {
				if visitedE[eid] {
					continue
				}
				e := g.edges[eid]
				visitedE[eid] = true
				resEdges = append(resEdges, copyEdge(e))

				other := e.SourceID
				if other == id {
					other = e.TargetID
				}
				if !visitedN[other] {
					visitedN[other] = true
					resNodes = append(resNodes, copyNode(g.nodes[other]))
					next = append(next, other)
				}
				if len(resEdges) >= q.MaxEdges || len(resNodes) >= q.MaxNodes {
					break loop
				}
			}
		}
		sort.Strings(next)
		frontier = next
	}

	return ports.Subgraph{Nodes: resNodes, Edges: resEdges}, nil
}

// GetNode is a tenant-scoped point read.
func (s *Store) GetNode(ctx context.Context, tenant ports.TenantID, nodeID string) (ports.Node, error) {
	_ = ctx
	if tenant == "" {
		return ports.Node{}, ports.ErrEmptyTenant
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.tenants[tenant]
	if !ok {
		return ports.Node{}, ports.ErrNodeNotFound
	}
	n, ok := g.nodes[nodeID]
	if !ok {
		return ports.Node{}, ports.ErrNodeNotFound
	}
	return copyNode(n), nil
}

// ListNodes returns all nodes for a tenant in ascending ID order.
func (s *Store) ListNodes(ctx context.Context, tenant ports.TenantID) ([]ports.Node, error) {
	_ = ctx
	if tenant == "" {
		return nil, ports.ErrEmptyTenant
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.tenants[tenant]
	if !ok {
		return nil, nil
	}
	ids := make([]string, 0, len(g.nodes))
	for id := range g.nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]ports.Node, 0, len(ids))
	for _, id := range ids {
		result = append(result, copyNode(g.nodes[id]))
	}
	return result, nil
}

// ListEdges returns all edges for a tenant in ascending ID order.
func (s *Store) ListEdges(ctx context.Context, tenant ports.TenantID) ([]ports.Edge, error) {
	_ = ctx
	if tenant == "" {
		return nil, ports.ErrEmptyTenant
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.tenants[tenant]
	if !ok {
		return nil, nil
	}
	ids := make([]string, 0, len(g.edges))
	for id := range g.edges {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]ports.Edge, 0, len(ids))
	for _, id := range ids {
		result = append(result, copyEdge(g.edges[id]))
	}
	return result, nil
}

func (g *tenantGraph) removeNode(id string) {
	for eid := range g.adj[id] {
		g.removeEdge(eid)
	}
	delete(g.adj, id)
	delete(g.nodes, id)
}

func (g *tenantGraph) removeEdge(id string) {
	e, ok := g.edges[id]
	if !ok {
		return
	}
	delete(g.edges, id)
	if adj, ok := g.adj[e.SourceID]; ok {
		delete(adj, id)
	}
	if adj, ok := g.adj[e.TargetID]; ok {
		delete(adj, id)
	}
}

func dataString(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func dataAttrs(m map[string]any) map[string]any {
	a, _ := m["attributes"].(map[string]any)
	return a
}

func mergeAttrs(dst, src map[string]any) {
	for k, v := range src {
		dst[k] = v
	}
}

// makeFilter returns a predicate that returns true for any value that matches
// one of the accepted strings, or for all values when the accepted set is empty.
func makeFilter(accepted []string) func(string) bool {
	if len(accepted) == 0 {
		return func(string) bool { return true }
	}
	set := map[string]bool{}
	for _, v := range accepted {
		set[v] = true
	}
	return func(v string) bool { return set[v] }
}

func copyNode(n *ports.Node) ports.Node {
	c := *n
	c.Attributes = copyAttrs(n.Attributes)
	return c
}

func copyEdge(e *ports.Edge) ports.Edge {
	c := *e
	c.Attributes = copyAttrs(e.Attributes)
	return c
}

// Traversal runs a typed bounded undirected BFS from the existing roots.
// EdgeTypes and Kinds filters are applied during the walk; empty filters mean
// "accept any". Subgraph.Truncated is set true when any bound is hit.
// Results are deterministic for a given graph state and query.
func (s *Store) Traversal(ctx context.Context, q ports.TraversalQuery) (ports.Subgraph, error) {
	if q.TenantID == "" {
		return ports.Subgraph{}, ports.ErrEmptyTenant
	}
	if q.MaxDepth <= 0 || q.MaxNodes <= 0 || q.MaxEdges <= 0 {
		return ports.Subgraph{}, ports.ErrUnboundedQuery
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	g, ok := s.tenants[q.TenantID]
	if !ok {
		return ports.Subgraph{}, nil
	}

	edgeMatches := makeFilter(q.EdgeTypes)
	nodeMatches := makeFilter(q.Kinds)

	visitedN := map[string]bool{}
	visitedE := map[string]bool{}
	resNodes := []ports.Node{}
	resEdges := []ports.Edge{}

	frontier := []string{}
	for _, r := range q.Roots {
		if n, ok := g.nodes[r]; ok && !visitedN[r] {
			visitedN[r] = true
			resNodes = append(resNodes, copyNode(n)) // roots always included as starting points
			frontier = append(frontier, r)
		}
	}
	sort.Strings(frontier)

	truncated := false
loop:
	for depth := 0; depth < q.MaxDepth && len(frontier) > 0; depth++ {
		if err := ctx.Err(); err != nil {
			return ports.Subgraph{}, err
		}
		next := []string{}
		for _, id := range frontier {
			edgeIDs := make([]string, 0, len(g.adj[id]))
			for eid := range g.adj[id] {
				edgeIDs = append(edgeIDs, eid)
			}
			sort.Strings(edgeIDs)
			for _, eid := range edgeIDs {
				if visitedE[eid] {
					continue
				}
				e := g.edges[eid]
				// Apply edge-type filter.
				if !edgeMatches(e.Type) {
					continue
				}
				visitedE[eid] = true
				resEdges = append(resEdges, copyEdge(e))

				other := e.SourceID
				if other == id {
					other = e.TargetID
				}
				if !visitedN[other] {
					n := g.nodes[other]
					if nodeMatches(n.Kind) {
						visitedN[other] = true
						resNodes = append(resNodes, copyNode(n))
						next = append(next, other)
					}
				}
				if len(resEdges) >= q.MaxEdges || len(resNodes) >= q.MaxNodes {
					truncated = true
					break loop
				}
			}
		}
		sort.Strings(next)
		frontier = next
	}

	return ports.Subgraph{Nodes: resNodes, Edges: resEdges, Truncated: truncated}, nil
}

func copyAttrs(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	c := make(map[string]any, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}
