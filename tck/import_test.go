package tck_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	checkpointmem "github.com/Rubentxu/golem/adapters/checkpoint/memstore"
	graphmem "github.com/Rubentxu/golem/adapters/graph/memstore"
	journalmem "github.com/Rubentxu/golem/adapters/journal/memstore"
	registrymem "github.com/Rubentxu/golem/adapters/registry/memstore"
	searchmem "github.com/Rubentxu/golem/adapters/search/memstore"
	transportmem "github.com/Rubentxu/golem/adapters/transport/memstore"
	appplanning "github.com/Rubentxu/golem/internal/application/planning"
	appprojects "github.com/Rubentxu/golem/internal/application/projects"
	"github.com/Rubentxu/golem/internal/application/runtime"
	appwork "github.com/Rubentxu/golem/internal/application/work"
	"github.com/Rubentxu/golem/internal/importer/tuleap"
	"github.com/Rubentxu/golem/internal/ports"
)

// TestTuleapImport covers the M2 exit criterion: import a Tuleap fixture
// (project, tracker, artifacts, links, comments), find it through the
// normal API surfaces, and re-run the import idempotently (no new
// events).
func TestTuleapImport(t *testing.T) {
	fx, err := tuleap.LoadFixture("../testdata/tuleap/fixture-basic.json")
	if err != nil {
		t.Fatal(err)
	}

	rt, err := runtime.New(runtime.Options{
		Journal:    journalmem.NewJournal(),
		Graph:      graphmem.NewGraph(),
		Registry:   registrymem.NewRegistry(),
		Transport:  transportmem.NewTransport(),
		Checkpoint: checkpointmem.NewCheckpoints(),
		Search:     searchmem.NewSearch(),
	})
	if err != nil {
		t.Fatal(err)
	}
	rt.Bus.Register(appwork.CmdCreateWorkItem, appwork.CreateWorkItemHandler(rt.IDs, rt.Graph))
	rt.Bus.Register(appwork.CmdRegisterWorkType, appwork.RegisterWorkTypeHandler())
	rt.Bus.Register(appwork.CmdLinkWorkItems, appwork.LinkWorkItemsHandler(rt.Graph))
	rt.Bus.Register(appwork.CmdAddComment, appwork.AddCommentHandler(rt.IDs, rt.Journal))
	rt.Bus.Register(appprojects.CmdCreateProject, appprojects.CreateProjectHandler(rt.IDs))
	rt.Bus.Register(appplanning.CmdCreateIteration, appplanning.CreateIterationHandler(rt.IDs))

	ctx := context.Background()
	sync := func(ctx context.Context) error {
		for {
			n, err := rt.ProjectBatch(ctx, 10)
			if err != nil {
				return err
			}
			if n == 0 {
				return nil
			}
		}
	}
	rep := tuleap.Import(ctx, fx, "t_import", rt.Bus, tuleap.Options{Sync: sync})
	if len(rep.Errors) != 0 {
		t.Fatalf("import errors: %v", rep.Errors)
	}
	if rep.Projects != 1 || rep.Trackers != 1 || rep.Artifacts != 2 || rep.Links != 1 || rep.Comments != 1 {
		t.Fatalf("report = %+v", rep)
	}

	// Drain loops so projections catch up.
	for {
		n, _ := rt.ProjectBatch(ctx, 10)
		m, _ := rt.PublishBatch(ctx, 10)
		s, _ := rt.SearchBatch(ctx, 10)
		if n == 0 && m == 0 && s == 0 {
			break
		}
	}

	// Assertions on the projected graph.
	item1 := "tuleap-item-9001"
	item2 := "tuleap-item-9002"
	n, err := rt.Graph.GetNode(ctx, "t_import", item2)
	if err != nil {
		t.Fatalf("imported item missing: %v", err)
	}
	if n.Attributes["external_provider"] != "tuleap" || n.Attributes["external_id"] != "9002" {
		t.Fatalf("external identity not projected: %+v", n.Attributes)
	}
	if n.Attributes["status"] != "open" {
		t.Fatalf("imported status = %v (workflow initial), want open", n.Attributes["status"])
	}
	sub, err := rt.Graph.Neighborhood(ctx, ports.NeighborhoodQuery{
		TenantID: "t_import", Roots: []string{item2}, MaxDepth: 1, MaxNodes: 10, MaxEdges: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	hasDepends := false
	for _, e := range sub.Edges {
		if e.Type == "DEPENDS_ON" {
			hasDepends = true
		}
	}
	if !hasDepends {
		t.Fatalf("imported DEPENDS_ON edge missing: %+v", sub.Edges)
	}

	// Comments live in the item history.
	evs, err := rt.Journal.ReadStream(ctx, "t_import", "workitem:"+item1, 0)
	if err != nil || len(evs) != 2 { // created + comment
		t.Fatalf("item history = %d events, err %v; want 2", len(evs), err)
	}
	if evs[1].EventType != "work.comment.added.v1" {
		t.Fatalf("second event = %s, want comment", evs[1].EventType)
	}

	// Search finds the imported comment text.
	page, err := rt.Search.Query(ctx, ports.SearchQuery{Tenant: "t_import", Q: "Imported from tracker", Limit: 10})
	if err != nil || len(page.Hits) != 1 || page.Hits[0].Doc.Kind != "Comment" {
		t.Fatalf("search comment: %d hits, err %v", len(page.Hits), err)
	}

	// Idempotent re-import: every receipt duplicates, nothing journaled.
	headBefore, err := rt.Journal.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rep2 := tuleap.Import(ctx, fx, "t_import", rt.Bus, tuleap.Options{Sync: sync})
	if len(rep2.Errors) != 0 {
		t.Fatalf("re-import errors: %v", rep2.Errors)
	}
	if rep2.Projects+rep2.Trackers+rep2.Artifacts+rep2.Links+rep2.Comments != 0 {
		t.Fatalf("re-import created entities: %+v", rep2)
	}
	if rep2.Skipped != 6 { // 1 project + 1 tracker + 2 artifacts + 1 link + 1 comment
		t.Fatalf("re-import skipped = %d, want 6: %+v", rep2.Skipped, rep2)
	}
	headAfter, err := rt.Journal.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if headBefore != headAfter {
		t.Fatalf("re-import journaled events: %d -> %d", headBefore, headAfter)
	}

	// Malformed fixture path fails cleanly.
	if _, err := tuleap.LoadFixture("../testdata/tuleap/does-not-exist.json"); err == nil {
		t.Fatal("missing fixture must error")
	}

	_ = json.Marshal
	_ = os.Getenv
	_ = time.Now
}
