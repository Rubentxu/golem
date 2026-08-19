package ports

import (
	"context"
)

// GraphNodeFetcher is the kernel-level narrow port for point reads on the
// graph. It is used by mounted handlers that need to read a single node
// without traversing relationships (as opposed to GraphStore.Neighborhood
// which is a bounded traversal).
//
// This interface is introduced in M10b to satisfy REQ-MOUNT-GraphNodeFetcher.
// It mirrors EntityRefReader (M10) as a narrow, single-method kernel port.
type GraphNodeFetcher interface {
	// GetNode returns a single node by tenant and id. Returns ErrNodeNotFound
	// when the node does not exist. The call is scoped to one node (no traversal).
	GetNode(ctx context.Context, tenant TenantID, id string) (Node, error)
}

// NewGraphNodeFetcherOverGraphStore returns a GraphNodeFetcher that delegates
// to the provided GraphStore. This is the reference adapter.
func NewGraphNodeFetcherOverGraphStore(gs GraphStore) GraphNodeFetcher {
	return &graphNodeFetcherAdapter{gs: gs}
}

type graphNodeFetcherAdapter struct {
	gs GraphStore
}

func (a *graphNodeFetcherAdapter) GetNode(ctx context.Context, tenant TenantID, id string) (Node, error) {
	return a.gs.GetNode(ctx, tenant, id)
}
