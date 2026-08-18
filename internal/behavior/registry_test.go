package behavior

import (
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

func testEvent(tenant string) ports.RawEvent {
	return ports.RawEvent{EventID: "e", TenantID: tenant, EventType: "evt.a"}
}

func testBehavior(id, version, sub string) *Behavior {
	return &Behavior{ID: id, Version: version, Subscriptions: []string{sub}}
}

// S1/S2 — register by (ID, Version); two versions coexist.
func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(testBehavior("b", "1", "evt.a")); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(testBehavior("b", "2", "evt.a")); err != nil {
		t.Fatal(err)
	}
	if r.Get("b", "1") == nil || r.Get("b", "2") == nil {
		t.Fatal("both versions must resolve")
	}
	// duplicate version rejected
	if err := r.Register(testBehavior("b", "1", "evt.a")); err == nil {
		t.Error("duplicate (id,version) must be rejected")
	}
}

// S3 — subscription index returns candidates per event type.
func TestRegistryCandidates(t *testing.T) {
	r := NewRegistry()
	r.Register(testBehavior("b1", "1", "evt.a"))
	r.Register(testBehavior("b2", "1", "evt.b"))
	if got := r.Candidates("evt.a"); len(got) != 1 || got[0].ID != "b1" {
		t.Errorf("candidates(evt.a) = %+v", got)
	}
	if got := r.Candidates("evt.missing"); len(got) != 0 {
		t.Errorf("candidates(missing) = %+v, want empty", got)
	}
}

// reject: cheap predicates.
func TestRejectFilters(t *testing.T) {
	b := &Behavior{Filters: []Filter{{Field: "tenant", Op: "==", Value: "t1"}}}
	ev := testEvent("t1")
	if reject(b, ev) != "" {
		t.Error("matching filter must not reject")
	}
	b.Filters[0].Value = "t2"
	if reject(b, ev) == "" {
		t.Error("mismatched filter must reject")
	}
	b.Filters = []Filter{{Field: "nope", Op: "==", Value: "x"}}
	if reject(b, ev) == "" {
		t.Error("unknown filter field must reject")
	}
}
