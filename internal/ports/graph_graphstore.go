package ports

import "context"

// graphStoreEntityRefReader adapts a GraphStore to implement EntityRefReader
// by delegating to GetNode.
type graphStoreEntityRefReader struct {
	gs GraphStore
}

// NewEntityRefReaderOverGraphStore creates an EntityRefReader that delegates to
// the provided GraphStore.
func NewEntityRefReaderOverGraphStore(gs GraphStore) EntityRefReader {
	return graphStoreEntityRefReader{gs: gs}
}

// Exists returns true if an entity with the given kind and id exists in the graph.
func (g graphStoreEntityRefReader) Exists(ctx context.Context, tenant TenantID, kind, id string) (bool, error) {
	node, err := g.gs.GetNode(ctx, tenant, id)
	if err != nil {
		if err == ErrNodeNotFound {
			return false, nil
		}
		return false, err
	}
	return node.Kind == kind, nil
}

// KindOf returns the kind of the entity with the given id.
func (g graphStoreEntityRefReader) KindOf(ctx context.Context, tenant TenantID, id string) (string, error) {
	node, err := g.gs.GetNode(ctx, tenant, id)
	if err != nil {
		return "", err
	}
	return node.Kind, nil
}
