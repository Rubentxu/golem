package ports

import (
	"context"
)

// JournalStreamReader is the kernel-level narrow port for reading a journal
// stream from a specific version onward. It is used by mounted handlers that
// need to read stream events (e.g., GET /api/v1/work-items/{id}/events).
//
// This interface is introduced in M10b to satisfy REQ-MOUNT-JournalStreamReader.
// It is a narrow, single-method port that delegates to JournalStore.ReadStream.
type JournalStreamReader interface {
	// ReadStream returns all events in the given stream at or after fromVersion.
	// The events are returned in position order. An empty slice with no error
	// means the stream exists but has no events at or after fromVersion.
	ReadStream(ctx context.Context, tenant TenantID, stream string, fromVersion uint64) ([]RawEvent, error)
}

// NewJournalStreamReaderOverJournalStore returns a JournalStreamReader that
// delegates to the provided JournalStore. This is the reference adapter.
func NewJournalStreamReaderOverJournalStore(js JournalStore) JournalStreamReader {
	return &journalStreamReaderAdapter{js: js}
}

type journalStreamReaderAdapter struct {
	js JournalStore
}

func (a *journalStreamReaderAdapter) ReadStream(ctx context.Context, tenant TenantID, stream string, fromVersion uint64) ([]RawEvent, error) {
	return a.js.ReadStream(ctx, tenant, stream, fromVersion)
}
