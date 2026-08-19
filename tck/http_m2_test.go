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

// TestM2SliceRequirementsAndConcurrency covers the M2 additions over
// real HTTP: requirements with Requirement→Work traceability via links,
// GET with ETag, optimistic updates with If-Match and 409 on conflict.
func TestM2SliceRequirementsAndConcurrency(t *testing.T) {
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
	rt.Bus.Register(appwork.CmdLinkWorkItems, appwork.LinkWorkItemsHandler(rt.Graph))
	rt.Bus.Register(appreq.CmdCreateRequirement, appreq.CreateRequirementHandler(rt.IDs))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = rt.Run(ctx, 10, 5*time.Millisecond) }()

	srv := httptest.NewServer(httpapi.New(rt.Bus, rt.Graph, rt.Journal).Handler())
	defer srv.Close()

	client := srv.Client()
	post := func(path, idemKey, body string, hdr map[string]string) *http.Response {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, srv.URL+path, strings.NewReader(body))
		req.Header.Set("X-Golem-Tenant", "t_m2")
		if idemKey != "" {
			req.Header.Set("Idempotency-Key", idemKey)
		}
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	decodeTo := func(resp *http.Response, v any) {
		t.Helper()
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			t.Fatalf("decode %s: %v", resp.Status, err)
		}
	}
	waitProjected := func(kind, id string) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for {
			sub, err := rt.Graph.Neighborhood(ctx, ports.NeighborhoodQuery{
				TenantID: "t_m2", Roots: []string{id}, MaxDepth: 1, MaxNodes: 1, MaxEdges: 1,
			})
			if err == nil && len(sub.Nodes) == 1 && sub.Nodes[0].Kind == kind {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s %s never projected", kind, id)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}

	// 1. Create one requirement and two work items.
	resp := post("/api/v1/requirements", "req-key-0001", `{"title":"Traceability","statement":"Req→Work trace"}`, nil)
	var reqReceipt httpapi.Receipt
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("requirement: %s", resp.Status)
	}
	decodeTo(resp, &reqReceipt)

	var workReceipt httpapi.Receipt
	resp = post("/api/v1/work-items", "work-key-0001", `{"title":"Implement trace","type":"task"}`, nil)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("work item: %s", resp.Status)
	}
	decodeTo(resp, &workReceipt)

	// IDs recoverable from journal streams.
	evs, _, _ := rt.Journal.Replay(ctx, 0, 0)
	ids := map[string]string{}
	for _, e := range evs {
		switch {
		case strings.HasPrefix(e.StreamID, "requirement:"):
			ids["req"] = e.StreamID[len("requirement:"):]
		case strings.HasPrefix(e.StreamID, "workitem:"):
			if _, ok := ids["work"]; !ok {
				ids["work"] = e.StreamID[len("workitem:"):]
			}
		}
	}
	waitProjected("Requirement", ids["req"])
	waitProjected("WorkItem", ids["work"])

	// 2. Link work item → requirement (IMPLEMENTS = Requirement→Work trace).
	resp = post("/api/v1/work-items/"+ids["work"]+"/links", "link-key-0001",
		`{"to_id":"`+ids["req"]+`","relation":"implements"}`, nil)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("link: %s", resp.Status)
	}
	resp.Body.Close()

	// Invalid relations are rejected with the canonical ontology.
	resp = post("/api/v1/work-items/"+ids["work"]+"/links", "link-key-0002",
		`{"to_id":"`+ids["req"]+`","relation":"hates"}`, nil)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("invalid relation: %s", resp.Status)
	}
	resp.Body.Close()

	// 3. GET the work item: ETag exposes the stream version.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/work-items/"+ids["work"], nil)
	req.Header.Set("X-Golem-Tenant", "t_m2")
	getResp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var item struct {
		ETagAttrs struct {
			Version uint64 `json:"stream_version"`
		} `json:"-"`
		Version uint64 `json:"stream_version"`
	}
	etag := getResp.Header.Get("ETag")
	decodeTo(getResp, &item)
	_ = item
	// The stream holds created + linked events, so the version is 2.
	if etag != `"2"` {
		t.Fatalf("ETag = %q, want \"2\" (created + linked)", etag)
	}

	// 4. PATCH with correct If-Match: accepted.
	req, _ = http.NewRequest(http.MethodPatch, srv.URL+"/api/v1/work-items/"+ids["work"], strings.NewReader(`{"status":"in_progress"}`))
	req.Header.Set("X-Golem-Tenant", "t_m2")
	req.Header.Set("Idempotency-Key", "upd-key-0001")
	req.Header.Set("If-Match", etag)
	patchResp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if patchResp.StatusCode != http.StatusAccepted {
		t.Fatalf("patch: %s", patchResp.Status)
	}
	patchResp.Body.Close()

	// 5. PATCH with the stale If-Match: 409 revision conflict.
	req, _ = http.NewRequest(http.MethodPatch, srv.URL+"/api/v1/work-items/"+ids["work"], strings.NewReader(`{"status":"done"}`))
	req.Header.Set("X-Golem-Tenant", "t_m2")
	req.Header.Set("Idempotency-Key", "upd-key-0002")
	req.Header.Set("If-Match", etag)
	conflictResp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var problem httpapi.Problem
	decodeTo(conflictResp, &problem)
	if conflictResp.StatusCode != http.StatusConflict || problem.Code != httpapi.CodeRevisionConflict {
		t.Fatalf("conflict: %s %+v", conflictResp.Status, problem)
	}

	// 6. Unknown entity on GET: 404 problem.
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/api/v1/work-items/ghost", nil)
	req.Header.Set("X-Golem-Tenant", "t_m2")
	notFound, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	decodeTo(notFound, &problem)
	if notFound.StatusCode != http.StatusNotFound || problem.Code != httpapi.CodeNotFound {
		t.Fatalf("not found: %s %+v", notFound.Status, problem)
	}
}

// TestDynamicSchemasAndWorkflows covers configurable work items (M2):
// register a WorkType with custom fields and a workflow, then create a
// typed item (field validation, initial state), follow valid transitions
// and get 422 on invalid ones. Untyped items keep free-form status.
func TestDynamicSchemasAndWorkflows(t *testing.T) {
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
	rt.Bus.Register(appwork.CmdRegisterWorkType, appwork.RegisterWorkTypeHandler())
	rt.Bus.Register(appwork.CmdCreateWorkItem, appwork.CreateWorkItemHandler(rt.IDs, appwork.NewWorkItemReaderOverGraphStore(rt.Graph)))
	rt.Bus.Register(appwork.CmdUpdateWorkItem, appwork.UpdateWorkItemHandler(rt.Journal, appwork.NewWorkItemReaderOverGraphStore(rt.Graph)))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = rt.Run(ctx, 10, 5*time.Millisecond) }()

	srv := httptest.NewServer(httpapi.New(rt.Bus, rt.Graph, rt.Journal).Handler())
	defer srv.Close()
	client := srv.Client()

	do := func(method, path, idemKey, body string) *http.Response {
		t.Helper()
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		req.Header.Set("X-Golem-Tenant", "t_types")
		if idemKey != "" {
			req.Header.Set("Idempotency-Key", idemKey)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	// 1. Register the type: schema (priority required, estimate optional)
	// and workflow open→in_progress→done plus open→done.
	typeDef := `{
		"name": "task",
		"initial": "open",
		"states": ["open", "in_progress", "done"],
		"transitions": [{"from":"open","to":"in_progress"},{"from":"in_progress","to":"done"},{"from":"open","to":"done"}],
		"fields": [{"name":"priority","type":"string","required":true},{"name":"estimate","type":"number","required":false}]
	}`
	if resp := do(http.MethodPost, "/api/v1/work-types", "type-key-0001", typeDef); resp.StatusCode != http.StatusAccepted {
		t.Fatalf("register type: %s", resp.Status)
	} else {
		resp.Body.Close()
	}

	// Invalid definitions are rejected: transition referencing ghost state.
	badDef := `{"name":"bad","initial":"a","states":["a"],"transitions":[{"from":"a","to":"zz"}],"fields":[]}`
	if resp := do(http.MethodPost, "/api/v1/work-types", "type-key-0002", badDef); resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("invalid def: %s", resp.Status)
	} else {
		resp.Body.Close()
	}

	// Wait until the type is projected.
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := rt.Graph.GetNode(ctx, "t_types", "task"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("work type never projected")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// GET the definition back.
	if resp := do(http.MethodGet, "/api/v1/work-types/task", "", ""); resp.StatusCode != http.StatusOK {
		t.Fatalf("get type: %s", resp.Status)
	} else {
		resp.Body.Close()
	}

	// 2. Typed item without the required field: 422.
	if resp := do(http.MethodPost, "/api/v1/work-items", "item-key-0001",
		`{"title":"No priority","type":"task","type_name":"task"}`); resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("missing required field: %s", resp.Status)
	} else {
		resp.Body.Close()
	}

	// 3. Typed item with wrong field type: 422.
	if resp := do(http.MethodPost, "/api/v1/work-items", "item-key-0002",
		`{"title":"Bad number","type":"task","type_name":"task","fields":{"priority":"high","estimate":"lots"}}`); resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("wrong field type: %s", resp.Status)
	} else {
		resp.Body.Close()
	}

	// 4. Valid typed item: accepted, starts at workflow initial "open".
	var itemID string
	resp := do(http.MethodPost, "/api/v1/work-items", "item-key-0003",
		`{"title":"Typed item","type":"task","type_name":"task","fields":{"priority":"high","estimate":3}}`)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("typed item: %s", resp.Status)
	}
	resp.Body.Close()
	evs, _, _ := rt.Journal.Replay(ctx, 0, 0)
	for _, e := range evs {
		if strings.HasPrefix(e.StreamID, "workitem:") && strings.Contains(string(e.Payload), "Typed item") {
			itemID = e.StreamID[len("workitem:"):]
		}
	}
	if itemID == "" {
		t.Fatal("typed item not journaled")
	}

	// Wait for projection, then assert initial state + custom fields.
	for {
		n, err := rt.Graph.GetNode(ctx, "t_types", itemID)
		if err == nil {
			if n.Attributes["status"] != "open" {
				t.Fatalf("initial status = %v, want open", n.Attributes["status"])
			}
			if n.Attributes["field_priority"] != "high" {
				t.Fatalf("custom field = %v", n.Attributes["field_priority"])
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("typed item never projected")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// 5. Invalid transition (done→open is not declared): 422.
	if resp := do(http.MethodPatch, "/api/v1/work-items/"+itemID, "upd-key-0001", `{"status":"done"}`); resp.StatusCode != http.StatusAccepted {
		t.Fatalf("open→done: %s", resp.Status)
	} else {
		resp.Body.Close()
	}
	// Wait until the status update projects, then attempt an invalid one.
	for {
		n, err := rt.Graph.GetNode(ctx, "t_types", itemID)
		if err == nil && n.Attributes["status"] == "done" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("status update never projected")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if resp := do(http.MethodPatch, "/api/v1/work-items/"+itemID, "upd-key-0002", `{"status":"open"}`); resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("done→open should be 422: %s", resp.Status)
	} else {
		var p httpapi.Problem
		_ = json.NewDecoder(resp.Body).Decode(&p)
		resp.Body.Close()
		if p.Code != httpapi.CodeDomainRejection {
			t.Fatalf("problem = %+v", p)
		}
	}
}
