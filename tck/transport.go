package tck

import (
	"context"
	"testing"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// RunEventTransportTCK is the black-box conformance kit for the
// EventTransport port (ADR-012, ADR-020, ADR-046). Every transport
// adapter — in-memory baseline, NATS JetStream, future candidates — must
// pass it: at-least-once Fetch/Ack delivery in order, idempotent acks,
// duplicate-publish tolerance.
//
// The factory must return an empty, isolated transport per call.
func RunEventTransportTCK(t *testing.T, newTransport func() ports.EventTransport) {
	mk := func(n int) ports.RawEvent {
		return ports.RawEvent{
			EventID:    "01JTCKTR" + itoa(100000 + n)[1:],
			TenantID:   "t_tck",
			EventType:  "work.item.created.v1",
			OccurredAt: time.Now().UTC(),
			Actor:      ports.Actor{Type: "user", ID: "u_1"},
			Payload:    []byte(`{}`),
		}
	}

	t.Run("publish fetch preserves order and respects max", func(t *testing.T) {
		tr := newTransport()
		ctx := context.Background()
		if err := tr.Publish(ctx, []ports.RawEvent{mk(1), mk(2), mk(3)}); err != nil {
			t.Fatal(err)
		}
		got, err := tr.Fetch(ctx, 2)
		if err != nil || len(got) != 2 {
			t.Fatalf("fetch = %d events, err %v; want 2", len(got), err)
		}
		if got[0].EventID != mk(1).EventID || got[1].EventID != mk(2).EventID {
			t.Fatalf("order violated: %s, %s", got[0].EventID, got[1].EventID)
		}
	})

	t.Run("unacked events are redelivered", func(t *testing.T) {
		tr := newTransport()
		ctx := context.Background()
		if err := tr.Publish(ctx, []ports.RawEvent{mk(1), mk(2)}); err != nil {
			t.Fatal(err)
		}
		got, _ := tr.Fetch(ctx, 10)
		if len(got) != 2 {
			t.Fatalf("first fetch = %d, want 2", len(got))
		}
		again, _ := tr.Fetch(ctx, 10)
		if len(again) != 2 {
			t.Fatalf("redelivery = %d, want 2 (at-least-once)", len(again))
		}
	})

	t.Run("ack advances and is idempotent", func(t *testing.T) {
		tr := newTransport()
		ctx := context.Background()
		if err := tr.Publish(ctx, []ports.RawEvent{mk(1), mk(2)}); err != nil {
			t.Fatal(err)
		}
		if err := tr.Ack(ctx, mk(1).EventID); err != nil {
			t.Fatal(err)
		}
		// Acking an already-acked or unknown id must not error.
		if err := tr.Ack(ctx, mk(1).EventID); err != nil {
			t.Fatalf("double ack errored: %v", err)
		}
		if err := tr.Ack(ctx, "01JUNKNOWN00000000000000000"); err != nil {
			t.Fatalf("unknown ack errored: %v", err)
		}
		got, _ := tr.Fetch(ctx, 10)
		if len(got) != 1 || got[0].EventID != mk(2).EventID {
			t.Fatalf("after ack = %+v, want only event 2", ids(got))
		}
	})

	t.Run("duplicate publish is tolerated", func(t *testing.T) {
		tr := newTransport()
		ctx := context.Background()
		e := mk(1)
		if err := tr.Publish(ctx, []ports.RawEvent{e}); err != nil {
			t.Fatal(err)
		}
		if err := tr.Publish(ctx, []ports.RawEvent{e}); err != nil {
			t.Fatal(err)
		}
		got, _ := tr.Fetch(ctx, 10)
		if len(got) != 1 {
			t.Fatalf("duplicate publish delivered %d copies, want 1", len(got))
		}
	})

	t.Run("empty fetch is nil-safe", func(t *testing.T) {
		tr := newTransport()
		got, err := tr.Fetch(context.Background(), 5)
		if err != nil || len(got) != 0 {
			t.Fatalf("empty fetch = %d events, err %v", len(got), err)
		}
	})
}

func ids(events []ports.RawEvent) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = e.EventID
	}
	return out
}
