package tck

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	graphmem "github.com/Rubentxu/golem/adapters/graph/memstore"
	"github.com/Rubentxu/golem/internal/behavior"
	"github.com/Rubentxu/golem/internal/clock"
	idgen "github.com/Rubentxu/golem/internal/ids"
	"github.com/Rubentxu/golem/internal/lens"
	"github.com/Rubentxu/golem/internal/ports"
)

func behaviorEvent(t *testing.T, eventType, tenant string) ports.RawEvent {
	return ports.RawEvent{
		EventID:       "evt-1",
		TenantID:      tenant,
		StreamID:      "stream-1",
		EventType:     eventType,
		SchemaVersion: 1,
		OccurredAt:    time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC),
		Payload:       json.RawMessage(`{}`),
	}
}

func newBehaviorEngine(t *testing.T, reg *behavior.Registry) *behavior.Engine {
	ts := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	return behavior.NewEngine(reg, graphmem.NewGraph(), clock.Fixed(ts))
}

// S4/S5 — matching subscription executes once; non-matching is a no-op.
func TestBehaviorPipeline_MatchAndNoOp(t *testing.T) {
	reg := behavior.NewRegistry()
	calls := 0
	b := &behavior.Behavior{
		ID: "counter", Version: "1", Subscriptions: []string{"evt.hit"},
		Handler: func(context.Context, behavior.HandlerInput) (behavior.HandlerOutput, error) {
			calls++
			return behavior.HandlerOutput{}, nil
		},
	}
	if err := reg.Register(b); err != nil {
		t.Fatal(err)
	}
	eng := newBehaviorEngine(t, reg)

	out, err := eng.Handle(context.Background(), behaviorEvent(t, "evt.hit", "t"))
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || calls != 1 {
		t.Fatalf("outcomes=%d calls=%d, want 1/1", len(out), calls)
	}
	out, err = eng.Handle(context.Background(), behaviorEvent(t, "evt.miss", "t"))
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 || calls != 1 {
		t.Fatalf("no-op violated: outcomes=%d calls=%d", len(out), calls)
	}
}

// S6 — determinism: same input twice, byte-identical outcomes.
func TestBehaviorPipeline_Deterministic(t *testing.T) {
	reg := behavior.NewRegistry()
	b := &behavior.Behavior{
		ID: "emit", Version: "1", Subscriptions: []string{"evt.x"},
		Handler: func(_ context.Context, in behavior.HandlerInput) (behavior.HandlerOutput, error) {
			ev, _ := json.Marshal(map[string]string{"k": "v"})
			return behavior.HandlerOutput{Events: []ports.RawEvent{{
				EventID: in.IDs.NewID(), TenantID: in.Event.TenantID,
				StreamID: "out", EventType: "out.v1", Payload: ev,
			}}}, nil
		},
	}
	reg.Register(b)
	eng := newBehaviorEngine(t, reg)

	a, err := eng.Handle(context.Background(), behaviorEvent(t, "evt.x", "t"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := eng.Handle(context.Background(), behaviorEvent(t, "evt.x", "t"))
	if err != nil {
		t.Fatal(err)
	}
	ja, _ := json.Marshal(a)
	jc, _ := json.Marshal(c)
	if string(ja) != string(jc) {
		t.Errorf("non-deterministic pipeline:\n%s\n%s", ja, jc)
	}
}

// S7 — filter rejects before handler execution.
func TestBehaviorPipeline_FilterRejects(t *testing.T) {
	reg := behavior.NewRegistry()
	calls := 0
	reg.Register(&behavior.Behavior{
		ID: "f", Version: "1", Subscriptions: []string{"evt.y"},
		Filters: []behavior.Filter{{Field: "tenant", Op: "==", Value: "t-allowed"}},
		Handler: func(context.Context, behavior.HandlerInput) (behavior.HandlerOutput, error) {
			calls++
			return behavior.HandlerOutput{}, nil
		},
	})
	eng := newBehaviorEngine(t, reg)
	out, err := eng.Handle(context.Background(), behaviorEvent(t, "evt.y", "t-other"))
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 || len(out) != 1 || out[0].Skipped == "" {
		t.Fatalf("filter must skip: calls=%d out=%+v", calls, out)
	}
}

// S8/S9 — relation behavior: roots derived from the event; lens with the
// incident nodes; skips when roots resolve empty.
func TestBehaviorPipeline_Relation(t *testing.T) {
	g := graphmem.NewGraph()
	_, err := g.Apply(context.Background(), ports.GraphMutation{TenantID: "t", Ops: []ports.GraphOp{
		{Kind: ports.OpUpsertNode, Target: "vuln-1", Data: map[string]any{"kind": "Vulnerability", "attributes": map[string]any{"cve": "CVE-9"}}},
	}})
	if err != nil {
		t.Fatal(err)
	}

	reg := behavior.NewRegistry()
	seen := ""
	reg.Register(&behavior.Behavior{
		ID: "rel", Version: "1", Subscriptions: []string{"vulnerability.reported.v1"},
		Relation: &behavior.RelationSpec{
			RootsFromEvent: func(ev ports.RawEvent) ([]string, error) {
				var p struct {
					VulnID string `json:"vuln_id"`
				}
				if err := json.Unmarshal(ev.Payload, &p); err != nil {
					return nil, err
				}
				if p.VulnID == "" {
					return nil, nil // empty roots → skip (S9)
				}
				return []string{p.VulnID}, nil
			},
		},
		LensSpec: &lens.Spec{Roots: []string{"placeholder"}, MaxDepth: 1, MaxNodes: 10, MaxEdges: 10, Evidence: true},
		Handler: func(_ context.Context, in behavior.HandlerInput) (behavior.HandlerOutput, error) {
			for _, n := range in.LensResult.Nodes {
				seen = n.ID
			}
			return behavior.HandlerOutput{}, nil
		},
	})

	eng := behavior.NewEngine(reg, g, clock.Fixed(time.Now()))
	ev := behaviorEvent(t, "vulnerability.reported.v1", "t")
	ev.Payload = json.RawMessage(`{"vuln_id":"vuln-1"}`)
	out, err := eng.Handle(context.Background(), ev)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || seen != "vuln-1" {
		t.Fatalf("relation behavior must see the incident node: out=%+v seen=%q", out, seen)
	}

	// Empty roots → skipped.
	ev.Payload = json.RawMessage(`{}`)
	out, err = eng.Handle(context.Background(), ev)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Skipped == "" {
		t.Fatalf("empty relation roots must skip: %+v", out)
	}
}

// Shadow primer: CveGateV1 vs CveGateV2 over the same lens result differ
// exactly by the mitigation event + proposal.
func TestCveGate_V1V2Divergence(t *testing.T) {
	g := graphmem.NewGraph()
	_, err := g.Apply(context.Background(), ports.GraphMutation{TenantID: "t", Ops: []ports.GraphOp{
		{Kind: ports.OpUpsertNode, Target: "vuln-1", Data: map[string]any{"kind": "Vulnerability", "attributes": map[string]any{"cve": "CVE-9"}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)

	run := func(h behavior.Handler) behavior.HandlerOutput {
		res, err := lens.Execute(context.Background(), g, "t",
			lens.Spec{Roots: []string{"vuln-1"}, MaxDepth: 1, MaxNodes: 10, MaxEdges: 10, Evidence: true})
		if err != nil {
			t.Fatal(err)
		}
		out, err := h(context.Background(), behavior.HandlerInput{
			Event:      behaviorEvent(t, "vulnerability.reported.v1", "t"),
			LensResult: *res,
			Clock:      clock.Fixed(ts),
			IDs:        idgen.NewGenerator(clock.Fixed(ts)),
		})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}

	v1 := run(behavior.CveGateV1)
	v2 := run(behavior.CveGateV2)
	if len(v1.Events) != 1 || len(v2.Events) != 2 {
		t.Fatalf("v1=%d v2=%d events, want 1/2", len(v1.Events), len(v2.Events))
	}
	if v2.Proposals == nil || v1.Proposals != nil {
		t.Fatalf("proposal divergence: v1=%v v2=%v", v1.Proposals, v2.Proposals)
	}
}
