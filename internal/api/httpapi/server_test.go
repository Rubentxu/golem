package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Rubentxu/golem/internal/application/command"
	"github.com/Rubentxu/golem/internal/ports"
)

// fake command submitter capturing the command and returning a canned
// receipt or an error.
type fakeCommands struct {
	got  command.Command
	repl func(cmd command.Command) (ports.CommandReceipt, error)
}

func (f *fakeCommands) Submit(_ context.Context, cmd command.Command) (ports.CommandReceipt, error) {
	f.got = cmd
	return f.repl(cmd)
}

type fakeGraph struct {
	query ports.NeighborhoodQuery
	sub   ports.Subgraph
	err   error
}

func (f *fakeGraph) Neighborhood(_ context.Context, q ports.NeighborhoodQuery) (ports.Subgraph, error) {
	f.query = q
	return f.sub, f.err
}

func (f *fakeGraph) GetNode(_ context.Context, _ ports.TenantID, _ string) (ports.Node, error) {
	if len(f.sub.Nodes) == 0 {
		return ports.Node{}, ports.ErrNodeNotFound
	}
	return f.sub.Nodes[0], nil
}

type fakeStreams struct {
	events []ports.RawEvent
}

func (f *fakeStreams) ReadStream(_ context.Context, _ ports.TenantID, _ string, _ uint64) ([]ports.RawEvent, error) {
	return f.events, nil
}

func do(t *testing.T, h http.Handler, method, target, tenant, idemKey, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if tenant != "" {
		req.Header.Set("X-Golem-Tenant", tenant)
	}
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
}

func TestCreateWorkItemValidation(t *testing.T) {
	h := New(&fakeCommands{}, &fakeGraph{}, &fakeStreams{}).Handler()

	cases := []struct {
		name   string
		tenant string
		idem   string
		body   string
		status int
		code   string
	}{
		{"missing tenant", "", "idem-key-1", `{}`, http.StatusBadRequest, CodeMissingTenant},
		{"missing idempotency key", "t1", "", `{}`, http.StatusBadRequest, CodeInvalidArgument},
		{"short idempotency key", "t1", "short", `{}`, http.StatusBadRequest, CodeInvalidArgument},
		{"invalid json", "t1", "idem-key-1", `{`, http.StatusBadRequest, CodeInvalidArgument},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := do(t, h, http.MethodPost, "/api/v1/work-items", c.tenant, c.idem, c.body)
			if rec.Code != c.status {
				t.Fatalf("status = %d, want %d", rec.Code, c.status)
			}
			var p Problem
			decode(t, rec, &p)
			if p.Code != c.code {
				t.Fatalf("code = %s, want %s", p.Code, c.code)
			}
		})
	}
}

func TestCreateWorkItemHappyPath(t *testing.T) {
	cmds := &fakeCommands{repl: func(c command.Command) (ports.CommandReceipt, error) {
		return ports.CommandReceipt{CommandID: c.CommandID, TenantID: c.TenantID, EventIDs: []string{"ev1"}, Position: 7}, nil
	}}
	h := New(cmds, &fakeGraph{}, &fakeStreams{}).Handler()

	rec := do(t, h, http.MethodPost, "/api/v1/work-items", "t1", "client-key-42", `{"title":"Slice","type":"task"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
	var r Receipt
	decode(t, rec, &r)
	if r.CommandID != "client-key-42" || r.Position != 7 || len(r.EventIDs) != 1 {
		t.Fatalf("receipt = %+v", r)
	}
	if cmds.got.TenantID != "t1" || cmds.got.Actor.ID != "anonymous" {
		t.Fatalf("command wiring = %+v", cmds.got)
	}
	if cmds.got.CorrelationID != "corr-1" && cmds.got.CorrelationID != "" {
		t.Fatalf("unexpected correlation: %q", cmds.got.CorrelationID)
	}
}

func TestNeighborhoodBoundsEnforced(t *testing.T) {
	h := New(&fakeCommands{}, &fakeGraph{}, &fakeStreams{}).Handler()

	cases := []struct {
		name   string
		tenant string
		body   string
		code   string
	}{
		{"missing tenant", "", `{"roots":["a"],"max_depth":1,"max_nodes":1,"max_edges":1}`, CodeMissingTenant},
		{"no roots", "t1", `{"roots":[],"max_depth":1,"max_nodes":1,"max_edges":1}`, CodeUnboundedQuery},
		{"zero depth", "t1", `{"roots":["a"],"max_depth":0,"max_nodes":1,"max_edges":1}`, CodeUnboundedQuery},
		{"missing limits", "t1", `{"roots":["a"]}`, CodeUnboundedQuery},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := do(t, h, http.MethodPost, "/api/v1/graph/neighborhood", c.tenant, "", c.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			var p Problem
			decode(t, rec, &p)
			if p.Code != c.code {
				t.Fatalf("code = %s, want %s", p.Code, c.code)
			}
		})
	}
}

func TestNeighborhoodReturnsSubgraphDTO(t *testing.T) {
	g := &fakeGraph{sub: ports.Subgraph{
		Nodes: []ports.Node{{ID: "wi-1", Kind: "WorkItem", Revision: 1, Attributes: map[string]any{"title": "t"}}},
	}}
	h := New(&fakeCommands{}, g, &fakeStreams{}).Handler()

	rec := do(t, h, http.MethodPost, "/api/v1/graph/neighborhood", "t1", "",
		`{"roots":["wi-1"],"max_depth":1,"max_nodes":10,"max_edges":10}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var sub Subgraph
	decode(t, rec, &sub)
	if len(sub.Nodes) != 1 || sub.Nodes[0].Kind != "WorkItem" || sub.Nodes[0].Attributes["title"] != "t" {
		t.Fatalf("subgraph = %+v", sub)
	}
	if g.query.TenantID != "t1" || g.query.MaxDepth != 1 {
		t.Fatalf("query wiring = %+v", g.query)
	}
}
