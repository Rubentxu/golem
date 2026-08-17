// Package outbox implements the transactional-outbox publisher as a
// journal tail (ADR-020 + ADR-033): accepted events are read from the
// Graph Journal by checkpoint and published to the EventTransport.
//
// This design has no dual-write window by construction — the journal is
// the only source of truth and publishing is a derived, retryable
// operation (ADR-032: event acceptance never depends on external
// delivery). Crash between publish and checkpoint save replays the batch:
// at-least-once delivery with idempotent consumers.
package outbox

import (
	"context"
	"fmt"

	"github.com/Rubentxu/golem/internal/ports"
)

// Checkpoint key used by the publisher.
const CheckpointKey = "outbox"

// Publisher tails the journal into the transport.
type Publisher struct {
	journal    ports.JournalStore
	transport  ports.EventTransport
	checkpoint ports.CheckpointStore
}

// New wires a publisher.
func New(journal ports.JournalStore, transport ports.EventTransport, checkpoint ports.CheckpointStore) *Publisher {
	return &Publisher{journal: journal, transport: transport, checkpoint: checkpoint}
}

// Pump performs one tail cycle: read a batch after the checkpoint,
// publish it, then advance the checkpoint. Returns how many events were
// published; callers loop until 0 or ctx.Done.
func (p *Publisher) Pump(ctx context.Context, batchSize int) (int, error) {
	if batchSize <= 0 {
		batchSize = 100
	}
	from, err := p.checkpoint.Load(ctx, CheckpointKey)
	if err != nil {
		return 0, fmt.Errorf("outbox: load checkpoint: %w", err)
	}
	batch, last, err := p.journal.Replay(ctx, from, batchSize)
	if err != nil {
		return 0, fmt.Errorf("outbox: replay from %d: %w", from, err)
	}
	if len(batch) == 0 {
		return 0, nil
	}
	if err := p.transport.Publish(ctx, batch); err != nil {
		return 0, fmt.Errorf("outbox: publish %d events: %w", len(batch), err)
	}
	if err := p.checkpoint.Save(ctx, CheckpointKey, last); err != nil {
		return 0, fmt.Errorf("outbox: save checkpoint %d: %w", last, err)
	}
	return len(batch), nil
}
