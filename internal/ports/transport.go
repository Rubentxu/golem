package ports

import "context"

// EventTransport is the outbound/inbound event transport port (ADR-012:
// NATS JetStream is the reference implementation, never a core
// dependency; ADR-033: the broker is transport, never the source of
// truth — the Graph Journal is).
//
// Delivery contract (EVENT_MODEL):
//   - Publish may be retried by callers; brokers deduplicate by EventID
//     where supported (JetStream Msg-Id) and consumers must be
//     idempotent anyway (at-least-once).
//   - Fetch returns up to max undelivered/unacknowledged events in
//     journal order per transport partition. Events stay eligible for
//     redelivery until Acked — this Fetch/Ack pair is the idempotent
//     inbox of ADR-020.
//   - Ack is idempotent: acking an unknown or already-acked event is a
//     no-op, because at-least-once delivery may redeliver duplicates.
type EventTransport interface {
	Publish(ctx context.Context, events []RawEvent) error
	Fetch(ctx context.Context, max int) ([]RawEvent, error)
	Ack(ctx context.Context, eventID string) error
}
