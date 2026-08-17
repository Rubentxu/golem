package ports

import "context"

// CheckpointStore persists resume positions for journal tailing
// components (GRAPH_JOURNAL: "Checkpoint + replay idempotente").
// Two canonical keys exist today — "outbox" for the transport publisher
// and "projection" for the graph projector — both storing a journal
// StreamPosition. Adapters must make Save atomic and durable per key;
// a lost checkpoint is safe (replay is idempotent upstream) but wasteful.
type CheckpointStore interface {
	Load(ctx context.Context, key string) (StreamPosition, error)
	Save(ctx context.Context, key string, pos StreamPosition) error
}
