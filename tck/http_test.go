package tck_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	checkpointmem "github.com/Rubentxu/golem/adapters/checkpoint/memstore"
	graphmem "github.com/Rubentxu/golem/adapters/graph/memstore"
	journalmem "github.com/Rubentxu/golem/adapters/journal/memstore"
	registrymem "github.com/Rubentxu/golem/adapters/registry/memstore"
	transportmem "github.com/Rubentxu/golem/adapters/transport/memstore"
	"github.com/Rubentxu/golem/internal/api/httpapi"
	appreq "github.com/Rubentxu/golem/internal/application/requirements"
	"github.com/Rubentxu/golem/internal/application/runtime"
	appwork "github.com/Rubentxu/golem/internal/application/work"
	"github.com/Rubentxu/golem/internal/ports"
)

// TestHTTPVerticalSlice is the first demo of START_HERE, over real HTTP:
// POST command → 202 receipt → journal → (async) graph projection →
// bounded neighborhood query returns the projected node. Includes tenant
// isolation through headers and idempotent retry of the same command.
func TestHTTPVerticalSlice(t *testing.T) {
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
	rt.Bus.Register(appwork.CmdCreateWorkItem, appwork.CreateWorkItemHandler(rt.IDs, appwork.NewWorkItemReaderOverGraphStore(rt.Graph)))
	rt.Bus.Register(appwork.CmdUpdateWorkItem, appwork.UpdateWorkItemHandler(rt.Journal, appwork.NewWorkItemReaderOverGraphStore(rt.Graph)))
	rt.Bus.Register(appwork.CmdLinkWorkItems, appwork.LinkWorkItemsHandler(ports.NewEntityRefReaderOverGraphStore(rt.Graph)))
	rt.Bus.Register(appreq.CmdCreateRequirement, appreq.CreateRequirementHandler(rt.IDs))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = rt.Run(ctx, 10, 5*time.Millisecond) }()

	srv := httptest.NewServer(httpapi.New(rt.Bus, rt.Graph, rt.Journal).Handler())
	defer srv.Close()

	post := func(tenant, idemKey, body string) (int, httpapi.Receipt) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/work-items", strings.NewReader(body))
		req.Header.Set("X-Golem-Tenant", tenant)
		req.Header.Set("Idempotency-Key", idemKey)
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var receipt httpapi.Receipt
		if err := json.NewDecoder(resp.Body).Decode(&receipt); err != nil {
			t.Fatalf("decode receipt: %v", err)
		}
		return resp.StatusCode, receipt
	}

	// 1. Command accepted.
	code, r1 := post("t_demo", "slice-key-0001", `{"title":"Vertical slice","type":"task"}`)
	if code != http.StatusAccepted {
		t.Fatalf("create status = %d, want 202", code)
	}

	// 2. Same Idempotency-Key retried: duplicate receipt, no new events.
	code, r2 := post("t_demo", "slice-key-0001", `{"title":"Vertical slice","type":"task"}`)
	if code != http.StatusOK || !r2.Duplicate || r2.CommandID != r1.CommandID {
		t.Fatalf("retry: code=%d receipt=%+v", code, r2)
	}

	// 3. The projection catches up; the neighborhood query finds the node.
	roots := `"roots":["` + itemID(t, rt) + `"]`
	body := "{" + roots + `,"max_depth":1,"max_nodes":10,"max_edges":10}`
	var sub httpapi.Subgraph
	deadline := time.Now().Add(3 * time.Second)
	for {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/graph/neighborhood", strings.NewReader(body))
		req.Header.Set("X-Golem-Tenant", "t_demo")
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		err = json.NewDecoder(resp.Body).Decode(&sub)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("decode subgraph: %v", err)
		}
		if len(sub.Nodes) == 1 && sub.Nodes[0].Attributes["title"] == "Vertical slice" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("projection never caught up; last subgraph: %+v", sub)
		}
		time.Sleep(5 * time.Millisecond)
	}

	// 4. Tenant isolation through the wire: another tenant sees nothing.
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/graph/neighborhood", strings.NewReader(body))
	req.Header.Set("X-Golem-Tenant", "t_other")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var other httpapi.Subgraph
	if err := json.NewDecoder(resp.Body).Decode(&other); err != nil {
		t.Fatal(err)
	}
	if len(other.Nodes) != 0 {
		t.Fatalf("tenant isolation violated: %+v", other.Nodes)
	}

	// 5. Missing tenant is rejected with a problem body.
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/api/v1/work-items", strings.NewReader(`{}`))
	req.Header.Set("Idempotency-Key", "slice-key-0002")
	resp, err = srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var p httpapi.Problem
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest || p.Code != httpapi.CodeMissingTenant {
		t.Fatalf("missing tenant: status=%d problem=%+v", resp.StatusCode, p)
	}
}

func itemID(t *testing.T, rt *runtime.Runtime) string {
	t.Helper()
	evs, _, err := rt.Journal.Replay(context.Background(), 0, 0)
	if err != nil || len(evs) == 0 {
		t.Fatalf("journal replay: %d events, err %v", len(evs), err)
	}
	const prefix = "workitem:"
	return evs[0].StreamID[len(prefix):]
}
