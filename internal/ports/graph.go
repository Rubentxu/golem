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
	TenantID TenantID
	Ops      []GraphOp
}

// Graph operation kinds.
const (
	OpUpsertNode = "upsert_node"
	OpRemoveNode = "remove_node"
	OpUpsertEdge = "upsert_edge"
	OpRemoveEdge = "remove_edge"
)

// GraphOp is a single node/edge operation.
//
// Canonical Data keys:
//
//	upsert_node: "kind" (string), "attributes" (map)
//	upsert_edge: "type", "source", "target" (strings), "attributes" (map)
//
// Target is the node or edge ID. Upserts merge attributes and increment
// revisions; kind and type are immutable.
type GraphOp struct {
	Kind   string
	Target string
	Data   map[string]any
}

// NeighborhoodQuery is a bounded traversal. Every traversal must specify
// tenant, roots and explicit limits (max depth, max nodes/edges, deadline
// at the context level).
type NeighborhoodQuery struct {
	TenantID TenantID
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

// TraversalQuery is a typed bounded traversal. Every typed traversal must
// specify tenant, roots, optional edge-type / node-kind filters, and explicit
// limits (max depth, max nodes/edges, deadline at the context level).
// Empty EdgeTypes or Kinds mean "any".
type TraversalQuery struct {
	TenantID  TenantID
	Roots     []string
	EdgeTypes []string // empty = any edge type
	Kinds     []string // empty = any node kind
	MaxDepth  int
	MaxNodes  int
	MaxEdges  int
}

// Subgraph is a bounded result set of nodes and edges.
type Subgraph struct {
	Nodes     []Node
	Edges     []Edge
	Truncated bool // true when any bound (depth/nodes/edges) was hit
}

// GraphStore is the port for the Engineering Graph projection. The graph
// database behind it is spike-gated (ADR-013); in-memory and reference
// adapters must pass the same TCK.
type GraphStore interface {
	Apply(ctx context.Context, tx GraphMutation) (Revision, error)
	Neighborhood(ctx context.Context, q NeighborhoodQuery) (Subgraph, error)
	// Traversal is a typed bounded walk with optional edge-type and node-kind
	// filters. It sets Subgraph.Truncated=true when any bound (MaxDepth,
	// MaxNodes, MaxEdges) is reached. The walk is undirected: it follows both
	// incoming and outgoing edges from each visited node.
	Traversal(ctx context.Context, q TraversalQuery) (Subgraph, error)
	// GetNode is a bounded point read of one node. Tenant-scoped; returns
	// ErrNodeNotFound when the node does not exist in the tenant graph.
	// (Query-safety bounds apply to traversals, not point lookups.)
	GetNode(ctx context.Context, tenant TenantID, nodeID string) (Node, error)
	// ListNodes returns all nodes for a tenant. Used by canonical export.
	// Returns nodes in ascending ID order for deterministic results.
	ListNodes(ctx context.Context, tenant TenantID) ([]Node, error)
	// ListEdges returns all edges for a tenant in ascending ID order.
	ListEdges(ctx context.Context, tenant TenantID) ([]Edge, error)
	Capabilities(ctx context.Context) GraphCapabilities
}

// EntityRefReader is the kernel-level port for reading entity existence and kind.
// It is used by contexts that need to verify entity existence without full
// graph traversal.
type EntityRefReader interface {
	// Exists returns true if an entity with the given kind and id exists.
	Exists(ctx context.Context, tenant TenantID, kind, id string) (bool, error)
	// KindOf returns the kind of the entity with the given id.
	// Returns ErrNodeNotFound if the entity does not exist.
	KindOf(ctx context.Context, tenant TenantID, id string) (string, error)
}
