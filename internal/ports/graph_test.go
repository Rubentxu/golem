package ports

import (
	"context"
	"testing"
)

// fakeNodeGraphStore is a minimal in-memory GraphStore for testing EntityRefReader.
type fakeNodeGraphStore struct {
	nodes map[string]Node
}

func (f *fakeNodeGraphStore) Apply(ctx context.Context, tx GraphMutation) (Revision, error) {
	return 1, nil
}

func (f *fakeNodeGraphStore) Neighborhood(ctx context.Context, q NeighborhoodQuery) (Subgraph, error) {
	return Subgraph{}, nil
}

func (f *fakeNodeGraphStore) Traversal(ctx context.Context, q TraversalQuery) (Subgraph, error) {
	return Subgraph{}, nil
}

func (f *fakeNodeGraphStore) GetNode(ctx context.Context, tenant TenantID, nodeID string) (Node, error) {
	n, ok := f.nodes[nodeID]
	if !ok {
		return Node{}, ErrNodeNotFound
	}
	return n, nil
}

func (f *fakeNodeGraphStore) ListNodes(ctx context.Context, tenant TenantID) ([]Node, error) {
	return nil, nil
}

func (f *fakeNodeGraphStore) ListEdges(ctx context.Context, tenant TenantID) ([]Edge, error) {
	return nil, nil
}

func (f *fakeNodeGraphStore) Capabilities(ctx context.Context) GraphCapabilities {
	return GraphCapabilities{}
}

func TestEntityRefReader_Exists(t *testing.T) {
	store := &fakeNodeGraphStore{
		nodes: map[string]Node{
			"item-1": {ID: "item-1", Kind: "WorkItem", Revision: 1},
			"type-1": {ID: "type-1", Kind: "WorkType", Revision: 1},
		},
	}
	reader := NewEntityRefReaderOverGraphStore(store)
	ctx := context.Background()
	tenant := TenantID("t1")

	// Existing entity.
	exists, err := reader.Exists(ctx, tenant, "WorkItem", "item-1")
	if err != nil {
		t.Fatalf("Exists for existing: %v", err)
	}
	if !exists {
		t.Fatal("expected exists=true for existing WorkItem")
	}

	// Non-existing entity.
	exists, err = reader.Exists(ctx, tenant, "WorkItem", "nonexistent")
	if err != nil {
		t.Fatalf("Exists for nonexistent: %v", err)
	}
	if exists {
		t.Fatal("expected exists=false for nonexistent entity")
	}

	// Wrong kind but existing ID.
	exists, err = reader.Exists(ctx, tenant, "WrongKind", "item-1")
	if err != nil {
		t.Fatalf("Exists for wrong kind: %v", err)
	}
	if exists {
		t.Fatal("expected exists=false for wrong kind")
	}
}

func TestEntityRefReader_KindOf(t *testing.T) {
	store := &fakeNodeGraphStore{
		nodes: map[string]Node{
			"item-1": {ID: "item-1", Kind: "WorkItem", Revision: 1},
			"type-1": {ID: "type-1", Kind: "WorkType", Revision: 1},
		},
	}
	reader := NewEntityRefReaderOverGraphStore(store)
	ctx := context.Background()
	tenant := TenantID("t1")

	// Existing entity.
	kind, err := reader.KindOf(ctx, tenant, "item-1")
	if err != nil {
		t.Fatalf("KindOf for existing: %v", err)
	}
	if kind != "WorkItem" {
		t.Fatalf("expected WorkItem, got %s", kind)
	}

	// Non-existing entity.
	_, err = reader.KindOf(ctx, tenant, "nonexistent")
	if err != ErrNodeNotFound {
		t.Fatalf("expected ErrNodeNotFound, got %v", err)
	}
}
