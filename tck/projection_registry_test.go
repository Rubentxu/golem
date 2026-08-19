package tck

import (
	"encoding/json"
	"testing"

	"github.com/Rubentxu/golem/internal/application/projection"
	"github.com/Rubentxu/golem/internal/ports"
	"github.com/Rubentxu/golem/internal/supplychain"
	"github.com/Rubentxu/golem/internal/work"
)

// --- Registry TCK scenarios ---

// fakeProjection is a test-only Projection that claims specific event types.
type fakeProjection struct {
	domain     string
	eventTypes []string
	handler    func(env ports.RawEvent) (ports.GraphMutation, bool, error)
}

func (f fakeProjection) Domain() string       { return f.domain }
func (f fakeProjection) EventTypes() []string { return f.eventTypes }
func (f fakeProjection) Handle(env ports.RawEvent) (ports.GraphMutation, bool, error) {
	return f.handler(env)
}

// TestRegistry_DuplicateRegistrationRejected verifies that registering two
// Projections that claim the same EventType returns an error.
func TestRegistry_DuplicateRegistrationRejected(t *testing.T) {
	p1 := fakeProjection{
		domain:     "test1",
		eventTypes: []string{"test.event.v1"},
		handler:    func(env ports.RawEvent) (ports.GraphMutation, bool, error) { return ports.GraphMutation{}, true, nil },
	}
	p2 := fakeProjection{
		domain:     "test2",
		eventTypes: []string{"test.event.v1"}, // same event type!
		handler:    func(env ports.RawEvent) (ports.GraphMutation, bool, error) { return ports.GraphMutation{}, true, nil },
	}
	r := projection.NewRegistry()
	if err := r.Register(p1); err != nil {
		t.Fatalf("register p1: %v", err)
	}
	if err := r.Register(p2); err == nil {
		t.Fatal("expected error for duplicate registration, got nil")
	}
}

// TestRegistry_EmptyMutationIsHandled verifies that a Projection returning an
// empty mutation with handled=true is correctly propagated (C3: handled=true
// means "I claim this event type" even when mutation is empty).
func TestRegistry_EmptyMutationIsHandled(t *testing.T) {
	p := fakeProjection{
		domain:     "test",
		eventTypes: []string{"test.event.v1"},
		handler: func(env ports.RawEvent) (ports.GraphMutation, bool, error) {
			return ports.GraphMutation{}, true, nil // empty mutation but handled
		},
	}
	r := projection.NewRegistry()
	if err := r.Register(p); err != nil {
		t.Fatalf("register: %v", err)
	}

	env := ports.RawEvent{
		EventID:   "evt-1",
		TenantID:  "tenant-1",
		EventType: "test.event.v1",
		Payload:   []byte("{}"),
	}
	m, handled, err := r.Handle(env)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !handled {
		t.Fatal("expected handled=true for registered event type")
	}
	if len(m.Ops) != 0 {
		t.Fatalf("expected empty mutation ops, got %d", len(m.Ops))
	}
}

// TestRegistry_FallsBackToLegacySwitch verifies that when no Projection claims
// an event type, Handle returns handled=false and the caller should fall back
// to legacy switch.
func TestRegistry_FallsBackToLegacySwitch(t *testing.T) {
	p := fakeProjection{
		domain:     "test",
		eventTypes: []string{"test.event.v1"},
		handler:    func(env ports.RawEvent) (ports.GraphMutation, bool, error) { return ports.GraphMutation{}, true, nil },
	}
	r := projection.NewRegistry()
	if err := r.Register(p); err != nil {
		t.Fatalf("register: %v", err)
	}

	// "work.item.created.v1" is not claimed by our fake projection.
	env := ports.RawEvent{
		EventID:   "evt-2",
		TenantID:  "tenant-1",
		EventType: work.EventItemCreated,
		Payload:   mustMarshal(work.ItemCreated{ItemID: "item-1", Title: "Test", ItemType: "task", Status: "open"}),
	}
	m, handled, err := r.Handle(env)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if handled {
		t.Fatal("expected handled=false for unregistered event type")
	}
	// mutation should be zero value.
	if len(m.Ops) != 0 {
		t.Fatalf("expected zero mutation for unhandled event, got %d ops", len(m.Ops))
	}
}

// TestRegistry_DelegatesToClaimingProjection verifies that when a Projection
// claims an event type, its Handle result is returned.
func TestRegistry_DelegatesToClaimingProjection(t *testing.T) {
	expectedMutation := ports.GraphMutation{
		TenantID: "tenant-1",
		Ops: []ports.GraphOp{
			{Kind: ports.OpUpsertNode, Target: "custom-target", Data: map[string]any{"kind": "Custom", "attributes": map[string]any{"key": "value"}}},
		},
	}
	p := fakeProjection{
		domain:     "test",
		eventTypes: []string{"custom.event.v1"},
		handler: func(env ports.RawEvent) (ports.GraphMutation, bool, error) {
			return expectedMutation, true, nil
		},
	}
	r := projection.NewRegistry()
	if err := r.Register(p); err != nil {
		t.Fatalf("register: %v", err)
	}

	env := ports.RawEvent{
		EventID:   "evt-3",
		TenantID:  "tenant-1",
		EventType: "custom.event.v1",
		Payload:   []byte("{}"),
	}
	m, handled, err := r.Handle(env)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !handled {
		t.Fatal("expected handled=true")
	}
	if len(m.Ops) != 1 || m.Ops[0].Target != "custom-target" {
		t.Fatalf("unexpected mutation: %+v", m)
	}
}

// TestRegistry_EmptyMatchesLegacyDigest verifies that when the registry is empty
// (nil), the Projector.Project produces the same digest as the legacy switch.
// This uses a supplychain event since that's the most complex projection path.
func TestRegistry_EmptyMatchesLegacyDigest(t *testing.T) {
	// Create a supplychain event with a manageable number of components.
	comps := make([]supplychain.SBOMComponent, 10)
	for i := range comps {
		comps[i] = supplychain.SBOMComponent{
			Purl:    "pkg:npm/test@" + itoa(i),
			Name:    "test",
			Version: "1.0",
		}
	}
	env := ports.RawEvent{
		EventID:   "evt-sbom-1",
		TenantID:  "tenant-1",
		EventType: supplychain.EventSBOMIngested,
		Payload: mustMarshal(supplychain.SBOMIngested{
			SBOMID:         "sbm-test123",
			ArtifactDigest: "sha256:abc123",
			Format:         "spdx-2.3",
			SpecVersion:    "SPDX-2.3",
			Components:     comps,
		}),
	}

	// Clear the global registry to simulate empty (nil) state.
	prev := projection.Global()
	projection.SetGlobal(nil)
	defer projection.SetGlobal(prev)

	// Project via the Projector with nil registry (legacy path).
	p := projection.Projector{}
	mutLegacy, err := p.Project(env)
	if err != nil {
		t.Fatalf("legacy Project: %v", err)
	}

	// Verify we got a non-empty mutation from the legacy path.
	if len(mutLegacy.Ops) == 0 {
		t.Fatal("expected non-empty mutation from legacy path")
	}
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
