package tck_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	journalbbolt "github.com/Rubentxu/golem/adapters/journal/bbolt"
	graphmem "github.com/Rubentxu/golem/adapters/graph/memstore"
	journalmem "github.com/Rubentxu/golem/adapters/journal/memstore"
	"github.com/Rubentxu/golem/internal/ports"
)

// seedGraph applies a list of GraphOps to the given graph store.
// It is used to set up test fixtures for port TCK tests.
func seedGraph(t *testing.T, gs ports.GraphStore, ops []ports.GraphOp) {
	t.Helper()
	ctx := context.Background()
	tenant := ports.TenantID("t_tck")
	_, err := gs.Apply(ctx, ports.GraphMutation{TenantID: tenant, Ops: ops})
	if err != nil {
		t.Fatalf("seedGraph apply: %v", err)
	}
}

// seedJournal appends raw events to a journal stream.
// It is used to set up test fixtures for port TCK tests.
func seedJournal(t *testing.T, jrnl ports.JournalStore, stream string, evts []ports.RawEvent) {
	t.Helper()
	ctx := context.Background()
	if len(evts) == 0 {
		return
	}
	_, err := jrnl.Append(ctx, evts)
	if err != nil {
		t.Fatalf("seedJournal append: %v", err)
	}
	_ = stream // stream ID is encoded in each event's StreamID field
}

// PortTCKEnvHelper holds the graph and journal stores for a port TCK test stack.
type PortTCKEnvHelper struct {
	Graph   ports.GraphStore
	Journal ports.JournalStore
}

// newMemStack creates a fresh memstore-backed stack (graph + journal).
// Each call returns a new isolated instance.
func newMemStack(t *testing.T) PortTCKEnvHelper {
	t.Helper()
	return PortTCKEnvHelper{
		Graph:   graphmem.NewGraph(),
		Journal: journalmem.NewJournal(),
	}
}

// withBbolt creates a bbolt-backed stack (graph + journal).
// The journal is backed by a temporary bbolt file that is removed when the test ends.
// Skips the test if GOLEM_TEST_BBOLT is not set.
func withBbolt(t *testing.T) PortTCKEnvHelper {
	t.Helper()
	if os.Getenv("GOLEM_TEST_BBOLT") == "" {
		t.Skip("GOLEM_TEST_BBOLT not set")
	}
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "context_ports_tck.bbolt")
	jrnl, err := journalbbolt.NewJournal(path, journalbbolt.Options{})
	if err != nil {
		t.Fatalf("create bbolt journal: %v", err)
	}
	t.Cleanup(func() { jrnl.Close() })
	return PortTCKEnvHelper{
		Graph:   graphmem.NewGraph(),
		Journal: jrnl,
	}
}

// mkTestEvent creates a RawEvent with the given fields for use in TCK fixtures.
func mkTestEvent(streamID, eventType, eventID string, payload any) ports.RawEvent {
	b, _ := json.Marshal(payload)
	return ports.RawEvent{
		EventID:       eventID,
		TenantID:      "t_tck",
		StreamID:      streamID,
		EventType:     eventType,
		SchemaVersion: 1,
		OccurredAt:    time.Now().UTC(),
		Actor:         ports.Actor{Type: "user", ID: "u_1"},
		Payload:       b,
	}
}

// mkWorkItemCommand creates a WorkItemCommand payload for seeding.
func mkWorkItemCommand(itemID, name string, payload map[string]any) ports.RawEvent {
	b, _ := json.Marshal(payload)
	return ports.RawEvent{
		EventID:       name + "-" + itemID,
		TenantID:      "t_tck",
		StreamID:      "workitem:" + itemID,
		EventType:     name,
		SchemaVersion: 1,
		OccurredAt:    time.Now().UTC(),
		Actor:         ports.Actor{Type: "user", ID: "u_1"},
		Payload:       b,
	}
}

// mkGraphNode is a helper to build a GraphOp for upserting a node.
func mkGraphNode(id, kind string, attrs map[string]any) ports.GraphOp {
	return ports.GraphOp{
		Kind:   ports.OpUpsertNode,
		Target: id,
		Data: map[string]any{
			"kind":       kind,
			"attributes": attrs,
		},
	}
}

// mkGraphEdge is a helper to build a GraphOp for upserting an edge.
func mkGraphEdge(id, edgeType, source, target string) ports.GraphOp {
	return ports.GraphOp{
		Kind:   ports.OpUpsertEdge,
		Target: id,
		Data: map[string]any{
			"type":   edgeType,
			"source": source,
			"target": target,
		},
	}
}

// suppress unused import warning
var _ = os.Getenv
