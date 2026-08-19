package command

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// ---- fakes (application tests run against fake ports, TEST_STRATEGY) ----

type fakeRegistry struct {
	receipts map[string]ports.CommandReceipt
}

func (f *fakeRegistry) Find(_ context.Context, id string) (ports.CommandReceipt, bool, error) {
	r, ok := f.receipts[id]
	return r, ok, nil
}

func (f *fakeRegistry) Save(_ context.Context, r ports.CommandReceipt) error {
	if _, ok := f.receipts[r.CommandID]; ok {
		return ports.ErrDuplicateCommand
	}
	f.receipts[r.CommandID] = r
	return nil
}

type fakeClock struct{ t time.Time }

func (c fakeClock) Now() time.Time { return c.t }

type fakeIDs struct{ n int }

func (g *fakeIDs) NewID() string {
	g.n++
	return "ID" + strings.Repeat("0", 22) + string(rune('0'+g.n))
}

func newBusWithJournal(t *testing.T) (*Bus, *[]ports.RawEvent) {
	t.Helper()
	events := &[]ports.RawEvent{}
	journal := journalFunc(func(_ context.Context, evs []ports.RawEvent) ([]ports.AppendResult, error) {
		for _, e := range evs {
			*events = append(*events, e)
		}
		out := make([]ports.AppendResult, len(evs))
		for i := range evs {
			out[i] = ports.AppendResult{EventID: evs[i].EventID, Position: ports.StreamPosition(len(*events))}
		}
		return out, nil
	})
	bus := NewBus(journalFunc(journal), &fakeRegistry{receipts: map[string]ports.CommandReceipt{}}, &fakeIDs{}, fakeClock{t: time.Unix(1_700_000_000, 0)})
	return bus, events
}

type journalFunc func(ctx context.Context, events []ports.RawEvent) ([]ports.AppendResult, error)

func (f journalFunc) Append(ctx context.Context, e []ports.RawEvent) ([]ports.AppendResult, error) {
	return f(ctx, e)
}
func (f journalFunc) AppendIf(ctx context.Context, _ ports.StreamVersion, e []ports.RawEvent) ([]ports.AppendResult, error) {
	return f(ctx, e)
}
func (f journalFunc) ReadStream(ctx context.Context, _ ports.TenantID, _ string, _ uint64) ([]ports.RawEvent, error) {
	return nil, nil
}
func (f journalFunc) Replay(ctx context.Context, _ ports.StreamPosition, _ int) ([]ports.RawEvent, ports.StreamPosition, error) {
	return nil, 0, nil
}
func (f journalFunc) Head(context.Context) (ports.StreamPosition, error) {
	return 0, nil
}
func (f journalFunc) Backup(context.Context) (ports.BackupHandle, error) {
	return ports.BackupHandle{}, nil
}
func (f journalFunc) Restore(context.Context, ports.BackupHandle) error {
	return nil
}

var okHandler Handler = func(_ context.Context, _ Command) ([]EventDraft, error) {
	return []EventDraft{{
		EventType:     "work.item.created.v1",
		StreamID:      "workitem:wi-1",
		SchemaVersion: 1,
		Payload:       map[string]any{"item_id": "wi-1"},
	}}, nil
}

func validCmd() Command {
	return Command{
		Name:     "work.create-work-item",
		TenantID: "t_test",
		Actor:    ports.Actor{Type: "user", ID: "u_1"},
		Payload:  map[string]any{},
	}
}

// ---- tests ----

func TestSubmitWiresEnvelopeMetadata(t *testing.T) {
	bus, events := newBusWithJournal(t)
	bus.Register("work.create-work-item", okHandler)

	receipt, err := bus.Submit(context.Background(), validCmd())
	if err != nil {
		t.Fatal(err)
	}
	if len(*events) != 1 {
		t.Fatalf("journaled %d events, want 1", len(*events))
	}
	env := (*events)[0]
	if env.CommandID != receipt.CommandID {
		t.Fatalf("envelope command_id = %q, receipt = %q", env.CommandID, receipt.CommandID)
	}
	if env.CorrelationID == "" {
		t.Fatal("correlation id not generated")
	}
	if env.TenantID != "t_test" || env.Actor.ID != "u_1" {
		t.Fatalf("tenant/actor not propagated: %+v", env)
	}
	if env.OccurredAt.Unix() != 1_700_000_000 {
		t.Fatalf("occurred_at not from injected clock: %v", env.OccurredAt)
	}
	var payload map[string]any
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("payload not JSON: %v", err)
	}
}

func TestSubmitValidatesInput(t *testing.T) {
	bus, _ := newBusWithJournal(t)
	bus.Register("work.create-work-item", okHandler)

	noName := validCmd()
	noName.Name = ""
	if _, err := bus.Submit(context.Background(), noName); !errors.Is(err, ErrEmptyName) {
		t.Fatalf("err = %v, want ErrEmptyName", err)
	}
	noTenant := validCmd()
	noTenant.TenantID = ""
	if _, err := bus.Submit(context.Background(), noTenant); !errors.Is(err, ports.ErrEmptyTenant) {
		t.Fatalf("err = %v, want ErrEmptyTenant", err)
	}
	noActor := validCmd()
	noActor.Actor = ports.Actor{}
	if _, err := bus.Submit(context.Background(), noActor); !errors.Is(err, ports.ErrEmptyActor) {
		t.Fatalf("err = %v, want ErrEmptyActor", err)
	}
	unknown := validCmd()
	unknown.Name = "nope"
	if _, err := bus.Submit(context.Background(), unknown); !errors.Is(err, ErrUnknownCommand) {
		t.Fatalf("err = %v, want ErrUnknownCommand", err)
	}
}

func TestDomainRejectionJournalsNothing(t *testing.T) {
	bus, events := newBusWithJournal(t)
	rejected := errors.New("domain says no")
	bus.Register("work.create-work-item", func(_ context.Context, _ Command) ([]EventDraft, error) {
		return nil, rejected
	})
	if _, err := bus.Submit(context.Background(), validCmd()); !errors.Is(err, rejected) {
		t.Fatalf("err = %v, want domain error", err)
	}
	if len(*events) != 0 {
		t.Fatalf("rejected command journaled %d events", len(*events))
	}
}

func TestSubmitIsIdempotentByCommandID(t *testing.T) {
	bus, events := newBusWithJournal(t)
	bus.Register("work.create-work-item", okHandler)

	cmd := validCmd()
	cmd.CommandID = "cmd-fixed-1"
	r1, err := bus.Submit(context.Background(), cmd)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := bus.Submit(context.Background(), cmd)
	if err != nil {
		t.Fatal(err)
	}
	if !r2.Duplicate {
		t.Fatal("second submit not flagged duplicate")
	}
	if r1.CommandID != r2.CommandID || len(r2.EventIDs) != len(r1.EventIDs) || r2.Position != r1.Position {
		t.Fatalf("receipts differ: %+v vs %+v", r1, r2)
	}
	if len(*events) != 1 {
		t.Fatalf("journal has %d events after retry, want 1", len(*events))
	}
}

// TestBus_BatchesMultipleEventsPerCommand verifies that a command handler which
// produces N EventDrafts results in exactly one journal.Append call with all N
// envelopes (not N separate calls). Batching matters for throughput: ADR-087
// showed that journal batch_size=50 yields ~4446 ops/s; per-event append
// destroys that gain. This test documents the batching invariant.
func TestBus_BatchesMultipleEventsPerCommand(t *testing.T) {
	appendCallCount := 0
	var capturedEnvelopes []ports.RawEvent

	journal := journalFunc(func(_ context.Context, events []ports.RawEvent) ([]ports.AppendResult, error) {
		appendCallCount++
		capturedEnvelopes = append(capturedEnvelopes, events...)
		out := make([]ports.AppendResult, len(events))
		for i := range events {
			out[i] = ports.AppendResult{EventID: events[i].EventID, Position: ports.StreamPosition(i + 1)}
		}
		return out, nil
	})

	bus := NewBus(journal, &fakeRegistry{receipts: map[string]ports.CommandReceipt{}}, &fakeIDs{}, fakeClock{t: time.Unix(1_700_000_000, 0)})

	// Handler produces 3 events: this simulates a command that has multiple
	// domain consequences (e.g. a work item creation with automatic link
	// creation and notification events).
	handler := Handler(func(_ context.Context, _ Command) ([]EventDraft, error) {
		return []EventDraft{
			{EventType: "work.item.created.v1", StreamID: "workitem:wi-1", SchemaVersion: 1, Payload: map[string]any{"item_id": "wi-1"}},
			{EventType: "work.item.linked.v1", StreamID: "workitem:wi-1", SchemaVersion: 1, Payload: map[string]any{"from": "wi-1", "to": "req-1", "rel": "satisfies"}},
			{EventType: "work.notification.sent.v1", StreamID: "workitem:wi-1", SchemaVersion: 1, Payload: map[string]any{"msg": "created"}},
		}, nil
	})
	bus.Register("test.multi-event", handler)

	receipt, err := bus.Submit(context.Background(), Command{
		Name:     "test.multi-event",
		TenantID: "t_test",
		Actor:    ports.Actor{Type: "user", ID: "u_1"},
	})
	if err != nil {
		t.Fatalf("Submit error: %v", err)
	}

	// ADR-087 batching invariant: one command → one Append call, not per-event.
	if appendCallCount != 1 {
		t.Fatalf("journal.Append call count = %d, want 1 — all envelopes must be coalesced into one call", appendCallCount)
	}
	if len(capturedEnvelopes) != 3 {
		t.Fatalf("envelopes captured = %d, want 3: %v", len(capturedEnvelopes), capturedEnvelopes)
	}
	if len(receipt.EventIDs) != 3 {
		t.Fatalf("receipt.EventIDs = %d, want 3", len(receipt.EventIDs))
	}
}
