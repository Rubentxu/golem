package ports

import (
	"context"
	"encoding/json"
	"errors"
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

// StreamVersion identifies a stream state for optimistic concurrency:
// Version is the number of events already in the stream (ADR-021).
type StreamVersion struct {
	TenantID TenantID
	StreamID string
	Version  uint64
}

// JournalStore is the authoritative causal history port (ADR-005).
//
// Contract:
//   - Append is atomic per batch: either all new events are persisted or
//     none; already-known event_ids are idempotent no-ops.
//   - AppendIf is the conditional form: it succeeds only when the stream
//     currently holds exactly expected.Version events, else fails with
//     ErrVersionConflict without persisting anything. This is the kernel
//     optimistic-concurrency primitive (ADR-021).
//   - Replay returns events with position > from, in position order, and
//     the position of the last returned event (a checkpoint).
//   - Event acceptance never depends on external sinks (ADR-032).
type JournalStore interface {
	Append(ctx context.Context, events []RawEvent) ([]AppendResult, error)
	AppendIf(ctx context.Context, expected StreamVersion, events []RawEvent) ([]AppendResult, error)
	ReadStream(ctx context.Context, tenant TenantID, streamID string, fromVersion uint64) ([]RawEvent, error)
	Replay(ctx context.Context, from StreamPosition, limit int) ([]RawEvent, StreamPosition, error)
	// Head returns the position of the newest persisted event (0 when
	// empty). Tail loops use it to measure lag (OBSERVABILITY.md).
	Head(ctx context.Context) (StreamPosition, error)
	// Backup creates a consistent snapshot and returns a handle.
	Backup(ctx context.Context) (BackupHandle, error)
	// Restore restores from a backup handle.
	Restore(ctx context.Context, handle BackupHandle) error
}

// BackupHandle identifies a journal backup (REQ-DR-001).
type BackupHandle struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Digest    string `json:"digest"` // sha256 of backup content
	SizeBytes int64  `json:"size_bytes"`
}

// CommandRecord describes a command for the optional CommandJournal interface.
type CommandRecord struct {
	CommandID     string
	CommandKind   string
	TenantID      TenantID
	Actor         Actor
	CorrelationID string
	Fingerprint   string `json:",omitempty"`
}

// CommandJournalReceipt acknowledges a command journal append.
type CommandJournalReceipt struct {
	CommandID   string
	EventIDs    []string
	Position    StreamPosition
	Tenant      TenantID
	Actor       Actor
	Correlation string
	Duplicate   bool
}

// CommandJournal is the optional command-journal port.
// Journals that implement this interface provide atomic append with
// idempotent retry: same command_id with same payload returns a cached
// receipt (Duplicate=true); same command_id with different payload returns
// ErrCommandMismatch.
type CommandJournal interface {
	AppendCommand(ctx context.Context, cmd CommandRecord, events []RawEvent) (CommandJournalReceipt, error)
}

// JournalStoreBackupErrors are the error sentinels for DR operations.
var (
	// ErrBackupInProgress is returned when a backup is already running.
	ErrBackupInProgress = errors.New("journal: backup in progress")
	// ErrRestoreMismatch is returned when the backup digest does not match.
	ErrRestoreMismatch = errors.New("journal: restore digest mismatch")
	// ErrRestoreNotEmpty is returned when Restore is called on a non-empty journal.
	ErrRestoreNotEmpty = errors.New("journal: restore requires empty journal")
)
