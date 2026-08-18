package scenario

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Rubentxu/golem/internal/clock"
	"github.com/Rubentxu/golem/internal/ids"
	"github.com/Rubentxu/golem/internal/ports"
)

// fakeJournal is a minimal in-memory ports.JournalStore (same pattern as
// internal/packs activator tests — extraction to testutil is the
// documented carry-forward DUP-M51-1).
type fakeJournal struct {
	mu     sync.Mutex
	events []ports.RawEvent
	pos    ports.StreamPosition
}

func (f *fakeJournal) Append(_ context.Context, events []ports.RawEvent) ([]ports.AppendResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []ports.AppendResult
	for _, e := range events {
		f.pos++
		f.events = append(f.events, e)
		out = append(out, ports.AppendResult{EventID: e.EventID, Position: f.pos})
	}
	return out, nil
}

func (f *fakeJournal) AppendIf(_ context.Context, expected ports.StreamVersion, events []ports.RawEvent) ([]ports.AppendResult, error) {
	return nil, fmt.Errorf("%w: not implemented", ports.ErrVersionConflict)
}

func (f *fakeJournal) ReadStream(_ context.Context, tenant ports.TenantID, streamID string, fromVersion uint64) ([]ports.RawEvent, error) {
	return nil, nil
}

func (f *fakeJournal) Replay(_ context.Context, from ports.StreamPosition, limit int) ([]ports.RawEvent, ports.StreamPosition, error) {
	return f.events, f.pos, nil
}

func (f *fakeJournal) Head(_ context.Context) (ports.StreamPosition, error) {
	return f.pos, nil
}

func (f *fakeJournal) Backup(_ context.Context) (ports.BackupHandle, error) {
	return ports.BackupHandle{}, nil
}

func (f *fakeJournal) Restore(_ context.Context, _ ports.BackupHandle) error {
	return nil
}

// S20 — promote happy path: batch + scenario.promoted.v1, lineage ok.
func TestPromote_HappyPath(t *testing.T) {
	j := &fakeJournal{pos: 7} // journal head at 7
	ts := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	overlay := json.RawMessage(`{"event_id":"evt-o1","tenant_id":"t","stream_id":"s","event_type":"evt.o"}`)
	s := &ports.Scenario{
		ID:           "scn-1",
		TenantID:     "t",
		BasePosition: 7,
		Overlay:      []json.RawMessage{overlay},
		Approved:     true,
	}

	res, err := Promote(context.Background(), j, ids.NewGenerator(clock.Fixed(ts)), clock.Fixed(ts), s)
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if res.EventsApplied != 1 {
		t.Errorf("events applied = %d, want 1", res.EventsApplied)
	}
	// batch: overlay event + promoted event
	if len(j.events) != 2 {
		t.Fatalf("journal events = %d, want 2 (overlay + promoted)", len(j.events))
	}
	promoted := j.events[1]
	if promoted.EventType != ports.EventScenarioPromoted {
		t.Errorf("last event = %q, want %q", promoted.EventType, ports.EventScenarioPromoted)
	}
	if promoted.StreamID != "scenario.scn-1" {
		t.Errorf("promoted stream = %q", promoted.StreamID)
	}
	var payload map[string]any
	if err := json.Unmarshal(promoted.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["scenario_id"] != "scn-1" || payload["events_applied"] != float64(1) {
		t.Errorf("promoted payload = %v", payload)
	}
}

// S21 — lineage conflict: base position ahead of journal head.
func TestPromote_LineageConflict(t *testing.T) {
	j := &fakeJournal{pos: 3}
	ts := time.Now()
	s := &ports.Scenario{ID: "scn-x", TenantID: "t", BasePosition: 9, Approved: true}
	_, err := Promote(context.Background(), j, ids.NewGenerator(clock.Fixed(ts)), clock.Fixed(ts), s)
	if !errors.Is(err, ErrScenarioConflict) {
		t.Fatalf("err = %v, want ErrScenarioConflict", err)
	}
	if len(j.events) != 0 {
		t.Error("journal mutated on conflict")
	}
}

// S22 — missing approval rejected without journal mutation.
func TestPromote_NoApproval(t *testing.T) {
	j := &fakeJournal{pos: 5}
	ts := time.Now()
	s := &ports.Scenario{ID: "scn-y", TenantID: "t", BasePosition: 5, Approved: false}
	_, err := Promote(context.Background(), j, ids.NewGenerator(clock.Fixed(ts)), clock.Fixed(ts), s)
	if !errors.Is(err, ErrScenarioConflict) {
		t.Fatalf("err = %v, want ErrScenarioConflict", err)
	}
	if len(j.events) != 0 {
		t.Error("journal mutated without approval")
	}
}
