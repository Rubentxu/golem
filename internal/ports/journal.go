package ports

import (
	"context"
	"encoding/json"
)

// StreamPosition is the global, monotonically increasing position of an
// event in the Graph Journal. Command receipts carry it to enable optional
// read-your-write semantics.
type StreamPosition uint64

// RawEvent is a journal envelope with an opaque, schema-versioned payload.
// The Journal stores payloads as raw JSON bytes: it never interprets event
// business data (ADR-030 — the journal combines state semantics and causal
// history, not payload knowledge).
type RawEvent = Envelope[json.RawMessage]

// AppendResult reports the outcome of appending one event. Duplicate is
// true — without error — when the same event_id was already journaled:
// producers may retry safely (idempotent acceptance, ADR-020/032).
type AppendResult struct {
	EventID   string
	Position  StreamPosition
	Duplicate bool
}

// JournalStore is the authoritative causal history port (ADR-005).
//
// Contract:
//   - Append is atomic per batch: either all new events are persisted or
//     none; already-known event_ids are idempotent no-ops.
//   - Replay returns events with position > from, in position order, and
//     the position of the last returned event (a checkpoint).
//   - Event acceptance never depends on external sinks (ADR-032).
type JournalStore interface {
	Append(ctx context.Context, events []RawEvent) ([]AppendResult, error)
	ReadStream(ctx context.Context, tenant TenantID, streamID string, fromVersion uint64) ([]RawEvent, error)
	Replay(ctx context.Context, from StreamPosition, limit int) ([]RawEvent, StreamPosition, error)
}
