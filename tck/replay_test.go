package tck_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	graphmem "github.com/Rubentxu/golem/adapters/graph/memstore"
	journalmem "github.com/Rubentxu/golem/adapters/journal/memstore"
	"github.com/Rubentxu/golem/internal/application/projection"
	"github.com/Rubentxu/golem/internal/ports"
	"github.com/Rubentxu/golem/internal/work"
)

// TestJournalReplayRebuildsSameGraphDigest is the M1 exit criterion:
// a graph projection rebuilt from the journal (with checkpointed paging,
// as a recovering projector would) yields the same canonical digest as
// the live projection. Derived stores are disposable and reproducible
// (ADR-049).
func TestJournalReplayRebuildsSameGraphDigest(t *testing.T) {
	const tenant = "t_demo"
	roots := []string{"wi-1", "wi-2", "wi-3", "wi-4", "wi-5"}

	events := []ports.RawEvent{
		created(tenant, "01E1", "wi-1", "Slice kernel", "epic"),
		created(tenant, "01E2", "wi-2", "Journal port", "task"),
		created(tenant, "01E3", "wi-3", "Graph projection", "task"),
		created(tenant, "01E4", "wi-4", "GraphStoreTCK", "task"),
		created(tenant, "01E5", "wi-5", "Replay digest", "task"),
		linked(tenant, "01E6", "wi-1", "wi-2", "CONTAINS"),
		linked(tenant, "01E7", "wi-1", "wi-3", "CONTAINS"),
		linked(tenant, "01E8", "wi-3", "wi-4", "DEPENDS_ON"),
		updated(tenant, "01E9", "wi-3", nil, strPtr("in_progress")),
	}

	jrnl := journalmem.NewJournal()
	if _, err := jrnl.Append(context.Background(), events); err != nil {
		t.Fatalf("append: %v", err)
	}

	// Live projection: consume the journal in one pass.
	live := graphmem.NewGraph()
	replayInto(t, jrnl, live, 0)

	// Recovery projection: a fresh store rebuilt page by page, resuming
	// from the last checkpoint after each batch (SP-003 rehearsal).
	rebuilt := graphmem.NewGraph()
	for checkpoint := ports.StreamPosition(0); ; {
		batch, last, err := jrnl.Replay(context.Background(), checkpoint, 3)
		if err != nil {
			t.Fatalf("replay page: %v", err)
		}
		if len(batch) == 0 {
			break
		}
		for _, env := range batch {
			if _, err := projection.ApplyIfHandled(projection.Projector{}, rebuilt, env); err != nil {
				t.Fatalf("rebuild: %v", err)
			}
		}
		checkpoint = last
	}

	digestLive := digestOf(t, live, tenant, roots)
	digestRebuilt := digestOf(t, rebuilt, tenant, roots)
	if digestLive == "" || digestLive != digestRebuilt {
		t.Fatalf("digest mismatch after replay: live=%s rebuilt=%s", digestLive, digestRebuilt)
	}

	// Content assertions on the rebuilt graph.
	sub := subgraphOf(t, rebuilt, tenant, roots)
	if len(sub.Nodes) != 5 {
		t.Fatalf("nodes = %d, want 5", len(sub.Nodes))
	}
	if len(sub.Edges) != 3 {
		t.Fatalf("edges = %d, want 3", len(sub.Edges))
	}
	for _, n := range sub.Nodes {
		if n.ID == "wi-3" && n.Attributes["status"] != "in_progress" {
			t.Fatalf("wi-3 status = %v, want in_progress (update not merged)", n.Attributes["status"])
		}
	}

	// The live store itself must also be reproducible: re-projecting the
	// same journal into another fresh store gives the same digest again.
	again := graphmem.NewGraph()
	replayInto(t, jrnl, again, 0)
	if d := digestOf(t, again, tenant, roots); d != digestLive {
		t.Fatalf("projection is not deterministic: %s vs %s", d, digestLive)
	}
}

func replayInto(t *testing.T, j ports.JournalStore, g ports.GraphStore, limit int) {
	t.Helper()
	for checkpoint := ports.StreamPosition(0); ; {
		batch, last, err := j.Replay(context.Background(), checkpoint, limit)
		if err != nil {
			t.Fatalf("replay: %v", err)
		}
		if len(batch) == 0 {
			return
		}
		for _, env := range batch {
			if _, err := projection.ApplyIfHandled(projection.Projector{}, g, env); err != nil {
				t.Fatalf("project: %v", err)
			}
		}
		checkpoint = last
	}
}

func subgraphOf(t *testing.T, g ports.GraphStore, tenant string, roots []string) ports.Subgraph {
	t.Helper()
	sub, err := g.Neighborhood(context.Background(), ports.NeighborhoodQuery{
		TenantID: ports.TenantID(tenant), Roots: roots, MaxDepth: 2, MaxNodes: 100, MaxEdges: 100,
	})
	if err != nil {
		t.Fatalf("neighborhood: %v", err)
	}
	return sub
}

func digestOf(t *testing.T, g ports.GraphStore, tenant string, roots []string) string {
	t.Helper()
	d, err := projection.Digest(subgraphOf(t, g, tenant, roots))
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	return d
}

func created(tenant, eid, id, title, typ string) ports.RawEvent {
	return env(tenant, eid, "workitem:"+id, work.EventItemCreated, work.ItemCreated{
		ItemID: id, Title: title, ItemType: typ, Status: "open",
	})
}

func linked(tenant, eid, from, to, rel string) ports.RawEvent {
	return env(tenant, eid, "workitem:"+from, work.EventItemLinked, work.ItemLinked{
		FromID: from, ToID: to, Relation: rel,
	})
}

func updated(tenant, eid, id string, title, status *string) ports.RawEvent {
	return env(tenant, eid, "workitem:"+id, work.EventItemUpdated, work.ItemUpdated{
		ItemID: id, Title: title, Status: status,
	})
}

func env(tenant, eid, stream, eventType string, payload any) ports.RawEvent {
	b, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return ports.RawEvent{
		EventID:       eid,
		TenantID:      tenant,
		StreamID:      stream,
		EventType:     eventType,
		SchemaVersion: 1,
		OccurredAt:    time.Now().UTC(),
		Actor:         ports.Actor{Type: "user", ID: "u_1"},
		Payload:       b,
	}
}

func strPtr(s string) *string { return &s }
