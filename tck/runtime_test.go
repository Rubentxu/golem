package tck_test

import (
	"context"
	"testing"

	checkpointmem "github.com/Rubentxu/golem/adapters/checkpoint/memstore"
	graphmem "github.com/Rubentxu/golem/adapters/graph/memstore"
	journalmem "github.com/Rubentxu/golem/adapters/journal/memstore"
	registrymem "github.com/Rubentxu/golem/adapters/registry/memstore"
	transportmem "github.com/Rubentxu/golem/adapters/transport/memstore"
	"github.com/Rubentxu/golem/internal/application/command"
	"github.com/Rubentxu/golem/internal/application/runtime"
	appwork "github.com/Rubentxu/golem/internal/application/work"
	"github.com/Rubentxu/golem/internal/ports"
)

// TestRuntimeLoops wires the full modular-monolith runtime with the
// reference adapters and proves the asynchronous half of the write path
// (ARCHITECTURE): accepted events reach the graph projection and the
// event transport through checkpointed tail loops, and consumers ack
// idempotently (ADR-020 at-least-once).
func TestRuntimeLoops(t *testing.T) {
	ctx := context.Background()

	rt, err := runtime.New(runtime.Options{
		Journal:    journalmem.NewJournal(),
		Graph:      graphmem.NewGraph(),
		Registry:   registrymem.NewRegistry(),
		Transport:  transportmem.NewTransport(),
		Checkpoint: checkpointmem.NewCheckpoints(),
	})
	if err != nil {
		t.Fatal(err)
	}
	rt.Bus.Register(appwork.CmdCreateWorkItem, appwork.CreateWorkItemHandler(rt.IDs, rt.Graph))

	// Three commands: two accepted, one domain-rejected.
	mk := func(title string) string {
		r, err := rt.Bus.Submit(ctx, command.Command{
			Name:     appwork.CmdCreateWorkItem,
			TenantID: "t_demo",
			Actor:    ports.Actor{Type: "user", ID: "u_1"},
			Payload:  appwork.CreateWorkItem{Title: title, ItemType: "task"},
		})
		if err != nil {
			t.Fatalf("submit %q: %v", title, err)
		}
		return r.CommandID
	}
	mk("First item")
	mk("Second item")
	if _, err := rt.Bus.Submit(ctx, command.Command{
		Name:     appwork.CmdCreateWorkItem,
		TenantID: "t_demo",
		Actor:    ports.Actor{Type: "user", ID: "u_1"},
		Payload:  appwork.CreateWorkItem{Title: "  ", ItemType: "task"},
	}); err == nil {
		t.Fatal("expected domain rejection")
	}

	// Pump both loops until caught up.
	for {
		n, err := rt.ProjectBatch(ctx, 10)
		if err != nil {
			t.Fatal(err)
		}
		m, err := rt.PublishBatch(ctx, 10)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 && m == 0 {
			break
		}
	}

	// Projection: both nodes exist under the tenant.
	sub, err := rt.Graph.Neighborhood(ctx, ports.NeighborhoodQuery{
		TenantID: "t_demo", Roots: []string{itemIDOf(t, rt, 0), itemIDOf(t, rt, 1)}, MaxDepth: 1, MaxNodes: 10, MaxEdges: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sub.Nodes) != 2 {
		t.Fatalf("projected %d nodes, want 2", len(sub.Nodes))
	}

	// Transport: exactly the two accepted events, in order, redelivered
	// until acked; ack drains the queue.
	got, err := rt.Transport.Fetch(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("fetched %d events, want 2", len(got))
	}
	for _, e := range got {
		if err := rt.Transport.Ack(ctx, e.EventID); err != nil {
			t.Fatal(err)
		}
	}
	if again, _ := rt.Transport.Fetch(ctx, 10); len(again) != 0 {
		t.Fatalf("queue not drained: %d events remain", len(again))
	}

	// Caught-up loops are no-ops.
	if n, _ := rt.ProjectBatch(ctx, 10); n != 0 {
		t.Fatalf("project batch after catch-up = %d, want 0", n)
	}
	if m, _ := rt.PublishBatch(ctx, 10); m != 0 {
		t.Fatalf("publish batch after catch-up = %d, want 0", m)
	}
}

func itemIDOf(t *testing.T, rt *runtime.Runtime, n int) string {
	t.Helper()
	evs, _, err := rt.Journal.Replay(context.Background(), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if n >= len(evs) {
		t.Fatalf("event %d does not exist (journal has %d)", n, len(evs))
	}
	const prefix = "workitem:"
	if len(evs[n].StreamID) <= len(prefix) || evs[n].StreamID[:len(prefix)] != prefix {
		t.Fatalf("unexpected stream id %q", evs[n].StreamID)
	}
	return evs[n].StreamID[len(prefix):]
}
