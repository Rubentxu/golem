package tck_test

import (
	"context"
	"errors"
	"testing"

	graphmem "github.com/Rubentxu/golem/adapters/graph/memstore"
	journalmem "github.com/Rubentxu/golem/adapters/journal/memstore"
	registrymem "github.com/Rubentxu/golem/adapters/registry/memstore"
	"github.com/Rubentxu/golem/internal/application/command"
	"github.com/Rubentxu/golem/internal/application/projection"
	appwork "github.com/Rubentxu/golem/internal/application/work"
	"github.com/Rubentxu/golem/internal/clock"
	"github.com/Rubentxu/golem/internal/ids"
	"github.com/Rubentxu/golem/internal/ports"
)

// TestCommandToProjection closes the kernel write path end to end:
// Command → validation → journal append (idempotent by command_id) →
// receipt with journal position → graph projection sees the node.
func TestCommandToProjection(t *testing.T) {
	ctx := context.Background()
	clk := clock.SystemClock{}
	gen := ids.NewGenerator(clk)

	journal := journalmem.NewJournal()
	registry := registrymem.NewRegistry()
	graph := graphmem.NewGraph()

	bus := command.NewBus(journal, registry, gen, clk)
	bus.Register(appwork.CmdCreateWorkItem, appwork.CreateWorkItemHandler(gen, graph))

	cmd := command.Command{
		Name:     appwork.CmdCreateWorkItem,
		TenantID: "t_demo",
		Actor:    ports.Actor{Type: "user", ID: "u_1"},
		Payload:  appwork.CreateWorkItem{Title: "Command bus slice", ItemType: "task"},
	}

	// First submission: accepted, one event journaled.
	r1, err := bus.Submit(ctx, cmd)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if r1.Duplicate || len(r1.EventIDs) != 1 || r1.Position != 1 {
		t.Fatalf("receipt = %+v", r1)
	}

	// Retry with explicit stable command id: same receipt, no new events.
	cmd.CommandID = r1.CommandID
	r2, err := bus.Submit(ctx, cmd)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if !r2.Duplicate || r2.Position != r1.Position {
		t.Fatalf("retry receipt = %+v, want duplicate of %+v", r2, r1)
	}

	// Second, distinct command: journal grows to two events.
	cmd2 := cmd
	cmd2.CommandID = ""
	cmd2.Payload = appwork.CreateWorkItem{Title: "Second item", ItemType: "bug"}
	r3, err := bus.Submit(ctx, cmd2)
	if err != nil {
		t.Fatalf("second submit: %v", err)
	}
	if r3.Duplicate || r3.Position != 2 {
		t.Fatalf("second receipt = %+v", r3)
	}

	// Domain rejection journals nothing.
	bad := cmd
	bad.CommandID = ""
	bad.Payload = appwork.CreateWorkItem{Title: "   ", ItemType: "task"}
	if _, err := bus.Submit(ctx, bad); !errors.Is(err, appwork.ErrEmptyTitle) {
		t.Fatalf("err = %v, want ErrEmptyTitle", err)
	}

	// Journal holds exactly the two accepted events.
	evs, last, err := journal.Replay(ctx, 0, 0)
	if err != nil || len(evs) != 2 || last != 2 {
		t.Fatalf("journal: %d events, checkpoint %d, err %v", len(evs), last, err)
	}
	for _, e := range evs {
		if e.EventType != "work.item.created.v1" || e.CommandID == "" || e.CorrelationID == "" {
			t.Fatalf("envelope incomplete: %+v", e)
		}
	}

	// Projection consumes the journal; both work items appear in the graph.
	for _, e := range evs {
		if _, err := projection.ApplyIfHandled(projection.Projector{}, graph, e); err != nil {
			t.Fatalf("project: %v", err)
		}
	}
	itemID := payloadItemID(t, evs[0])
	sub, err := graph.Neighborhood(ctx, ports.NeighborhoodQuery{
		TenantID: "t_demo", Roots: []string{itemID}, MaxDepth: 1, MaxNodes: 10, MaxEdges: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sub.Nodes) != 1 || sub.Nodes[0].Attributes["title"] != "Command bus slice" {
		t.Fatalf("projected node = %+v", sub.Nodes)
	}
}

func payloadItemID(t *testing.T, e ports.RawEvent) string {
	t.Helper()
	// item_id is generated; recover it from the projected stream id instead
	// of parsing the payload: stream is "workitem:<id>".
	const prefix = "workitem:"
	if len(e.StreamID) <= len(prefix) || e.StreamID[:len(prefix)] != prefix {
		t.Fatalf("unexpected stream id %q", e.StreamID)
	}
	return e.StreamID[len(prefix):]
}
