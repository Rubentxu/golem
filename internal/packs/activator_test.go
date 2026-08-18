package packs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Rubentxu/golem/internal/clock"
	"github.com/Rubentxu/golem/internal/ids"
	"github.com/Rubentxu/golem/internal/ports"
)

// fakeJournal is a minimal in-memory ports.JournalStore used to exercise the
// activator without importing adapters (ADR-047). AppendIf enforces stream
// versions so idempotency semantics match the real stores.
type fakeJournal struct {
	mu      sync.Mutex
	events  []ports.RawEvent
	streams map[string]uint64 // "tenant|stream" → version (count)
	pos     ports.StreamPosition
}

func newFakeJournal() *fakeJournal {
	return &fakeJournal{streams: map[string]uint64{}}
}

func (f *fakeJournal) Append(_ context.Context, events []ports.RawEvent) ([]ports.AppendResult, error) {
	return f.append(events, false, ports.StreamVersion{})
}

func (f *fakeJournal) AppendIf(_ context.Context, expected ports.StreamVersion, events []ports.RawEvent) ([]ports.AppendResult, error) {
	return f.append(events, true, expected)
}

func (f *fakeJournal) append(events []ports.RawEvent, conditional bool, expected ports.StreamVersion) ([]ports.AppendResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := fmt.Sprintf("%s|%s", expected.TenantID, expected.StreamID)
	if conditional {
		if got := f.streams[key]; got != expected.Version {
			return nil, fmt.Errorf("%w: stream %s at %d, expected %d",
				ports.ErrVersionConflict, expected.StreamID, got, expected.Version)
		}
	}
	var results []ports.AppendResult
	for _, e := range events {
		f.pos++
		f.events = append(f.events, e)
		streamKey := fmt.Sprintf("%s|%s", e.TenantID, e.StreamID)
		f.streams[streamKey]++
		results = append(results, ports.AppendResult{EventID: e.EventID, Position: f.pos})
	}
	return results, nil
}

func (f *fakeJournal) ReadStream(_ context.Context, tenant ports.TenantID, streamID string, fromVersion uint64) ([]ports.RawEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []ports.RawEvent
	for _, e := range f.events {
		if ports.TenantID(e.TenantID) == tenant && e.StreamID == streamID && uint64(e.SchemaVersion) >= fromVersion {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeJournal) Replay(_ context.Context, from ports.StreamPosition, limit int) ([]ports.RawEvent, ports.StreamPosition, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []ports.RawEvent
	for _, e := range f.events {
		if ports.StreamPosition(len(out)) >= ports.StreamPosition(limit) && limit > 0 {
			break
		}
		out = append(out, e)
	}
	return out, f.pos, nil
}

func (f *fakeJournal) Head(_ context.Context) (ports.StreamPosition, error) {
	return f.pos, nil
}

// validManifestBytes returns a canonical, digest-correct manifest for tests.
func validManifest(t *testing.T, mutate func(*Manifest)) (*Manifest, []byte) {
	t.Helper()
	m := &Manifest{
		FormatVersion:   "1",
		Name:            "demo-pack",
		Version:         "1.0.0",
		GolemAPI:        ">=0.5 <0.6",
		Capabilities:    []string{"graph.read"},
		Permissions:     []string{"scm.read"},
		IntegrityDigest: "", // computed below
	}
	if mutate != nil {
		mutate(m)
	}
	digest, err := m.ComputeDigest()
	if err != nil {
		t.Fatalf("ComputeDigest: %v", err)
	}
	m.IntegrityDigest = digest
	// Serialize NON-canonically (different key order via map round-trip is
	// overkill; standard json.Marshal is fine — DigestMatchesCanonical
	// re-canonicalises on verify).
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return m, raw
}

func newTestActivator(journal ports.JournalStore) *Activator {
	ts := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	return NewActivator(journal, ids.NewGenerator(clock.Fixed(ts)), clock.Fixed(ts), NewPermissionCatalogV1())
}

// S8 — activation happy path journals exactly one event.
func TestActivate_HappyPath(t *testing.T) {
	j := newFakeJournal()
	a := newTestActivator(j)
	m, raw := validManifest(t, nil)

	res, err := a.Activate(context.Background(), "tenant-1", m, raw)
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if res.EventID == "" || res.Position == 0 {
		t.Errorf("unexpected result %+v", res)
	}

	events, err := j.ReadStream(context.Background(), "tenant-1", PackStreamID("tenant-1", "demo-pack"), 0)
	if err != nil {
		t.Fatalf("ReadStream: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	e := events[0]
	if e.EventType != ports.EventExtensionPackActivated {
		t.Errorf("EventType = %q, want %q", e.EventType, ports.EventExtensionPackActivated)
	}
	if !strings.HasPrefix(e.EventType, ports.ReservedEventPrefixExtensionPack) {
		t.Errorf("event %q does not use reserved prefix", e.EventType)
	}
	if e.TenantID != "tenant-1" {
		t.Errorf("TenantID = %q, want tenant-1", e.TenantID)
	}
	if e.Actor.Type != "service" || e.Actor.ID != "pack-activator" {
		t.Errorf("Actor = %+v", e.Actor)
	}
}

// S9 — payload shape Q7: identity data only, no envelope duplication.
func TestActivate_PayloadShape(t *testing.T) {
	j := newFakeJournal()
	a := newTestActivator(j)
	m, raw := validManifest(t, nil)

	if _, err := a.Activate(context.Background(), "t", m, raw); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	events, _ := j.ReadStream(context.Background(), "t", PackStreamID("t", "demo-pack"), 0)
	if len(events) != 1 {
		t.Fatalf("expected 1 event")
	}
	var p map[string]any
	if err := json.Unmarshal(events[0].Payload, &p); err != nil {
		t.Fatalf("payload decode: %v", err)
	}
	for _, key := range []string{"name", "version", "integrity_digest", "capabilities_required", "permissions"} {
		if _, ok := p[key]; !ok {
			t.Errorf("payload missing %q: %s", key, events[0].Payload)
		}
	}
	for _, absent := range []string{"tenant_id", "occurred_at", "actor"} {
		if _, ok := p[absent]; ok {
			t.Errorf("payload must NOT duplicate envelope field %q (Q7)", absent)
		}
	}
	if p["name"] != "demo-pack" {
		t.Errorf("payload name = %v", p["name"])
	}
}

// S10 — re-activation is rejected as ErrPackAlreadyActivated (journal dedupe).
func TestActivate_IdempotencyRejected(t *testing.T) {
	j := newFakeJournal()
	a := newTestActivator(j)
	m, raw := validManifest(t, nil)

	if _, err := a.Activate(context.Background(), "t1", m, raw); err != nil {
		t.Fatalf("first Activate: %v", err)
	}
	_, err := a.Activate(context.Background(), "t1", m, raw)
	if !errors.Is(err, ports.ErrPackAlreadyActivated) {
		t.Fatalf("second Activate err = %v, want ErrPackAlreadyActivated", err)
	}
	// Exactly one event despite two calls.
	events, _ := j.ReadStream(context.Background(), "t1", PackStreamID("t1", "demo-pack"), 0)
	if len(events) != 1 {
		t.Errorf("expected 1 event after re-activation attempt, got %d", len(events))
	}
}

// S11 — cross-tenant activation of the same pack is independent (Q11).
func TestActivate_CrossTenant(t *testing.T) {
	j := newFakeJournal()
	a := newTestActivator(j)
	m, raw := validManifest(t, nil)

	for _, tenant := range []ports.TenantID{"t1", "t2"} {
		if _, err := a.Activate(context.Background(), tenant, m, raw); err != nil {
			t.Fatalf("Activate %s: %v", tenant, err)
		}
	}
	for _, tenant := range []ports.TenantID{"t1", "t2"} {
		events, _ := j.ReadStream(context.Background(), tenant, PackStreamID(tenant, "demo-pack"), 0)
		if len(events) != 1 {
			t.Errorf("tenant %s: expected 1 event, got %d", tenant, len(events))
		}
	}
}

// S12 — digest mismatch is rejected before any journal mutation.
func TestActivate_DigestMismatch(t *testing.T) {
	j := newFakeJournal()
	a := newTestActivator(j)
	m, raw := validManifest(t, nil)
	m.IntegrityDigest = strings.Repeat("b", 64) // syntactically valid, wrong

	_, err := a.Activate(context.Background(), "t", m, raw)
	if !errors.Is(err, ports.ErrPackIntegrityFailed) {
		t.Fatalf("err = %v, want ErrPackIntegrityFailed", err)
	}
	if h, _ := j.Head(context.Background()); h != 0 {
		t.Errorf("journal mutated on failed activation: head=%d", h)
	}
}

// S13 — migrations declared → ErrUnsupportedInM51, no journal mutation.
func TestActivate_MigrationsRejected(t *testing.T) {
	j := newFakeJournal()
	a := newTestActivator(j)
	m, raw := validManifest(t, func(m *Manifest) {
		m.Migrations = []Migration{{ID: "001", Description: "d", Script: "s"}}
	})
	// digest was computed over the mutated manifest — raw matches, gate fires.

	_, err := a.Activate(context.Background(), "t", m, raw)
	if !errors.Is(err, ports.ErrUnsupportedInM51) {
		t.Fatalf("err = %v, want ErrUnsupportedInM51", err)
	}
	if h, _ := j.Head(context.Background()); h != 0 {
		t.Errorf("journal mutated: head=%d", h)
	}
}

// S13b — UI contributions declared → ErrUnsupportedInM51.
func TestActivate_UIRejected(t *testing.T) {
	j := newFakeJournal()
	a := newTestActivator(j)
	m, raw := validManifest(t, func(m *Manifest) {
		m.UI = []UIContribution{{Type: "dashboard", Path: "ui/x.json", Label: "X"}}
	})

	_, err := a.Activate(context.Background(), "t", m, raw)
	if !errors.Is(err, ports.ErrUnsupportedInM51) {
		t.Fatalf("err = %v, want ErrUnsupportedInM51", err)
	}
}

// S14 — reordered raw manifest bytes verify the same digest (Q3 determinism).
func TestActivate_ReorderedKeysVerify(t *testing.T) {
	j := newFakeJournal()
	a := newTestActivator(j)
	m, _ := validManifest(t, nil)
	reordered := []byte(`{"permissions":["scm.read"],` +
		`"golem_api":">=0.5 <0.6","format_version":"1",` +
		`"name":"demo-pack","version":"1.0.0",` +
		`"capabilities":["graph.read"],` +
		`"integrity_digest":"` + m.IntegrityDigest + `"}`)

	if _, err := a.Activate(context.Background(), "t", m, reordered); err != nil {
		t.Fatalf("Activate with reordered raw bytes: %v", err)
	}
}

// S15/S16 — Status returns activation or nil.
func TestStatus(t *testing.T) {
	j := newFakeJournal()
	a := newTestActivator(j)
	m, raw := validManifest(t, nil)

	got, err := a.Status(context.Background(), "t", "demo-pack")
	if err != nil || got != nil {
		t.Fatalf("Status before activation = (%v, %v), want (nil, nil)", got, err)
	}
	if _, err := a.Activate(context.Background(), "t", m, raw); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	got, err = a.Status(context.Background(), "t", "demo-pack")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if got == nil || got.Name != "demo-pack" || got.Version != "1.0.0" {
		t.Errorf("Status = %+v", got)
	}
	if got.IntegrityDigest != m.IntegrityDigest {
		t.Errorf("Status digest = %s, want %s", got.IntegrityDigest, m.IntegrityDigest)
	}
}
