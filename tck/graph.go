package tck

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

// RunGraphStoreTCK is the black-box conformance kit for the GraphStore
// port (ADR-046). It fixes the semantics of the Engineering Graph
// projection so any provider — in-memory, HugeGraph, NebulaGraph or a
// simulator — is replaceable by contract (ADR-052).
//
// The factory must return an empty, isolated store per call.
func RunGraphStoreTCK(t *testing.T, newStore func() ports.GraphStore) {
	upsertNode := func(id, kind string, attrs map[string]any) ports.GraphOp {
		return ports.GraphOp{Kind: ports.OpUpsertNode, Target: id, Data: map[string]any{"kind": kind, "attributes": attrs}}
	}
	upsertEdge := func(id, typ, src, tgt string) ports.GraphOp {
		return ports.GraphOp{Kind: ports.OpUpsertEdge, Target: id, Data: map[string]any{"type": typ, "source": src, "target": tgt}}
	}
	apply := func(t *testing.T, s ports.GraphStore, tenant string, ops ...ports.GraphOp) ports.Revision {
		t.Helper()
		rev, err := s.Apply(context.Background(), ports.GraphMutation{TenantID: ports.TenantID(tenant), Ops: ops})
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		return rev
	}

	t.Run("upsert and read back", func(t *testing.T) {
		s := newStore()
		apply(t, s, "t1", upsertNode("wi-1", "WorkItem", map[string]any{"title": "kernel"}))
		sub, err := s.Neighborhood(context.Background(), ports.NeighborhoodQuery{TenantID: "t1", Roots: []string{"wi-1"}, MaxDepth: 1, MaxNodes: 10, MaxEdges: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(sub.Nodes) != 1 || len(sub.Edges) != 0 {
			t.Fatalf("got %d nodes / %d edges, want 1/0", len(sub.Nodes), len(sub.Edges))
		}
		n := sub.Nodes[0]
		if n.Kind != "WorkItem" || n.Attributes["title"] != "kernel" {
			t.Fatalf("node = %+v", n)
		}
	})

	t.Run("revisions increment monotonically", func(t *testing.T) {
		s := newStore()
		r1 := apply(t, s, "t1", upsertNode("wi-1", "WorkItem", nil))
		r2 := apply(t, s, "t1", upsertNode("wi-1", "WorkItem", map[string]any{"title": "v2"}))
		if r2 <= r1 {
			t.Fatalf("graph revision did not increase: %d then %d", r1, r2)
		}
		sub, _ := s.Neighborhood(context.Background(), ports.NeighborhoodQuery{TenantID: "t1", Roots: []string{"wi-1"}, MaxDepth: 1, MaxNodes: 10, MaxEdges: 10})
		if sub.Nodes[0].Revision != 2 {
			t.Fatalf("node revision = %d, want 2", sub.Nodes[0].Revision)
		}
	})

	t.Run("upsert merges attributes", func(t *testing.T) {
		s := newStore()
		apply(t, s, "t1",
			upsertNode("wi-1", "WorkItem", map[string]any{"title": "a"}),
			upsertNode("wi-1", "WorkItem", map[string]any{"status": "open"}),
		)
		sub, _ := s.Neighborhood(context.Background(), ports.NeighborhoodQuery{TenantID: "t1", Roots: []string{"wi-1"}, MaxDepth: 1, MaxNodes: 10, MaxEdges: 10})
		attrs := sub.Nodes[0].Attributes
		if attrs["title"] != "a" || attrs["status"] != "open" {
			t.Fatalf("merge failed: %+v", attrs)
		}
	})

	t.Run("tenants are isolated", func(t *testing.T) {
		s := newStore()
		apply(t, s, "tA", upsertNode("wi-1", "WorkItem", nil))
		sub, err := s.Neighborhood(context.Background(), ports.NeighborhoodQuery{TenantID: "tB", Roots: []string{"wi-1"}, MaxDepth: 1, MaxNodes: 10, MaxEdges: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(sub.Nodes) != 0 {
			t.Fatalf("tenant B saw tenant A nodes: %+v", sub.Nodes)
		}
		// Same node ID in another tenant is an independent entity.
		apply(t, s, "tB", upsertNode("wi-1", "Project", map[string]any{"name": "other"}))
		sub, _ = s.Neighborhood(context.Background(), ports.NeighborhoodQuery{TenantID: "tA", Roots: []string{"wi-1"}, MaxDepth: 1, MaxNodes: 10, MaxEdges: 10})
		if sub.Nodes[0].Kind != "WorkItem" {
			t.Fatalf("tenant A node mutated by tenant B write: %+v", sub.Nodes[0])
		}
	})

	t.Run("traversals are bounded", func(t *testing.T) {
		s := newStore()
		ops := []ports.GraphOp{}
		for i := 1; i <= 10; i++ {
			ops = append(ops, upsertNode(fmt.Sprintf("n%d", i), "N", nil))
		}
		for i := 1; i < 10; i++ {
			ops = append(ops, upsertEdge(fmt.Sprintf("e%d", i), "DEPENDS_ON", fmt.Sprintf("n%d", i), fmt.Sprintf("n%d", i+1)))
		}
		apply(t, s, "t1", ops...)

		// Depth bound: 2 hops from n1 → n1,n2,n3.
		sub, err := s.Neighborhood(context.Background(), ports.NeighborhoodQuery{TenantID: "t1", Roots: []string{"n1"}, MaxDepth: 2, MaxNodes: 100, MaxEdges: 100})
		if err != nil {
			t.Fatal(err)
		}
		if len(sub.Nodes) != 3 || len(sub.Edges) != 2 {
			t.Fatalf("depth bound: %d nodes / %d edges, want 3/2", len(sub.Nodes), len(sub.Edges))
		}

		// Node budget respected without error.
		sub, err = s.Neighborhood(context.Background(), ports.NeighborhoodQuery{TenantID: "t1", Roots: []string{"n1"}, MaxDepth: 10, MaxNodes: 2, MaxEdges: 100})
		if err != nil {
			t.Fatal(err)
		}
		if len(sub.Nodes) > 2 {
			t.Fatalf("max nodes exceeded: %d", len(sub.Nodes))
		}

		// Unbounded queries are rejected, not truncated silently.
		if _, err := s.Neighborhood(context.Background(), ports.NeighborhoodQuery{TenantID: "t1", Roots: []string{"n1"}, MaxDepth: 0, MaxNodes: 10, MaxEdges: 10}); !errors.Is(err, ports.ErrUnboundedQuery) {
			t.Fatalf("err = %v, want ErrUnboundedQuery", err)
		}
	})

	t.Run("point read is tenant scoped", func(t *testing.T) {
		s := newStore()
		apply(t, s, "t1", upsertNode("wi-1", "WorkItem", map[string]any{"title": "x"}))
		n, err := s.GetNode(context.Background(), ports.TenantID("t1"), "wi-1")
		if err != nil {
			t.Fatal(err)
		}
		if n.Kind != "WorkItem" || n.Attributes["title"] != "x" {
			t.Fatalf("node = %+v", n)
		}
		if _, err := s.GetNode(context.Background(), ports.TenantID("t1"), "ghost"); !errors.Is(err, ports.ErrNodeNotFound) {
			t.Fatalf("err = %v, want ErrNodeNotFound", err)
		}
		if _, err := s.GetNode(context.Background(), ports.TenantID("t2"), "wi-1"); !errors.Is(err, ports.ErrNodeNotFound) {
			t.Fatalf("cross-tenant read: err = %v, want ErrNodeNotFound", err)
		}
	})

	t.Run("removing a node removes incident edges", func(t *testing.T) {
		s := newStore()
		apply(t, s, "t1",
			upsertNode("a", "N", nil), upsertNode("b", "N", nil),
			upsertEdge("e1", "DEPENDS_ON", "a", "b"),
		)
		apply(t, s, "t1", ports.GraphOp{Kind: ports.OpRemoveNode, Target: "a"})
		sub, err := s.Neighborhood(context.Background(), ports.NeighborhoodQuery{TenantID: "t1", Roots: []string{"b"}, MaxDepth: 1, MaxNodes: 10, MaxEdges: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(sub.Nodes) != 1 || len(sub.Edges) != 0 {
			t.Fatalf("got %d nodes / %d edges, want 1/0", len(sub.Nodes), len(sub.Edges))
		}
	})

	t.Run("edges require existing endpoints", func(t *testing.T) {
		s := newStore()
		apply(t, s, "t1", upsertNode("a", "N", nil))
		_, err := s.Apply(context.Background(), ports.GraphMutation{TenantID: "t1", Ops: []ports.GraphOp{upsertEdge("e1", "DEPENDS_ON", "a", "ghost")}})
		if !errors.Is(err, ports.ErrEndpointNotFound) {
			t.Fatalf("err = %v, want ErrEndpointNotFound", err)
		}
	})

	t.Run("node kind and edge type are immutable", func(t *testing.T) {
		s := newStore()
		apply(t, s, "t1", upsertNode("a", "WorkItem", nil), upsertNode("b", "WorkItem", nil), upsertEdge("e1", "DEPENDS_ON", "a", "b"))
		_, err := s.Apply(context.Background(), ports.GraphMutation{TenantID: "t1", Ops: []ports.GraphOp{upsertNode("a", "Project", nil)}})
		if !errors.Is(err, ports.ErrKindMismatch) {
			t.Fatalf("kind: err = %v, want ErrKindMismatch", err)
		}
		_, err = s.Apply(context.Background(), ports.GraphMutation{TenantID: "t1", Ops: []ports.GraphOp{upsertEdge("e1", "IMPLEMENTS", "a", "b")}})
		if !errors.Is(err, ports.ErrTypeMismatch) {
			t.Fatalf("type: err = %v, want ErrTypeMismatch", err)
		}
	})

	t.Run("canceled context aborts traversal", func(t *testing.T) {
		s := newStore()
		apply(t, s, "t1", upsertNode("a", "N", nil))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := s.Neighborhood(ctx, ports.NeighborhoodQuery{TenantID: "t1", Roots: []string{"a"}, MaxDepth: 1, MaxNodes: 1, MaxEdges: 1}); err == nil {
			t.Fatal("expected context error")
		}
	})
}
