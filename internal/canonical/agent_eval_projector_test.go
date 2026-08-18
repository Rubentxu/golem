package canonical

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

// mockGraphStore implements ports.GraphStore for testing.
type mockGraphStore struct {
	nodes map[string]ports.Node
	edges map[string]ports.Edge
}

func newMockGraph() *mockGraphStore {
	return &mockGraphStore{
		nodes: make(map[string]ports.Node),
		edges: make(map[string]ports.Edge),
	}
}

func (g *mockGraphStore) Apply(ctx context.Context, tx ports.GraphMutation) (ports.Revision, error) {
	for _, op := range tx.Ops {
		if op.Kind == ports.OpUpsertNode {
			node := ports.Node{
				ID:         op.Target,
				Kind:       op.Data["kind"].(string),
				Attributes: op.Data["attributes"].(map[string]any),
				Revision:   1,
			}
			g.nodes[op.Target] = node
		} else if op.Kind == ports.OpUpsertEdge {
			edge := ports.Edge{
				ID:         op.Target,
				Type:       op.Data["type"].(string),
				SourceID:   op.Data["source"].(string),
				TargetID:   op.Data["target"].(string),
				Attributes: op.Data["attributes"].(map[string]any),
				Revision:   1,
			}
			g.edges[op.Target] = edge
		}
	}
	return 1, nil
}

func (g *mockGraphStore) Neighborhood(ctx context.Context, q ports.NeighborhoodQuery) (ports.Subgraph, error) {
	return ports.Subgraph{}, nil
}

func (g *mockGraphStore) Traversal(ctx context.Context, q ports.TraversalQuery) (ports.Subgraph, error) {
	return ports.Subgraph{}, nil
}

func (g *mockGraphStore) GetNode(ctx context.Context, tenant ports.TenantID, nodeID string) (ports.Node, error) {
	if n, ok := g.nodes[nodeID]; ok {
		return n, nil
	}
	return ports.Node{}, ports.ErrNodeNotFound
}

func (g *mockGraphStore) ListNodes(ctx context.Context, tenant ports.TenantID) ([]ports.Node, error) {
	var result []ports.Node
	for _, n := range g.nodes {
		result = append(result, n)
	}
	return result, nil
}

func (g *mockGraphStore) ListEdges(ctx context.Context, tenant ports.TenantID) ([]ports.Edge, error) {
	var result []ports.Edge
	for _, e := range g.edges {
		result = append(result, e)
	}
	return result, nil
}

func (g *mockGraphStore) Capabilities(ctx context.Context) ports.GraphCapabilities {
	return ports.GraphCapabilities{}
}

func TestAgentEvalProjector_ProjectsDeterministicID(t *testing.T) {
	graph := newMockGraph()
	proj := NewAgentEvalProjector(graph)

	payload := AgentEvalPayload{
		EvalID:     "e-001",
		TenantID:   "t-test",
		BehaviorID: "b-001",
		RunSeq:     1,
		Outcome:    "pass",
	}
	payloadBytes, _ := json.Marshal(payload)
	env := ports.RawEvent{
		EventType: ports.EventAgentEvalCompleted,
		TenantID:  "t-test",
		Payload:   payloadBytes,
	}

	if err := proj.Project(context.Background(), env); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// ID must be deterministic: same behavior_id+tenant_id+run_seq → same ID
	id1 := deriveAgentEvalID("b-001", "t-test", 1)
	id2 := deriveAgentEvalID("b-001", "t-test", 1)
	if id1 != id2 {
		t.Errorf("expected deterministic ID, got %s vs %s", id1, id2)
	}

	// Different run_seq must produce different ID
	id3 := deriveAgentEvalID("b-001", "t-test", 2)
	if id1 == id3 {
		t.Errorf("expected different ID for different run_seq, got same: %s", id1)
	}
}

func TestAgentEvalProjector_ReplayIdempotent(t *testing.T) {
	graph := newMockGraph()
	proj := NewAgentEvalProjector(graph)

	payload := AgentEvalPayload{
		EvalID:           "e-001",
		TenantID:         "t-test",
		BehaviorID:       "b-001",
		RunSeq:           1,
		Outcome:          "pass",
		PolicyViolations: 0,
	}
	payloadBytes, _ := json.Marshal(payload)
	env := ports.RawEvent{
		EventType: ports.EventAgentEvalCompleted,
		TenantID:  "t-test",
		Payload:   payloadBytes,
	}

	// Project twice
	if err := proj.Project(context.Background(), env); err != nil {
		t.Fatalf("first project: %v", err)
	}
	if err := proj.Project(context.Background(), env); err != nil {
		t.Fatalf("second project: %v", err)
	}

	// Must not duplicate node — only one node with this ID
	nodes, _ := graph.ListNodes(context.Background(), "t-test")
	if len(nodes) != 1 {
		t.Errorf("expected 1 node (idempotent), got %d", len(nodes))
	}
}

func TestAgentEvalProjector_CreatesBehaviorEdge(t *testing.T) {
	graph := newMockGraph()
	proj := NewAgentEvalProjector(graph)

	payload := AgentEvalPayload{
		EvalID:           "e-001",
		TenantID:         "t-test",
		BehaviorID:       "b-001",
		RunSeq:           1,
		Outcome:          "pass",
		PolicyViolations: 0,
	}
	payloadBytes, _ := json.Marshal(payload)
	env := ports.RawEvent{
		EventType: ports.EventAgentEvalCompleted,
		TenantID:  "t-test",
		Payload:   payloadBytes,
	}

	if err := proj.Project(context.Background(), env); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	edges, _ := graph.ListEdges(context.Background(), "t-test")
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].Type != EdgeTypeEVALUATED {
		t.Errorf("expected EVALUATED edge, got %s", edges[0].Type)
	}
	if edges[0].TargetID != "b-001" {
		t.Errorf("expected target b-001, got %s", edges[0].TargetID)
	}
}

func TestAgentEvalProjector_CreatesProposalEdge(t *testing.T) {
	graph := newMockGraph()
	proj := NewAgentEvalProjector(graph)

	payload := AgentEvalPayload{
		EvalID:           "e-001",
		TenantID:         "t-test",
		BehaviorID:       "b-001",
		RunSeq:           1,
		Outcome:          "pass",
		ProposalID:       "p-001",
		PolicyViolations: 0,
	}
	payloadBytes, _ := json.Marshal(payload)
	env := ports.RawEvent{
		EventType: ports.EventAgentEvalCompleted,
		TenantID:  "t-test",
		Payload:   payloadBytes,
	}

	if err := proj.Project(context.Background(), env); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	edges, _ := graph.ListEdges(context.Background(), "t-test")
	if len(edges) != 2 {
		t.Fatalf("expected 2 edges (EVALUATED + OBSERVED), got %d", len(edges))
	}

	var observedEdge ports.Edge
	for _, e := range edges {
		if e.Type == EdgeTypeOBSERVED {
			observedEdge = e
			break
		}
	}
	if observedEdge.Type != EdgeTypeOBSERVED {
		t.Errorf("expected OBSERVED edge, got %v", observedEdge)
	}
	if observedEdge.TargetID != "p-001" {
		t.Errorf("expected target p-001, got %s", observedEdge.TargetID)
	}
}

func TestAgentEvalProjector_IgnoresNonAgentEvalEvents(t *testing.T) {
	graph := newMockGraph()
	proj := NewAgentEvalProjector(graph)

	env := ports.RawEvent{
		EventType: "proposal.proposed.v1",
		TenantID:  "t-test",
		Payload:   []byte(`{}`),
	}

	if err := proj.Project(context.Background(), env); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	nodes, _ := graph.ListNodes(context.Background(), "t-test")
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for non-agent.eval event, got %d", len(nodes))
	}
}
