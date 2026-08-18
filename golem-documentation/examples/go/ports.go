package ports

import "context"

type Revision uint64

type GraphCapabilities struct {
	Transactions         bool
	EdgeProperties       bool
	TemporalIndexes      bool
	DistributedTraversal bool
	VectorSearch         bool
}

type GraphMutation struct {
	TenantID string
	Ops      []GraphOp
}

type GraphOp struct {
	Kind   string
	Target string
	Data   map[string]any
}

type NeighborhoodQuery struct {
	TenantID string
	Roots    []string
	MaxDepth int
	MaxNodes int
	MaxEdges int
}

type Node struct {
	ID         string
	Kind       string
	Revision   Revision
	Attributes map[string]any
}

type Edge struct {
	ID         string
	Type       string
	SourceID   string
	TargetID   string
	Revision   Revision
	Attributes map[string]any
}

type Subgraph struct {
	Nodes []Node
	Edges []Edge
}

type GraphStore interface {
	Apply(context.Context, GraphMutation) (Revision, error)
	Neighborhood(context.Context, NeighborhoodQuery) (Subgraph, error)
	Capabilities(context.Context) GraphCapabilities
}
