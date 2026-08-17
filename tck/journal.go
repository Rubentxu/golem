package tck

import (
	"context"
	"testing"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// RunJournalStoreTCK is the black-box conformance kit for the JournalStore
// port. Adapters run it in their own package against the real provider
// (testcontainers) or an in-process implementation. Any failure means the
// adapter diverges from the journal contract and is not replaceable
// (ADR-046, ADR-052).
//
// The factory must return an empty, isolated store per call.
func RunJournalStoreTCK(t *testing.T, newStore func() ports.JournalStore) {
	mk := func(n int) ports.RawEvent {
		return ports.RawEvent{
			EventID:       validID(n),
			TenantID:      "t_tck",
			StreamID:      "workitem:wi-1",
			EventType:     "work.item.created.v1",
			SchemaVersion: 1,
			OccurredAt:    time.Now().UTC(),
			Actor:         ports.Actor{Type: "user", ID: "u_1"},
			Payload:       []byte(`{"n":` + itoa(n) + `}`),
		}
	}

	t.Run("append assigns increasing positions and replay preserves order", func(t *testing.T) {
		s := newStore()
		ctx := context.Background()
		res, err := s.Append(ctx, []ports.RawEvent{mk(1), mk(2), mk(3)})
		if err != nil {
			t.Fatal(err)
		}
		for i, r := range res {
			if want := ports.StreamPosition(i + 1); r.Position != want {
				t.Fatalf("position[%d] = %d, want %d", i, r.Position, want)
			}
			if r.Duplicate {
				t.Fatalf("first append of %s flagged duplicate", r.EventID)
			}
		}
		evs, last, err := s.Replay(ctx, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(evs) != 3 || last != 3 {
			t.Fatalf("replay got %d events, checkpoint %d; want 3, 3", len(evs), last)
		}
		for i, e := range evs {
			if e.EventID != validID(i+1) {
				t.Fatalf("order violated at %d: %s", i, e.EventID)
			}
		}
	})

	t.Run("duplicate event id is idempotent", func(t *testing.T) {
		s := newStore()
		ctx := context.Background()
		if _, err := s.Append(ctx, []ports.RawEvent{mk(1), mk(2)}); err != nil {
			t.Fatal(err)
		}
		res, err := s.Append(ctx, []ports.RawEvent{mk(2)})
		if err != nil {
			t.Fatalf("duplicate append must not error: %v", err)
		}
		if !res[0].Duplicate || res[0].Position != 2 {
			t.Fatalf("duplicate result = %+v, want {Position:2, Duplicate:true}", res[0])
		}
		evs, _, err := s.Replay(ctx, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(evs) != 2 {
			t.Fatalf("journal contains %d events after duplicate append, want 2", len(evs))
		}
	})

	t.Run("envelope validation rejects invalid events", func(t *testing.T) {
		s := newStore()
		ctx := context.Background()
		cases := []struct {
			name string
			mut  func(*ports.RawEvent)
			want error
		}{
			{"empty tenant", func(e *ports.RawEvent) { e.TenantID = "" }, ports.ErrEmptyTenant},
			{"empty event id", func(e *ports.RawEvent) { e.EventID = "" }, ports.ErrEmptyEventID},
			{"empty actor", func(e *ports.RawEvent) { e.Actor = ports.Actor{} }, ports.ErrEmptyActor},
			{"zero timestamp", func(e *ports.RawEvent) { e.OccurredAt = time.Time{} }, ports.ErrZeroTimestamp},
			{"event type too short", func(e *ports.RawEvent) { e.EventType = "work.created.v1" }, ports.ErrInvalidEventType},
			{"event type bad major", func(e *ports.RawEvent) { e.EventType = "work.item.created.one" }, ports.ErrInvalidEventType},
			{"event type empty segment", func(e *ports.RawEvent) { e.EventType = "work..created.v1" }, ports.ErrInvalidEventType},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				e := mk(99)
				c.mut(&e)
				if _, err := s.Append(ctx, []ports.RawEvent{e}); err != c.want {
					t.Fatalf("err = %v, want %v", err, c.want)
				}
			})
		}
	})

	t.Run("batch append is all-or-nothing", func(t *testing.T) {
		s := newStore()
		ctx := context.Background()
		bad := mk(50)
		bad.TenantID = ""
		if _, err := s.Append(ctx, []ports.RawEvent{mk(1), bad}); err == nil {
			t.Fatal("expected error for invalid batch")
		}
		evs, last, err := s.Replay(ctx, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(evs) != 0 || last != 0 {
			t.Fatalf("partial batch persisted: %d events, checkpoint %d", len(evs), last)
		}
	})

	t.Run("replay paging acts as checkpoints", func(t *testing.T) {
		s := newStore()
		ctx := context.Background()
		if _, err := s.Append(ctx, []ports.RawEvent{mk(1), mk(2), mk(3)}); err != nil {
			t.Fatal(err)
		}
		evs, last, err := s.Replay(ctx, 0, 2)
		if err != nil || len(evs) != 2 || last != 2 {
			t.Fatalf("page 1: %d events, checkpoint %d, err %v", len(evs), last, err)
		}
		evs, last, err = s.Replay(ctx, last, 2)
		if err != nil || len(evs) != 1 || last != 3 {
			t.Fatalf("page 2: %d events, checkpoint %d, err %v", len(evs), last, err)
		}
		evs, last, err = s.Replay(ctx, last, 0)
		if err != nil || len(evs) != 0 || last != 3 {
			t.Fatalf("exhausted: %d events, checkpoint %d, err %v", len(evs), last, err)
		}
	})

	t.Run("read stream is scoped by tenant and stream", func(t *testing.T) {
		s := newStore()
		ctx := context.Background()
		a1 := mk(1)
		a2 := mk(2)
		other := mk(3)
		other.StreamID = "workitem:wi-2"
		cross := mk(4)
		cross.TenantID = "t_other"
		if _, err := s.Append(ctx, []ports.RawEvent{a1, a2, other, cross}); err != nil {
			t.Fatal(err)
		}
		evs, err := s.ReadStream(ctx, "t_tck", "workitem:wi-1", 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(evs) != 2 {
			t.Fatalf("stream read got %d events, want 2", len(evs))
		}
		evs, err = s.ReadStream(ctx, "t_tck", "workitem:wi-1", 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(evs) != 1 || evs[0].EventID != a2.EventID {
			t.Fatalf("fromVersion=1 got %+v, want only %s", evs, a2.EventID)
		}
	})
}

func validID(n int) string {
	const base = "01JTCK000000000000000000"
	return base + itoa(1000 + n)[1:]
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
