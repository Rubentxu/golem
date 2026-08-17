package ports

import "context"

// Revision is the optimistic-concurrency version of a graph entity
// (ADR-021).
type Revision uint64

// GraphCapabilities reports what a GraphStore adapter can honor. Provider
// capabilities are negotiated explicitly (ADR-048); projections must degrade
// gracefully when a capability is false.
type GraphCapabilities struct {
	Transactions         bool
	EdgeProperties       bool
	TemporalIndexes      bool
	DistributedTraversal bool
	VectorSearch         bool
}

// GraphMutation is an atomic set of graph operations scoped to one tenant.
type GraphMutation struct {
	TenantID string
	Ops      []GraphOp
}

// GraphOp is a single node/edge operation. Kind is one of "upsert_node",
// "remove_node", "upsert_edge", "remove_edge"; Target is the entity ID.
type GraphOp struct {
	Kind   string
	Target string
	Data   map[string]any
}

// NeighborhoodQuery is a bounded traversal. Every traversal must specify
// tenant, roots and explicit limits (max depth, max nodes/edges, deadline
// at the context level).
type NeighborhoodQuery struct {
	TenantID string
	Roots    []string
	MaxDepth int
	MaxNodes int
	MaxEdges int
}

// Node is the canonical graph node envelope: id, kind, revision,
// attributes.
type Node struct {
	ID         string
	Kind       string
	Revision   Revision
	Attributes map[string]any
}

// Edge is the canonical graph edge envelope between node IDs.
type Edge struct {
	ID         string
	Type       string
	SourceID   string
	TargetID   string
	Revision   Revision
	Attributes map[string]any
}

// Subgraph is a bounded result set of nodes and edges.
type Subgraph struct {
	Nodes []Node
	Edges []Edge
}

// GraphStore is the port for the Engineering Graph projection. The graph
// database behind it is spike-gated (ADR-013); in-memory and reference
// adapters must pass the same TCK.
type GraphStore interface {
	Apply(ctx context.Context, tx GraphMutation) (Revision, error)
	Neighborhood(ctx context.Context, q NeighborhoodQuery) (Subgraph, error)
	Capabilities(ctx context.Context) GraphCapabilities
}
