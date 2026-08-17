// Package memstore provides the in-memory reference adapter of the
// EventTransport port: a FIFO queue with ack tracking and redelivery of
// unacknowledged events. It is the transport baseline for tests, the TCK
// and single-process deployments; the NATS JetStream adapter must be
// semantically equivalent (tck.RunEventTransportTCK).
package memstore

import (
	"context"
	"sync"

	"github.com/Rubentxu/golem/internal/ports"
)

type entry struct {
	event ports.RawEvent
	acked bool
}

// Transport is an in-memory at-least-once event transport.
type Transport struct {
	mu    sync.Mutex
	queue []entry
	byID  map[string]int // event_id -> queue index
}

// NewTransport builds an empty transport.
func NewTransport() *Transport {
	return &Transport{
		byID: map[string]int{},
	}
}

// Publish enqueues events in order. Duplicate event_ids are dropped:
// producers may retry publishes safely.
func (t *Transport) Publish(_ context.Context, events []ports.RawEvent) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, e := range events {
		if _, dup := t.byID[e.EventID]; dup {
			continue
		}
		t.byID[e.EventID] = len(t.queue)
		t.queue = append(t.queue, entry{event: e})
	}
	return nil
}

// Fetch returns up to max unacknowledged events in publish order,
// starting from the oldest unacknowledged one. A nil slice means
// nothing is pending.
func (t *Transport) Fetch(_ context.Context, max int) ([]ports.RawEvent, error) {
	if max <= 0 {
		return nil, nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	out := []ports.RawEvent{}
	for i := range t.queue {
		if t.queue[i].acked {
			continue
		}
		out = append(out, t.queue[i].event)
		if len(out) >= max {
			break
		}
	}
	return out, nil
}

// Ack marks an event as delivered. Idempotent: unknown or already-acked
// ids are no-ops (at-least-once redelivery makes strict errors harmful).
func (t *Transport) Ack(_ context.Context, eventID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if i, ok := t.byID[eventID]; ok {
		t.queue[i].acked = true
	}
	// Compact the acked prefix to keep Fetch scans bounded.
	n := 0
	for n < len(t.queue) && t.queue[n].acked {
		delete(t.byID, t.queue[n].event.EventID)
		n++
	}
	if n > 0 {
		t.queue = append([]entry(nil), t.queue[n:]...)
		for id, idx := range t.byID {
			t.byID[id] = idx - n
		}
	}
	return nil
}
