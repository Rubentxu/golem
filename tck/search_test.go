package tck_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	checkpointmem "github.com/Rubentxu/golem/adapters/checkpoint/memstore"
	graphmem "github.com/Rubentxu/golem/adapters/graph/memstore"
	journalmem "github.com/Rubentxu/golem/adapters/journal/memstore"
	registrymem "github.com/Rubentxu/golem/adapters/registry/memstore"
	searchmem "github.com/Rubentxu/golem/adapters/search/memstore"
	transportmem "github.com/Rubentxu/golem/adapters/transport/memstore"
	"github.com/Rubentxu/golem/internal/api/httpapi"
	"github.com/Rubentxu/golem/internal/application/command"
	appreq "github.com/Rubentxu/golem/internal/application/requirements"
	"github.com/Rubentxu/golem/internal/application/runtime"
	appwork "github.com/Rubentxu/golem/internal/application/work"
	"github.com/Rubentxu/golem/internal/ports"
)

// TestSearchProjectionAndRebuild proves ADR-015 end to end: commands flow
// into the search projection via the runtime loop, queries return
// tenant-scoped results over HTTP, and a fresh index rebuilt from the
// journal (the "search never owns data" drill) yields identical results.
func TestSearchProjectionAndRebuild(t *testing.T) {
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
	rt.Bus.Register(appwork.CmdCreateWorkItem, appwork.CreateWorkItemHandler(rt.IDs))
	rt.Bus.Register(appwork.CmdUpdateWorkItem, appwork.UpdateWorkItemHandler(rt.Journal))
	rt.Bus.Register(appreq.CmdCreateRequirement, appreq.CreateRequirementHandler(rt.IDs))

	ctx := context.Background()
	submit := func(name string, payload any) {
		t.Helper()
		_, err := rt.Bus.Submit(ctx, command.Command{
			Name: name, TenantID: "t_demo",
			Actor: ports.Actor{Type: "user", ID: "u_1"}, Payload: payload,
		})
		if err != nil {
			t.Fatalf("submit %s: %v", name, err)
		}
	}
	submit(appwork.CmdCreateWorkItem, appwork.CreateWorkItem{Title: "Kernel journal slice", ItemType: "task"})
	submit(appwork.CmdCreateWorkItem, appwork.CreateWorkItem{Title: "Graph projection digest", ItemType: "task"})
	submit(appwork.CmdCreateWorkItem, appwork.CreateWorkItem{Title: "Budget guard", ItemType: "bug"})
	submit(appreq.CmdCreateRequirement, appreq.CreateRequirement{Title: "Journal is authoritative", Statement: "search is derived"})

	// Drain all loops.
	for {
		n, _ := rt.ProjectBatch(ctx, 10)
		m, _ := rt.PublishBatch(ctx, 10)
		s, _ := rt.SearchBatch(ctx, 10)
		if n == 0 && m == 0 && s == 0 {
			break
		}
	}

	// Query over HTTP.
	srv := httptest.NewServer(httpapi.New(rt.Bus, rt.Graph, rt.Journal).WithSearch(rt.Search).Handler())
	defer srv.Close()

	type hit struct {
		ID    string  `json:"id"`
		Kind  string  `json:"kind"`
		Title string  `json:"title"`
		Score float64 `json:"score"`
	}
	type page struct {
		Hits       []hit  `json:"hits"`
		NextCursor string `json:"next_cursor"`
	}
	search := func(tenant, rawQuery string) page {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/search?"+rawQuery, nil)
		req.Header.Set("X-Golem-Tenant", tenant)
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var p page
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("search %q: status %d", rawQuery, resp.StatusCode)
		}
		if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
			t.Fatal(err)
		}
		return p
	}

	if p := search("t_demo", "q=slice"); len(p.Hits) != 1 || p.Hits[0].Title != "Kernel journal slice" {
		t.Fatalf("q=slice hits = %+v", p.Hits)
	}
	if p := search("t_demo", "q=authoritative&kind=Requirement"); len(p.Hits) != 1 || p.Hits[0].Kind != "Requirement" {
		t.Fatalf("kind filter hits = %+v", p.Hits)
	}
	if p := search("t_other", "q=slice"); len(p.Hits) != 0 {
		t.Fatalf("tenant leak: %+v", p.Hits)
	}

	// Rebuild drill (ADR-015/049): fresh index, journal replay, identical
	// results.
	fresh := searchmem.NewSearch()
	evs, _, err := rt.Journal.Replay(ctx, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, env := range evs {
		docs, err := rt.SearchProj.Project(env)
		if err != nil {
			t.Fatal(err)
		}
		if len(docs) > 0 {
			if err := fresh.Index(ctx, docs); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, q := range []struct{ text, kind string }{
		{"slice", ""}, {"graph", ""}, {"", ""}, {"authoritative", "Requirement"},
	} {
		want, err := rt.Search.Query(ctx, ports.SearchQuery{Tenant: "t_demo", Q: q.text, Kind: q.kind, Limit: 100})
		if err != nil {
			t.Fatal(err)
		}
		got, err := fresh.Query(ctx, ports.SearchQuery{Tenant: "t_demo", Q: q.text, Kind: q.kind, Limit: 100})
		if err != nil {
			t.Fatal(err)
		}
		if len(want.Hits) != len(got.Hits) {
			t.Fatalf("rebuild mismatch (q=%q kind=%q): %d vs %d hits", q.text, q.kind, len(want.Hits), len(got.Hits))
		}
		for i := range want.Hits {
			if want.Hits[i].Doc.ID != got.Hits[i].Doc.ID || want.Hits[i].Score != got.Hits[i].Score {
				t.Fatalf("rebuild mismatch (q=%q) at %d: %+v vs %+v", q.text, i, want.Hits[i], got.Hits[i])
			}
		}
	}
	_ = strings.TrimSpace
}
