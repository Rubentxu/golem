package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
)

// postgresTCK runs the JournalStore TCK against the postgres adapter.
// Skip if GOLEM_TEST_POSTGRES is not set.
func TestPostgresJournalTCK(t *testing.T) {
	connString := os.Getenv("GOLEM_TEST_POSTGRES")
	if connString == "" {
		t.Skip("GOLEM_TEST_POSTGRES not set — skipping postgres TCK")
	}

	ctx := context.Background()
	store, err := NewStore(ctx, connString)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	// Run the generic TCK tests.
	runJournalTCK(t, store)
}

// runJournalTCK runs the core journal store TCK tests.
func runJournalTCK(t *testing.T, store *Store) {
	ctx := context.Background()
	tenant := ports.TenantID("t-pg-tck")

	t.Run("AppendSingleEvent", func(t *testing.T) {
		store.Close()
		store, _ = NewStore(ctx, os.Getenv("GOLEM_TEST_POSTGRES"))
		defer store.Close()

		results, err := store.Append(ctx, []ports.RawEvent{{
			EventID:    "e1",
			TenantID:   string(tenant),
			StreamID:   "s1",
			EventType:  "test.event.v1",
			Actor:      ports.Actor{Type: "service", ID: "tck"},
			OccurredAt: time.Now(),
			Payload:    []byte(`{"msg":"hello"}`),
		}})
		if err != nil {
			t.Fatalf("Append: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("len(results) = %d, want 1", len(results))
		}
		if results[0].Duplicate {
			t.Error("first append should not be duplicate")
		}
	})

	t.Run("AppendDuplicateIsIdempotent", func(t *testing.T) {
		results, err := store.Append(ctx, []ports.RawEvent{{
			EventID:    "e1", // same event ID
			TenantID:   string(tenant),
			StreamID:   "s1",
			EventType:  "test.event.v1",
			Actor:      ports.Actor{Type: "service", ID: "tck"},
			OccurredAt: time.Now(),
			Payload:    []byte(`{"msg":"hello"}`),
		}})
		if err != nil {
			t.Fatalf("Append duplicate: %v", err)
		}
		if !results[0].Duplicate {
			t.Error("second append with same event_id should be duplicate")
		}
	})

	t.Run("AppendIfVersionMismatch", func(t *testing.T) {
		_, err := store.AppendIf(ctx, ports.StreamVersion{
			TenantID: tenant, StreamID: "s1", Version: 999,
		}, []ports.RawEvent{{
			EventID:    "e2",
			TenantID:   string(tenant),
			StreamID:   "s1",
			EventType:  "test.event.v1",
			Actor:      ports.Actor{Type: "service", ID: "tck"},
			OccurredAt: time.Now(),
			Payload:    []byte(`{}`),
		}})
		if err != ErrVersionConflict {
			t.Errorf("AppendIf: got %v, want ErrVersionConflict", err)
		}
	})

	t.Run("AppendIfSuccess", func(t *testing.T) {
		// Current stream s1 has version 1 (one event).
		results, err := store.AppendIf(ctx, ports.StreamVersion{
			TenantID: tenant, StreamID: "s1", Version: 1,
		}, []ports.RawEvent{{
			EventID:    "e3",
			TenantID:   string(tenant),
			StreamID:   "s1",
			EventType:  "test.event.v1",
			Actor:      ports.Actor{Type: "service", ID: "tck"},
			OccurredAt: time.Now(),
			Payload:    []byte(`{}`),
		}})
		if err != nil {
			t.Fatalf("AppendIf: %v", err)
		}
		if results[0].Duplicate {
			t.Error("AppendIf should not be duplicate")
		}
	})

	t.Run("ReadStream", func(t *testing.T) {
		events, err := store.ReadStream(ctx, tenant, "s1", 0)
		if err != nil {
			t.Fatalf("ReadStream: %v", err)
		}
		if len(events) != 3 {
			t.Errorf("len(events) = %d, want 3", len(events))
		}
	})

	t.Run("Replay", func(t *testing.T) {
		events, pos, err := store.Replay(ctx, 0, 0)
		if err != nil {
			t.Fatalf("Replay: %v", err)
		}
		if len(events) < 2 {
			t.Errorf("len(events) = %d, want >= 2", len(events))
		}
		if pos == 0 {
			t.Error("last position should be > 0")
		}
		_ = events
	})

	t.Run("Head", func(t *testing.T) {
		head, err := store.Head(ctx)
		if err != nil {
			t.Fatalf("Head: %v", err)
		}
		if head == 0 {
			t.Error("Head should be > 0 after events appended")
		}
	})
}
