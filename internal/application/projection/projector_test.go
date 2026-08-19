package projection

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Rubentxu/golem/internal/ci"
	"github.com/Rubentxu/golem/internal/ports"
	"github.com/Rubentxu/golem/internal/work"
)

func mkEvent(eventType string, payload any) ports.RawEvent {
	b, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return ports.RawEvent{
		EventID:       "01JTEST0000000000000000EV1",
		TenantID:      "t_test",
		StreamID:      "workitem:wi-1",
		EventType:     eventType,
		SchemaVersion: 1,
		OccurredAt:    time.Now().UTC(),
		Actor:         ports.Actor{Type: "user", ID: "u_1"},
		Payload:       b,
	}
}

func TestProjectItemCreated(t *testing.T) {
	m, err := (Projector{}).Project(mkEvent(work.EventItemCreated, work.ItemCreated{
		ItemID: "wi-1", Title: "Kernel slice", ItemType: "task", Status: "open",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Ops) != 1 {
		t.Fatalf("ops = %d, want 1", len(m.Ops))
	}
	op := m.Ops[0]
	if op.Kind != ports.OpUpsertNode || op.Target != "wi-1" || op.Data["kind"] != KindWorkItem {
		t.Fatalf("op = %+v", op)
	}
	if m.TenantID != "t_test" {
		t.Fatalf("tenant = %q, want t_test", m.TenantID)
	}
}

func TestProjectItemUpdatedOnlyChangedFields(t *testing.T) {
	newTitle := "renamed"
	m, err := (Projector{}).Project(mkEvent(work.EventItemUpdated, work.ItemUpdated{
		ItemID: "wi-1", Title: &newTitle,
	}))
	if err != nil {
		t.Fatal(err)
	}
	attrs := m.Ops[0].Data["attributes"].(map[string]any)
	if _, hasStatus := attrs["status"]; hasStatus {
		t.Fatalf("nil status must not be projected: %+v", attrs)
	}
	if attrs["title"] != newTitle {
		t.Fatalf("title = %v", attrs["title"])
	}
}

func TestProjectItemLinkedUsesEventIDAsEdgeID(t *testing.T) {
	m, err := (Projector{}).Project(mkEvent(work.EventItemLinked, work.ItemLinked{
		FromID: "wi-1", ToID: "wi-2", Relation: " depends_on ",
	}))
	if err != nil {
		t.Fatal(err)
	}
	op := m.Ops[0]
	if op.Kind != ports.OpUpsertEdge || op.Target != "01JTEST0000000000000000EV1" {
		t.Fatalf("op = %+v", op)
	}
	if op.Data["type"] != "DEPENDS_ON" {
		t.Fatalf("relation not canonicalized: %v", op.Data["type"])
	}
}

func TestProjectUnknownEventIsSkipped(t *testing.T) {
	m, err := (Projector{}).Project(mkEvent("deployment.service.registered.v1", map[string]any{"id": "svc-1"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Ops) != 0 {
		t.Fatalf("unknown event produced ops: %+v", m.Ops)
	}
}

func TestProjectInvalidPayloadErrors(t *testing.T) {
	ev := mkEvent(work.EventItemCreated, nil)
	ev.Payload = []byte(`{not json`)
	if _, err := (Projector{}).Project(ev); err == nil {
		t.Fatal("expected payload decode error")
	}
}

// fakeGraphStore records every Apply call so we can verify batching invariants.
// Batching matters for throughput: coalescing N ops into one Apply call reduces
// graph-store round-trips and amortizes transaction overhead (ADR-087).
type fakeGraphStore struct {
	calls []struct {
		tenant ports.TenantID
		ops    []ports.GraphOp
	}
}

func (f *fakeGraphStore) Apply(_ context.Context, tx ports.GraphMutation) (ports.Revision, error) {
	// Defensively copy ops so the record is stable.
	ops := make([]ports.GraphOp, len(tx.Ops))
	copy(ops, tx.Ops)
	f.calls = append(f.calls, struct {
		tenant ports.TenantID
		ops    []ports.GraphOp
	}{tx.TenantID, ops})
	return ports.Revision(len(f.calls)), nil
}

func (fakeGraphStore) Neighborhood(context.Context, ports.NeighborhoodQuery) (ports.Subgraph, error) {
	return ports.Subgraph{}, nil
}
func (fakeGraphStore) Traversal(context.Context, ports.TraversalQuery) (ports.Subgraph, error) {
	return ports.Subgraph{}, nil
}
func (fakeGraphStore) GetNode(context.Context, ports.TenantID, string) (ports.Node, error) {
	return ports.Node{}, ports.ErrNodeNotFound
}
func (fakeGraphStore) ListNodes(context.Context, ports.TenantID) ([]ports.Node, error) {
	return nil, nil
}
func (fakeGraphStore) ListEdges(context.Context, ports.TenantID) ([]ports.Edge, error) {
	return nil, nil
}
func (fakeGraphStore) Capabilities(context.Context) ports.GraphCapabilities {
	return ports.GraphCapabilities{Transactions: true}
}

// TestProjection_BatchesMultipleOpsPerEvent verifies that a single event which
// produces multiple ops (e.g. ci.build.completed with artifacts) results in
// exactly ONE Apply call containing all ops. This documents the batching
// invariant: ApplyIfHandled must not split a single event's ops into multiple
// Apply invocations.
func TestProjection_BatchesMultipleOpsPerEvent(t *testing.T) {
	store := &fakeGraphStore{}
	p := Projector{}

	// ci.build.completed produces: Build node + BUILT_BY edge + per-artifact
	// (Artifact node + PRODUCED edge). With 2 artifacts that is 1+1+2+2 = 6 ops.
	env := mkEvent(ci.EventBuildCompleted, ci.BuildCompleted{
		BuildID:  "build-1",
		Pipeline: "p-1",
		Commit:   "abc123",
		Status:   "success",
		Artifacts: []ci.ArtifactOut{
			{Digest: "sha256:aaaa", Name: "bin", Kind: "Artifact"},
			{Digest: "sha256:bbbb", Name: "lib", Kind: "Artifact"},
		},
	})

	_, err := ApplyIfHandled(p, store, env)
	if err != nil {
		t.Fatalf("ApplyIfHandled error: %v", err)
	}

	// ADR-087 batching invariant: one event must result in exactly one Apply call.
	if len(store.calls) != 1 {
		t.Fatalf("Apply call count = %d, want 1 — ops should be coalesced into one call", len(store.calls))
	}

	gotOps := store.calls[0].ops
	// Expected ops: build-node + built_by-edge + 2×(artifact-node + produced-edge) = 6
	if len(gotOps) != 6 {
		t.Fatalf("ops in single Apply = %d, want 6: %+v", len(gotOps), gotOps)
	}

	// Verify the ops represent the full mutation atomically.
	var nodeKinds, edgeTypes int
	for _, op := range gotOps {
		switch op.Kind {
		case ports.OpUpsertNode:
			nodeKinds++
		case ports.OpUpsertEdge:
			edgeTypes++
		}
	}
	if nodeKinds != 3 {
		t.Errorf("node ops = %d, want 3 (build + 2 artifacts)", nodeKinds)
	}
	if edgeTypes != 3 {
		t.Errorf("edge ops = %d, want 3 (built_by + 2×produced)", edgeTypes)
	}
}

// TestProjection_ApplyIfHandledSkipsEmptyMutation verifies that events which
// produce zero ops do not call Apply at all (no unnecessary round-trips).
func TestProjection_ApplyIfHandledSkipsEmptyMutation(t *testing.T) {
	store := &fakeGraphStore{}
	p := Projector{}

	// An unknown event type yields zero ops; ApplyIfHandled must skip it.
	env := mkEvent("unknown.entity.happened.v1", map[string]any{"id": "x"})
	handled, err := ApplyIfHandled(p, store, env)
	if err != nil {
		t.Fatalf("ApplyIfHandled error: %v", err)
	}
	if handled {
		t.Error("unknown event should not be handled")
	}
	if len(store.calls) != 0 {
		t.Errorf("Apply call count = %d, want 0 for empty mutation", len(store.calls))
	}
}

// TestApplyIfHandled_ShimReturnsFalseForUnknown verifies the shim semantics that
// scenario.Fork relies on: unknown events return (false, nil) so Fork increments
// OverlaySkipped. This is the (bool, error) contract that the scenario overlay
// path depends on to distinguish "applied" from "skipped".
func TestApplyIfHandled_ShimReturnsFalseForUnknown(t *testing.T) {
	store := &fakeGraphStore{}
	p := Projector{}

	unknownEvent := mkEvent("deployment.service.registered.v1", map[string]any{"id": "svc-1"})
	applied, err := ApplyIfHandled(p, store, unknownEvent)
	if err != nil {
		t.Fatalf("ApplyIfHandled unknown event: error = %v, want nil", err)
	}
	if applied {
		t.Error("unknown event should return applied=false")
	}
	if len(store.calls) != 0 {
		t.Errorf("Apply calls = %d, want 0 for unknown event", len(store.calls))
	}

	// Simulate scenario.Fork semantics: when applied=false, OverlaySkipped++.
	// This is the critical invariant: unknown events must NOT be counted as
	// applied, otherwise ForkResult.OverlayApplied would be wrong.
	overlaySkipped := 0
	if !applied {
		overlaySkipped++
	}
	if overlaySkipped != 1 {
		t.Errorf("overlaySkipped = %d, want 1 for unknown event", overlaySkipped)
	}
}
